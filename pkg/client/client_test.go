package client

import (
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
