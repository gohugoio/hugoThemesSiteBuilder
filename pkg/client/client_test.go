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
	})
}
