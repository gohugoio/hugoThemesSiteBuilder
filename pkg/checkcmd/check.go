package checkcmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bep/workers"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/rootcmd"
)

// Config for the check subcommand.
type Config struct {
	// Print the report as JSON.
	jsonOutput bool

	// Keep the temporary work dirs (for debugging).
	keep bool

	// Check all themes in themes.txt and themes.bitrot.txt and propose
	// moves between them.
	bitrot bool

	// Apply the moves proposed by bitrot.
	write bool

	// Number of themes to check in parallel.
	parallel int

	rootConfig *rootcmd.Config
}

// New returns a usable ffcli.Command for the check subcommand.
func New(rootConfig *rootcmd.Config) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
	}

	fs := flag.NewFlagSet(rootcmd.CommandName+" check", flag.ExitOnError)
	fs.BoolVar(&cfg.jsonOutput, "json", false, "print the report as JSON")
	fs.BoolVar(&cfg.keep, "keep", false, "keep the temporary work dirs (for debugging)")
	fs.BoolVar(&cfg.bitrot, "bitrot", false, "check all themes in themes.txt and themes.bitrot.txt and propose moves between them")
	fs.BoolVar(&cfg.write, "write", false, "apply the moves proposed by -bitrot")
	fs.IntVar(&cfg.parallel, "parallel", 4, "number of themes to check in parallel")
	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "check",
		ShortUsage: rootcmd.CommandName + " check [flags] <module-path>|<PR-number>...",
		ShortHelp:  "Check runs the basic theme submission checks for the given theme module(s).",
		LongHelp: `Check runs the basic (deterministic) theme submission checks for the given
theme module(s), e.g.:

   hugothemesitebuilder check github.com/user/my-theme

An all-digits argument is treated as a pull request number in this
repository, and is replaced with the theme(s) that pull request adds to
themes.txt:

   hugothemesitebuilder check 812

It resolves each theme with Hugo Modules, verifies the required files
(theme.toml, LICENSE, README.md, thumbnail and screenshot images), and
builds a small demo site with the theme to count errors and warnings.

The Hugo binary used can be overridden with the HUGOTHEMES_HUGO_LATEST
environment variable. If HUGOTHEMES_HUGO_BASELINE is also set (see
firstup.env), a demo site that fails to build is retried with that older
baseline version; the theme then passes the build check with a warning if
the baseline build succeeds, and fails it only if both versions fail.

The command exits with a non-zero status if any theme fails a check with
severity error.`,
		FlagSet: fs,
		Exec:    cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	if c.bitrot {
		return c.execBitrot(ctx)
	}

	if len(args) == 0 {
		return errors.New("check: at least one theme module path or PR number is required, e.g. github.com/user/my-theme")
	}

	args, err := expandPRArgs(args)
	if err != nil {
		return err
	}

	reports := c.checkThemes(ctx, args)

	if c.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(reports); err != nil {
			return err
		}
	} else {
		for _, r := range reports {
			r.render(os.Stdout)
		}
	}

	var failed int
	for _, r := range reports {
		if !r.Pass {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d theme(s) failed the basic checks", failed, len(reports))
	}
	return nil
}

// checkThemes checks the given themes, c.parallel at a time.
func (c *Config) checkThemes(ctx context.Context, modulePaths []string) []*Report {
	parallel := max(c.parallel, 1)
	reports := make([]*Report, len(modulePaths))
	w := workers.New(parallel)
	r, _ := w.Start(ctx)
	for i, modulePath := range modulePaths {
		i, modulePath := i, modulePath
		r.Run(func() error {
			reports[i] = c.checkTheme(ctx, modulePath)
			return nil
		})
	}
	client.CheckErr(r.Wait())
	return reports
}

func (c *Config) checkTheme(ctx context.Context, modulePath string) *Report {
	r := &Report{Module: modulePath}
	defer r.finalize()

	workDir, err := os.MkdirTemp("", "theme-check-")
	if err != nil {
		r.add("setup", SeverityError, "failed to create work dir: %s", err)
		return r
	}
	if c.keep {
		fmt.Fprintf(os.Stderr, "keeping work dir %s\n", workDir)
	} else {
		defer os.RemoveAll(workDir)
	}

	m := c.resolveModule(r, workDir, modulePath)
	if m == nil {
		return r
	}

	c.checkThemeToml(r, m)
	c.checkLicense(r, m.Dir)
	c.checkImages(r, m.Dir)
	c.checkReadme(r, m.Dir)
	c.checkHugoConfig(ctx, r, m, filepath.Join(workDir, "mod"))
	c.checkMetaURLs(ctx, r, m)
	c.buildDemoSite(ctx, r, workDir, modulePath)

	return r
}

