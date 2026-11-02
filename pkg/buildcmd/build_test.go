package buildcmd

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestFixReadmeContent(t *testing.T) {
	c := qt.New(t)

	// Issue #356
	s := `
{{<details "summary title">}}

block content

{{</details>}}
`
	c.Assert(fixReadmeContent(s), qt.Equals, "\n{{</*details \"summary title\"*/>}}\n\nblock content\n\n{{</*/details*/>}}\n")

	s = `
{{% details "summary title" %}}

block content

{{% /details %}}
`

	c.Assert(fixReadmeContent(s), qt.Equals, "\n{{%/* details \"summary title\" */%}}\n\nblock content\n\n{{%/* /details */%}}\n")
}

func TestSplitHosts(t *testing.T) {
	c := qt.New(t)

	c.Assert(splitHosts(""), qt.IsNil)
	c.Assert(splitHosts(" , "), qt.IsNil)
	c.Assert(splitHosts("codeberg.org"), qt.DeepEquals, []string{"codeberg.org"})
	c.Assert(splitHosts(" codeberg.org, gitlab.com ,"), qt.DeepEquals, []string{"codeberg.org", "gitlab.com"})
}
