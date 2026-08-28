package checkcmd

import (
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

func TestTailLines(t *testing.T) {
	c := qt.New(t)

	c.Assert(tailLines("a\nb\nc\n", 2), qt.DeepEquals, []string{"b", "c"})
	c.Assert(tailLines("a\nb", 5), qt.DeepEquals, []string{"a", "b"})
}
