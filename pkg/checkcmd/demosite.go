package checkcmd

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
)

//go:embed all:demosite
var demoSiteFS embed.FS

// buildDemoSite creates a small demo site in workDir that imports the given
// theme module, builds it with Hugo and reports errors and warnings.
func (c *Config) buildDemoSite(ctx context.Context, r *Report, workDir, modulePath string) {
	const check = "build"

	siteDir := filepath.Join(workDir, "demosite")
	sub, err := fs.Sub(demoSiteFS, "demosite")
	if err != nil {
		r.add(check, SeverityError, "failed to open embedded demo site: %s", err)
		return
	}
	if err := os.CopyFS(siteDir, sub); err != nil {
		r.add(check, SeverityError, "failed to create demo site: %s", err)
		return
	}

	config := map[string]interface{}{
		"baseURL":  "https://example.org/",
		"title":    "Hugo Theme Check Demo",
		"security": defaultSecurityConfig,
		"module": map[string]interface{}{
			"imports": []map[string]interface{}{
				{"path": modulePath},
			},
		},
	}
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		r.add(check, SeverityError, "failed to marshal demo site config: %s", err)
		return
	}
	if err := os.WriteFile(filepath.Join(siteDir, "config.json"), b, 0o666); err != nil {
		r.add(check, SeverityError, "failed to write demo site config: %s", err)
		return
	}

	cl, err := client.New(c.logWriter(), siteDir)
	if err != nil {
		r.add(check, SeverityError, "failed to create client: %s", err)
		return
	}
	if err := cl.InitModule(); err != nil {
		r.add(check, SeverityError, "failed to init demo site module: %s", err)
		return
	}

	c.checkNpmPackages(ctx, r, siteDir)

	latest := client.HugoBinary()
	out, err := runHugoCapture(ctx, latest, siteDir)
	if err != nil {
		failure := strings.Join(tailLines(out, 15), "\n")
		// A theme gets a pass (with a warning) if it still builds with the
		// baseline Hugo version; only themes that fail with both versions
		// fail the build check (and become bitrot candidates).
		if baseline := hugoBaselineBinary(); baseline != "" {
			bout, berr := runHugoCapture(ctx, baseline, siteDir)
			if berr == nil {
				message := "demo site built with baseline " + hugoVersion(ctx, baseline)
				if pages := parsePagesCount(bout); pages > 0 {
					message += " (" + strconv.Itoa(pages) + " pages)"
				}
				r.add(check, SeverityWarning, "%s, but the build failed with %s:\n%s", message, hugoVersion(ctx, latest), failure)
				return
			}
		}
		r.add(check, SeverityError, "demo site build failed with %s:\n%s", hugoVersion(ctx, latest), failure)
		return
	}

	message := "demo site built"
	if pages := parsePagesCount(out); pages > 0 {
		message += " (" + strconv.Itoa(pages) + " pages)"
	}
	// A theme that cannot render the home page is not functional. Other
	// missing layouts (e.g. taxonomy.html) are fine; many special purpose
	// themes ship a limited set of layouts, and they still surface below
	// as warnings.
	if slices.Contains(missingLayoutKinds(out), "home") {
		r.add(check, SeverityError, "%s, but the theme has no layout for the home page", message)
		return
	}
	if warnings := linesWithPrefix(out, "WARN"); len(warnings) > 0 {
		numWarnings := len(warnings)
		if numWarnings > 5 {
			warnings = warnings[:5]
		}
		r.add(check, SeverityWarning, "%s with %d warning(s):\n%s", message, numWarnings, strings.Join(warnings, "\n"))
		return
	}
	r.add(check, SeverityOK, "%s, no warnings", message)
}

// defaultSecurityConfig is Hugo's default security config (as of Hugo
// v0.165.0). It is set explicitly in the site configs so that an imported
// theme's config cannot override it.
var defaultSecurityConfig = map[string]interface{}{
	"enableInlineShortcodes": false,
	"exec": map[string]interface{}{
		"allow": []string{"^(dart-)?sass$", "^go$", "^git$", "^node$", "^postcss$"},
		"osEnv": []string{`(?i)^((HTTPS?|NO)_PROXY|PATH(EXT)?|APPDATA|TE?MP|TERM|GO\w+|(XDG_CONFIG_)?HOME|USERPROFILE|SSH_AUTH_SOCK|DISPLAY|LANG|SYSTEMDRIVE|PROGRAMDATA)$`},
	},
	"funcs": map[string]interface{}{
		"getenv": []string{"^HUGO_", "^CI$"},
	},
	"http": map[string]interface{}{
		"methods": []string{"(?i)GET|POST"},
		"urls":    []string{"(?i)^https?://[a-z0-9]", `! ^https?://\d+\.`, "! (?i)localhost", `! (?i)^https?://[^/?#]*@`},
	},
	"allowContent": []string{"! ^text/html$"},
	"node": map[string]interface{}{
		"permissions": map[string]interface{}{
			"allowAddons":       []string{"tailwindcss"},
			"allowChildProcess": []string{"tailwindcss"},
			"allowRead":         []string{"."},
			"allowWorker":       []string{"tailwindcss"},
		},
	},
}

// allowedNpmPackages is the allowlist of npm packages a theme may depend
// on. One of Hugo's selling points is that it can mostly be run without
// npm, so anything beyond the common CSS tooling is a red flag (and a
// potential supply-chain attack vector).
var allowedNpmPackages = map[string]bool{
	"@tailwindcss/aspect-ratio": true,
	"@tailwindcss/cli":          true,
	"@tailwindcss/forms":        true,
	"@tailwindcss/typography":   true,
	"autoprefixer":              true,
	"cssnano":                   true,
	"postcss":                   true,
	"postcss-cli":               true,
	"postcss-import":            true,
	"postcss-nesting":           true,
	"rtlcss":                    true,
	"tailwindcss":               true,
}

