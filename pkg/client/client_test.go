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

func TestHostMatches(t *testing.T) {
	c := qt.New(t)

	hosts := []string{"codeberg.org", "gitlab.com"}
	c.Assert(hostMatches("codeberg.org/user/theme", hosts), qt.IsTrue)
	c.Assert(hostMatches("Codeberg.org/user/theme", hosts), qt.IsTrue)
	c.Assert(hostMatches("gitlab.com/user/theme/v2", hosts), qt.IsTrue)
	c.Assert(hostMatches("github.com/user/theme", hosts), qt.IsFalse)
	c.Assert(hostMatches("codeberg.org", hosts), qt.IsTrue)
	c.Assert(hostMatches("codeberg.org/user/theme", nil), qt.IsFalse)
}

func TestRemoveHostsFromGoMod(t *testing.T) {
	c := qt.New(t)

	dir := t.TempDir()
	client := &Client{logWriter: io.Discard, outDir: dir}

	// No go.mod is fine.
	c.Assert(client.RemoveHostsFromGoMod([]string{"codeberg.org"}), qt.IsNil)

	goMod := `module example.com/build

go 1.27.0

require codeberg.org/single/theme v0.1.0

require (
	codeberg.org/user/theme v0.0.0-20260321101414-d37fb5f2a994 // indirect
	// codeberg.org/user/commented v0.1.0
	github.com/user/theme v1.1.36 // indirect
	gitlab.com/user/theme v0.2.0 // indirect
)
`
	filename := filepath.Join(dir, "go.mod")
	c.Assert(os.WriteFile(filename, []byte(goMod), 0o666), qt.IsNil)

	// Nothing to skip leaves the file untouched.
	c.Assert(client.RemoveHostsFromGoMod(nil), qt.IsNil)
	b, err := os.ReadFile(filename)
	c.Assert(err, qt.IsNil)
	c.Assert(string(b), qt.Equals, goMod)

	c.Assert(client.RemoveHostsFromGoMod([]string{"codeberg.org"}), qt.IsNil)
	b, err = os.ReadFile(filename)
	c.Assert(err, qt.IsNil)
	c.Assert(string(b), qt.Equals, `module example.com/build

go 1.27.0


require (
	// codeberg.org/user/commented v0.1.0
	github.com/user/theme v1.1.36 // indirect
	gitlab.com/user/theme v0.2.0 // indirect
)
`)
}
