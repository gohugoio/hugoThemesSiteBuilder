package checkcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestAspectRatioOK(t *testing.T) {
	c := qt.New(t)

	c.Assert(aspectRatioOK(900, 600), qt.Equals, true)
	c.Assert(aspectRatioOK(1500, 1000), qt.Equals, true)
	// Slightly off, but within tolerance.
	c.Assert(aspectRatioOK(1512, 1000), qt.Equals, true)
	c.Assert(aspectRatioOK(1920, 1080), qt.Equals, false)
	c.Assert(aspectRatioOK(600, 900), qt.Equals, false)
	c.Assert(aspectRatioOK(100, 0), qt.Equals, false)
}

func TestFindRelativeImageRefs(t *testing.T) {
	c := qt.New(t)

	md := `
# My Theme

![Screenshot](images/screenshot.png)
![Absolute](https://raw.githubusercontent.com/user/repo/main/images/tn.png)
![Rooted](/images/tn.png)
<img src="assets/logo.jpg" alt="logo">
<img src="//example.org/logo.png">
![Dupe](images/screenshot.png)
`
	c.Assert(findRelativeImageRefs(md), qt.DeepEquals, []string{
		"images/screenshot.png",
		"/images/tn.png",
		"assets/logo.jpg",
	})

	c.Assert(findRelativeImageRefs("no images here"), qt.IsNil)
}

func TestParsePagesCount(t *testing.T) {
	c := qt.New(t)

	out := `
                   | EN
-------------------+-----
  Pages            | 24
  Paginator pages  |  0
`
	c.Assert(parsePagesCount(out), qt.Equals, 24)
	c.Assert(parsePagesCount("no stats"), qt.Equals, 0)
}

func TestMissingLayoutKinds(t *testing.T) {
	c := qt.New(t)

	out := `
WARN  found no layout file for "html" for kind "home": You should create a template file.
WARN  found no layout file for "html" for kind "page": You should create a template file.
WARN  found no layout file for "html" for kind "home": Duplicate.
`
	c.Assert(missingLayoutKinds(out), qt.DeepEquals, []string{"home", "page"})
	c.Assert(missingLayoutKinds("all good"), qt.IsNil)
}

func TestCollectNpmDependencies(t *testing.T) {
	c := qt.New(t)

	dir := t.TempDir()
	write := func(name, content string) {
		filename := filepath.Join(dir, filepath.FromSlash(name))
		c.Assert(os.MkdirAll(filepath.Dir(filename), 0o777), qt.IsNil)
		c.Assert(os.WriteFile(filename, []byte(content), 0o666), qt.IsNil)
	}

	packages, err := collectNpmDependencies(dir)
	c.Assert(err, qt.IsNil)
	c.Assert(packages, qt.IsNil)

	write("package.json", `{
		"dependencies": {"tailwindcss": "^4.0.0"},
		"workspaces": ["packages/hugoautogen"]
	}`)
	write("packages/hugoautogen/package.json", `{
		"devDependencies": {"postcss": "^8.0.0", "autoprefixer": "^10.0.0", "tailwindcss": "^4.0.0"}
	}`)

	packages, err = collectNpmDependencies(dir)
	c.Assert(err, qt.IsNil)
	c.Assert(packages, qt.DeepEquals, []string{"autoprefixer", "postcss", "tailwindcss"})

	write("package.json", `not json`)
	_, err = collectNpmDependencies(dir)
	c.Assert(err, qt.IsNotNil)
}

func TestDisallowedNpmPackages(t *testing.T) {
	c := qt.New(t)

	c.Assert(disallowedNpmPackages([]string{"postcss", "tailwindcss"}), qt.IsNil)
	c.Assert(disallowedNpmPackages([]string{"left-pad", "postcss", "some-exotic-package"}), qt.DeepEquals, []string{"left-pad", "some-exotic-package"})
}

func TestTailLines(t *testing.T) {
	c := qt.New(t)

	c.Assert(tailLines("a\nb\nc\n", 2), qt.DeepEquals, []string{"b", "c"})
	c.Assert(tailLines("a\nb", 5), qt.DeepEquals, []string{"a", "b"})
}

func TestHugoConfigDecode(t *testing.T) {
	c := qt.New(t)

	var cfg hugoConfig
	c.Assert(json.Unmarshal([]byte(`{"module": {"hugoversion": {"min": "0.146.0", "max": "0.999.0"}}}`), &cfg), qt.IsNil)
	c.Assert(cfg.Module.HugoVersion.Min, qt.Equals, "0.146.0")
	c.Assert(cfg.Module.HugoVersion.Max, qt.Equals, "0.999.0")
}