// checkNpmPackages runs `hugo mod npm pack` to collect any npm
// dependencies declared by the theme (package.hugo.json), verifies them
// against allowedNpmPackages and, if all are allowed, installs them so the
// demo site build can use them. Lifecycle scripts are never run.
func (c *Config) checkNpmPackages(ctx context.Context, r *Report, siteDir string) {
	const check = "npm"

	if out, err := runHugoCapture(ctx, client.HugoBinary(), siteDir, "mod", "npm", "pack"); err != nil {
		r.add(check, SeverityWarning, "hugo mod npm pack failed:\n%s", strings.Join(tailLines(out, 5), "\n"))
		return
	}

	packages, err := collectNpmDependencies(siteDir)
	if err != nil {
		r.add(check, SeverityWarning, "failed to parse package.json: %s", err)
		return
	}
	if len(packages) == 0 {
		r.add(check, SeverityOK, "no npm dependencies")
		return
	}

	if disallowed := disallowedNpmPackages(packages); len(disallowed) > 0 {
		r.add(check, SeverityError, "npm package(s) not on the allowlist: %s", strings.Join(disallowed, ", "))
		return
	}

	if _, err := exec.LookPath("npm"); err != nil {
		r.add(check, SeverityWarning, "npm not found in PATH; skipping install of: %s", strings.Join(packages, ", "))
		return
	}

	// Never run lifecycle scripts from untrusted packages.
	if err := os.WriteFile(filepath.Join(siteDir, ".npmrc"), []byte("ignore-scripts=true\n"), 0o666); err != nil {
		r.add(check, SeverityError, "failed to write .npmrc: %s", err)
		return
	}
	out, err := runNpmInstall(ctx, siteDir)
	if err != nil {
		r.add(check, SeverityError, "npm install failed:\n%s", strings.Join(tailLines(out, 10), "\n"))
		return
	}
	r.add(check, SeverityOK, "installed: %s", strings.Join(packages, ", "))
}

type npmPackageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      []string          `json:"workspaces"`
}

// collectNpmDependencies returns the names of the packages in dependencies
// and devDependencies of the site's package.json and any workspaces in it
// (hugo mod npm pack puts the merged module dependencies in the
// packages/hugoautogen workspace), sorted.
func collectNpmDependencies(siteDir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(siteDir, "package.json"))
	if err != nil {
		// No npm dependencies.
		return nil, nil
	}
	var root npmPackageJSON
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, err
	}
	pkgs := []npmPackageJSON{root}
	for _, ws := range root.Workspaces {
		b, err := os.ReadFile(filepath.Join(siteDir, filepath.FromSlash(ws), "package.json"))
		if err != nil {
			continue
		}
		var pkg npmPackageJSON
		if err := json.Unmarshal(b, &pkg); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}

	seen := make(map[string]bool)
	var packages []string
	for _, pkg := range pkgs {
		for _, m := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
			for name := range m {
				if !seen[name] {
					seen[name] = true
					packages = append(packages, name)
				}
			}
		}
	}
	sort.Strings(packages)
	return packages, nil
}

func disallowedNpmPackages(packages []string) []string {
	var disallowed []string
	for _, name := range packages {
		if !allowedNpmPackages[name] {
			disallowed = append(disallowed, name)
		}
	}
	return disallowed
}

func runNpmInstall(ctx context.Context, dir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "npm", "install", "--no-audit", "--no-fund")
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// hugoBaselineBinary returns the (older) baseline Hugo binary set with the
// HUGOTHEMES_HUGO_BASELINE environment variable (see firstup.env), or the
// empty string if not set.
func hugoBaselineBinary() string {
	return os.Getenv("HUGOTHEMES_HUGO_BASELINE")
}

var hugoVersions sync.Map

// hugoVersion returns the version (e.g. "v0.165.0") reported by the given
// Hugo binary, falling back to the binary name itself.
func hugoVersion(ctx context.Context, bin string) string {
	if v, ok := hugoVersions.Load(bin); ok {
		return v.(string)
	}
	v := bin
	if out, err := runHugoCapture(ctx, bin, "", "version"); err == nil {
		if m := hugoVersionRe.FindString(out); m != "" {
			v = m
		}
	}
	hugoVersions.Store(bin, v)
	return v
}

var hugoVersionRe = regexp.MustCompile(`v\d+\.\d+\.\d+`)

func runHugoCapture(ctx context.Context, bin, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func linesWithPrefix(s, prefix string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, prefix) {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func tailLines(s string, n int) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

var noLayoutRe = regexp.MustCompile(`found no layout file for "html" for kind "(\w+)"`)

// missingLayoutKinds returns the page kinds Hugo could not find a layout
// for. A theme that cannot render the basic page kinds is not functional.
func missingLayoutKinds(out string) []string {
	var kinds []string
	seen := make(map[string]bool)
	for _, match := range noLayoutRe.FindAllStringSubmatch(out, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			kinds = append(kinds, match[1])
		}
	}
	return kinds
}

var pagesCountRe = regexp.MustCompile(`(?m)^\s*Pages\s*│?\s*[|│]\s*(\d+)`)

// parsePagesCount extracts the page count from Hugo's build stats table,
// returning 0 if not found.
func parsePagesCount(s string) int {
	match := pagesCountRe.FindStringSubmatch(s)
	if match == nil {
		return 0
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return n
}
