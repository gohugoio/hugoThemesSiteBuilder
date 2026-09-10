package client

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestModule(t *testing.T) {
	c := qt.New(t)

	c.Run("PathRepo", func(c *qt.C) {
		c.Assert(Module{Path: "github.com/a/b/c/d/e"}.PathRepo(), qt.Equals, "github.com/a/b")
		c.Assert(Module{Path: "github.com/a/b/c/d"}.PathRepo(), qt.Equals, "github.com/a/b")
		c.Assert(Module{Path: "github.com/a/b"}.PathRepo(), qt.Equals, "github.com/a/b")
		c.Assert(Module{Path: "github.com/a"}.PathRepo(), qt.Equals, "github.com/a")
		c.Assert(Module{Path: "github.com"}.PathRepo(), qt.Equals, "github.com")
		c.Assert(Module{Path: "github.com/a/v2"}.PathRepo(), qt.Equals, "github.com/a")
	})
}

func TestPathWithoutVersion(t *testing.T) {
	c := qt.New(t)

	c.Assert(PathWithoutVersion("github.com/gohugoio/hugo/v3"), qt.Equals, "github.com/gohugoio/hugo")
	c.Assert(PathWithoutVersion("github.com/gohugoio/hugo/v2"), qt.Equals, "github.com/gohugoio/hugo")
	c.Assert(PathWithoutVersion("github.com/gohugoio/hugo"), qt.Equals, "github.com/gohugoio/hugo")
}

func TestThemesFiles(t *testing.T) {
	c := qt.New(t)

	// The themes files live three levels above the out dir.
	root := t.TempDir()
	outDir := filepath.Join(root, "cmd", "hugothemesitebuilder", "build")
	c.Assert(os.MkdirAll(outDir, 0o777), qt.IsNil)
	cl, err := New(io.Discard, outDir)
	c.Assert(err, qt.IsNil)

	c.Assert(os.WriteFile(filepath.Join(root, ThemesTxt), []byte("github.com/a/one\ngithub.com/b/two\ngithub.com/c/three\n"), 0o666), qt.IsNil)

	paths, err := cl.ReadThemesFile(ThemesTxt)
	c.Assert(err, qt.IsNil)
	c.Assert(paths, qt.DeepEquals, []string{"github.com/a/one", "github.com/b/two", "github.com/c/three"})

	// Missing file is empty.
	paths, err = cl.ReadThemesFile(ThemesBitrotTxt)
	c.Assert(err, qt.IsNil)
	c.Assert(paths, qt.IsNil)

	// Move two to bitrot: removed from themes.txt, inserted (sorted,
	// case-insensitively) into a new themes.bitrot.txt with a header.
	c.Assert(cl.UpdateThemesFile(ThemesTxt, "", []string{"github.com/b/two"}, nil), qt.IsNil)
	c.Assert(cl.UpdateThemesFile(ThemesBitrotTxt, "a header", nil, []string{"github.com/b/two"}), qt.IsNil)
	c.Assert(cl.UpdateThemesFile(ThemesBitrotTxt, "a header", nil, []string{"github.com/AAA/first"}), qt.IsNil)

	paths, err = cl.ReadThemesFile(ThemesTxt)
	c.Assert(err, qt.IsNil)
	c.Assert(paths, qt.DeepEquals, []string{"github.com/a/one", "github.com/c/three"})

	b, err := os.ReadFile(filepath.Join(root, ThemesBitrotTxt))
	c.Assert(err, qt.IsNil)
	c.Assert(string(b), qt.Equals, "# a header\ngithub.com/AAA/first\ngithub.com/b/two\n")

	// Move one back.
	c.Assert(cl.UpdateThemesFile(ThemesBitrotTxt, "a header", []string{"github.com/b/two"}, nil), qt.IsNil)
	c.Assert(cl.UpdateThemesFile(ThemesTxt, "", nil, []string{"github.com/b/two"}), qt.IsNil)
	paths, err = cl.ReadThemesFile(ThemesTxt)
	c.Assert(err, qt.IsNil)
	c.Assert(paths, qt.DeepEquals, []string{"github.com/a/one", "github.com/b/two", "github.com/c/three"})
}
