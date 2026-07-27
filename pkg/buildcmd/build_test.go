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