// resolveModule fetches the theme with Hugo Modules and returns the module
// information, including the directory it got downloaded to. It returns nil
// if the module could not be resolved.
func (c *Config) resolveModule(r *Report, workDir, modulePath string) *client.Module {
	const check = "module"

	modDir := filepath.Join(workDir, "mod")
	if err := os.MkdirAll(modDir, 0o777); err != nil {
		r.add(check, SeverityError, "failed to create module dir: %s", err)
		return nil
	}

	// The resolution config ignores the theme's own config and imports so
	// that resolving the module is as robust as possible; how the theme
	// config behaves is judged separately.
	config := map[string]interface{}{
		"module": map[string]interface{}{
			"imports": []map[string]interface{}{
				{
					"path":          modulePath,
					"ignoreImports": true,
					"ignoreConfig":  true,
					"noMounts":      true,
				},
			},
		},
	}
	// The merged config deep-merges the theme's config (and resolves its
	// imports) so that e.g. module.hugoVersion is visible to "hugo config";
	// see themeHugoVersion. The security config is set explicitly so the
	// theme cannot override it.
	configMerged := map[string]interface{}{
		"_merge":   "deep",
		"security": defaultSecurityConfig,
		"module": map[string]interface{}{
			"imports": []map[string]interface{}{
				{
					"path":     modulePath,
					"noMounts": true,
				},
			},
		},
	}
	for filename, v := range map[string]interface{}{
		"config.json":        config,
		mergedConfigFilename: configMerged,
	} {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			r.add(check, SeverityError, "failed to marshal config: %s", err)
			return nil
		}
		if err := os.WriteFile(filepath.Join(modDir, filename), b, 0o666); err != nil {
			r.add(check, SeverityError, "failed to write config: %s", err)
			return nil
		}
	}

	cl, err := client.New(c.logWriter(), modDir)
	if err != nil {
		r.add(check, SeverityError, "failed to create client: %s", err)
		return nil
	}
	if err := cl.InitModule(); err != nil {
		r.add(check, SeverityError, "failed to init module: %s", err)
		return nil
	}

	mmap, err := cl.GetHugoModulesMap("config.json")
	if err != nil {
		r.add(check, SeverityError, "failed to resolve module: %s", err)
		return nil
	}

	for _, m := range mmap {
		m := m
		version := m.Version
		if version == "" {
			version = "(unversioned)"
		}
		message := "resolved " + version
		if !m.Time.IsZero() {
			message += " from " + m.Time.Format("2006-01-02")
		}
		r.add(check, SeverityOK, message)
		r.add("source", SeverityOK, sourceURL(&m))
		return &m
	}

	r.add(check, SeverityError, "module not found in the module graph")
	return nil
}

// pseudoVersionRe matches the timestamp-hash suffix of a Go pseudo-version,
// e.g. v0.0.0-20230101120000-abcdef123456.
var pseudoVersionRe = regexp.MustCompile(`-\d{14}-([0-9a-f]{12})$`)

// sourceURL returns a URL to browse the module source at the resolved
// version, falling back to the repository root for unknown hosts.
func sourceURL(m *client.Module) string {
	repo := m.PathRepo() // e.g. github.com/user/repo

	ref := strings.TrimSuffix(m.Version, "+incompatible")
	isCommit := false
	if match := pseudoVersionRe.FindStringSubmatch(ref); match != nil {
		ref = match[1]
		isCommit = true
	}

	switch {
	case ref == "":
		return "https://" + repo
	case strings.HasPrefix(repo, "github.com/"):
		return "https://" + repo + "/tree/" + ref
	case strings.HasPrefix(repo, "gitlab.com/"):
		return "https://" + repo + "/-/tree/" + ref
	case strings.HasPrefix(repo, "codeberg.org/"):
		if isCommit {
			return "https://" + repo + "/src/commit/" + ref
		}
		return "https://" + repo + "/src/tag/" + ref
	default:
		return "https://" + repo
	}
}

func (c *Config) logWriter() io.Writer {
	if c.rootConfig.Quiet {
		return io.Discard
	}
	return os.Stdout
}
