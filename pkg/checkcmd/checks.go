package checkcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
)

func (c *Config) checkThemeToml(r *Report, m *client.Module) {
	const check = "theme.toml"
	b, err := os.ReadFile(filepath.Join(m.Dir, "theme.toml"))
	if err != nil {
		r.add(check, SeverityError, "no theme.toml found in the theme root")
		return
	}
	var meta map[string]interface{}
	if err := toml.Unmarshal(b, &meta); err != nil {
		r.add(check, SeverityError, "failed to parse theme.toml: %s", err)
		return
	}
	var missing []string
	for _, key := range []string{"name", "description", "license"} {
		if v, found := meta[key]; !found || v == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		r.add(check, SeverityWarning, "missing recommended field(s): %s", strings.Join(missing, ", "))
		return
	}
	r.add(check, SeverityOK, "found with name, description and license")
}

var licenseFilenames = []string{
	"LICENSE", "LICENSE.md", "LICENSE.txt",
	"LICENCE", "LICENCE.md", "LICENCE.txt",
	"COPYING", "COPYING.txt", "UNLICENSE",
}

func (c *Config) checkLicense(r *Report, dir string) {
	const check = "license"
	entries, err := os.ReadDir(dir)
	if err != nil {
		r.add(check, SeverityError, "failed to read theme dir: %s", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		for _, name := range licenseFilenames {
			if strings.EqualFold(entry.Name(), name) {
				r.add(check, SeverityOK, entry.Name())
				return
			}
		}
	}
	r.add(check, SeverityError, "no LICENSE file found in the theme root; an Open Source license is required")
}

type imageSpec struct {
	base       string
	minW, minH int
}

var imageSpecs = []imageSpec{
	{base: "images/tn", minW: 900, minH: 600},
	{base: "images/screenshot", minW: 1500, minH: 1000},
}

func (c *Config) checkImages(r *Report, dir string) {
	for _, spec := range imageSpecs {
		c.checkImage(r, dir, spec)
	}
}

func (c *Config) checkImage(r *Report, dir string, spec imageSpec) {
	check := spec.base
	var filename string
	for _, ext := range []string{".png", ".jpg"} {
		candidate := filepath.Join(dir, filepath.FromSlash(spec.base)+ext)
		if _, err := os.Stat(candidate); err == nil {
			filename = candidate
			break
		}
	}
	if filename == "" {
		r.add(check, SeverityError, "missing; provide %s.{png,jpg}", spec.base)
		return
	}
	f, err := os.Open(filename)
	if err != nil {
		r.add(check, SeverityError, "failed to open: %s", err)
		return
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		r.add(check, SeverityError, "failed to decode %s: %s", filepath.Base(filename), err)
		return
	}
	if config.Width < spec.minW || config.Height < spec.minH {
		r.add(check, SeverityError, "%dx%d is below the minimum size %dx%d", config.Width, config.Height, spec.minW, spec.minH)
		return
	}
	if !aspectRatioOK(config.Width, config.Height) {
		r.add(check, SeverityWarning, "%dx%d does not have the required 3:2 aspect ratio", config.Width, config.Height)
		return
	}
	r.add(check, SeverityOK, "%s %dx%d", path.Base(filename), config.Width, config.Height)
}

// aspectRatioOK reports whether w:h is 3:2 within a small tolerance.
func aspectRatioOK(w, h int) bool {
	if h == 0 {
		return false
	}
	const (
		target    = 3.0 / 2.0
		tolerance = 0.02
	)
	ratio := float64(w) / float64(h)
	return math.Abs(ratio-target)/target <= tolerance
}

func (c *Config) checkReadme(r *Report, dir string) {
	const check = "readme"
	entries, err := os.ReadDir(dir)
	if err != nil {
		r.add(check, SeverityError, "failed to read theme dir: %s", err)
		return
	}
	var filename string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "README.md") {
			filename = filepath.Join(dir, entry.Name())
			break
		}
	}
	if filename == "" {
		r.add(check, SeverityError, "no README.md found in the theme root")
		return
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		r.add(check, SeverityError, "failed to read README.md: %s", err)
		return
	}
	content := string(b)
	numResults := len(r.Results)
	if len(strings.TrimSpace(content)) < 300 {
		r.add(check, SeverityWarning, "README.md is very short; add installation and configuration instructions")
	}

	/* We currently do not render README content, it seems to be impossible to control the content of 500 themes. */
	/*if refs := findRelativeImageRefs(content); len(refs) > 0 {
		if len(refs) > 3 {
			refs = refs[:3]
		}
		r.add(check, SeverityWarning, "README.md uses relative image path(s) (%s); use absolute URLs so images render on themes.gohugo.io", strings.Join(refs, ", "))
	}*/
	if len(r.Results) == numResults {
		r.add(check, SeverityOK, "found")
	}
}

