package checkcmd

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

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
	if refs := findRelativeImageRefs(content); len(refs) > 0 {
		if len(refs) > 3 {
			refs = refs[:3]
		}
		r.add(check, SeverityWarning, "README.md uses relative image path(s) (%s); use absolute URLs so images render on themes.gohugo.io", strings.Join(refs, ", "))
	}
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

func (c *Config) checkHugoConfig(r *Report, m *client.Module) {
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

	if m.HugoVersion.Min == "" {
		r.add("hugoVersion", SeverityWarning, "no module.hugoVersion.min set in the theme config")
	} else {
		msg := "min " + m.HugoVersion.Min
		if m.HugoVersion.Max != "" {
			msg += ", max " + m.HugoVersion.Max
		}
		r.add("hugoVersion", SeverityOK, msg)
	}
}

func (c *Config) checkMetaURLs(r *Report, m *client.Module) {
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
		} else {
			r.add(key, SeverityOK, s)
		}
	}
}
