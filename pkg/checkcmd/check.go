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
		ShortUsage: rootcmd.CommandName + " check [flags] <module-path>...",
		ShortHelp:  "Check runs the basic theme submission checks for the given theme module(s).",
		LongHelp: `Check runs the basic (deterministic) theme submission checks for the given
theme module(s), e.g.:

   hugothemesitebuilder check github.com/user/my-theme

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
		return errors.New("check: at least one theme module path is required, e.g. github.com/user/my-theme")
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
	c.checkHugoConfig(r, m)
	c.checkMetaURLs(r, m)
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
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		r.add(check, SeverityError, "failed to marshal config: %s", err)
		return nil
	}
	if err := os.WriteFile(filepath.Join(modDir, "config.json"), b, 0o666); err != nil {
		r.add(check, SeverityError, "failed to write config: %s", err)
		return nil
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
		return &m
	}

	r.add(check, SeverityError, "module not found in the module graph")
	return nil
}

func (c *Config) logWriter() io.Writer {
	if c.rootConfig.Quiet {
		return io.Discard
	}
	return os.Stdout
}