var (
	mdImageRe   = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^)\s]+)`)
	htmlImageRe = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)
)

// findRelativeImageRefs returns image references in md that are not
// absolute URLs.
func findRelativeImageRefs(md string) []string {
	var refs []string
	seen := make(map[string]bool)
	add := func(ref string) {
		if ref == "" || seen[ref] {
			return
		}
		for _, prefix := range []string{"http://", "https://", "//", "data:"} {
			if strings.HasPrefix(ref, prefix) {
				return
			}
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	for _, match := range mdImageRe.FindAllStringSubmatch(md, -1) {
		add(match[1])
	}
	for _, match := range htmlImageRe.FindAllStringSubmatch(md, -1) {
		add(match[1])
	}
	return refs
}

func (c *Config) checkHugoConfig(ctx context.Context, r *Report, m *client.Module, siteDir string) {
	const check = "config"
	exists := func(names ...string) string {
		for _, name := range names {
			if _, err := os.Stat(filepath.Join(m.Dir, name)); err == nil {
				return name
			}
		}
		return ""
	}

	switch {
	case exists("hugo.toml", "hugo.yaml", "hugo.yml", "hugo.json") != "":
		r.add(check, SeverityOK, exists("hugo.toml", "hugo.yaml", "hugo.yml", "hugo.json"))
	case exists("config") != "":
		r.add(check, SeverityOK, "config/")
	case exists("config.toml", "config.yaml", "config.yml", "config.json") != "":
		r.add(check, SeverityWarning, "%s uses the legacy config filename; rename it to hugo.toml", exists("config.toml", "config.yaml", "config.yml", "config.json"))
	default:
		r.add(check, SeverityWarning, "no Hugo config file found in the theme root")
	}

	hv, err := themeHugoVersion(ctx, siteDir)
	if err != nil {
		r.add("hugoVersion", SeverityWarning, "failed to read the merged site config: %s", err)
		return
	}
	if hv.Min == "" {
		r.add("hugoVersion", SeverityWarning, "no module.hugoVersion.min set in the theme config")
	} else {
		msg := "min " + hv.Min
		if hv.Max != "" {
			msg += ", max " + hv.Max
		}
		r.add("hugoVersion", SeverityOK, msg)
	}
}

// hugoConfig holds the fields we need from "hugo config --format json".
type hugoConfig struct {
	Module struct {
		HugoVersion client.HugoVersion `json:"hugoversion"`
	} `json:"module"`
}

// mergedConfigFilename is the site config that deep-merges the theme's own
// config; see resolveModule.
const mergedConfigFilename = "config-merged.json"

// themeHugoVersion returns module.hugoVersion from the merged config of
// the site in siteDir, which imports the theme and deep-merges its config
// ("hugo config mounts" does not report it).
func themeHugoVersion(ctx context.Context, siteDir string) (client.HugoVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, client.HugoBinary(), "--config", mergedConfigFilename, "config", "--format", "json")
	cmd.Dir = siteDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return client.HugoVersion{}, fmt.Errorf("hugo config failed: %s\n%s", err, strings.Join(tailLines(stderr.String(), 5), "\n"))
	}
	var cfg hugoConfig
	if err := json.Unmarshal(stdout.Bytes(), &cfg); err != nil {
		return client.HugoVersion{}, fmt.Errorf("failed to decode hugo config output: %s", err)
	}
	return cfg.Module.HugoVersion, nil
}

func (c *Config) checkMetaURLs(ctx context.Context, r *Report, m *client.Module) {
	cl := c.rootConfig.Client
	if cl == nil || !cl.OutFileExists("badhosts.txt") {
		return
	}
	for _, key := range []string{"demosite", "homepage"} {
		v, found := m.Meta[key]
		if !found {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		if cl.IsBadURL(s) {
			r.add(key, SeverityError, "%s points to a blocked or non-existing host", s)
			continue
		}
		if status, err := checkURLExists(ctx, s); err != nil {
			r.add(key, SeverityWarning, "%s is not reachable: %s", s, err)
		} else if status >= 400 {
			r.add(key, SeverityWarning, "%s returned HTTP status %d", s, status)
		} else {
			r.add(key, SeverityOK, s)
		}
	}
}

// checkURLExists issues a HEAD request for the given URL, falling back to
// GET (which some servers require), and returns the HTTP status code.
func checkURLExists(ctx context.Context, url string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	status, err := doURLRequest(ctx, http.MethodHead, url)
	if err == nil && status < 400 {
		return status, nil
	}
	return doURLRequest(ctx, http.MethodGet, url)
}

func doURLRequest(ctx context.Context, method, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "hugothemesitebuilder/check (+https://themes.gohugo.io/)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
