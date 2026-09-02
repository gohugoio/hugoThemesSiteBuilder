package checkcmd

import (
	"fmt"
	"io"
	"strings"
)

// Severity is the severity of a check result.
type Severity string

const (
	SeverityOK      Severity = "ok"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Result is the result of a single check.
type Result struct {
	Check    string   `json:"check"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Report holds all check results for one theme.
type Report struct {
	Module  string   `json:"module"`
	Results []Result `json:"results"`
	Pass    bool     `json:"pass"`
}

func (r *Report) add(check string, severity Severity, format string, a ...interface{}) {
	r.Results = append(r.Results, Result{
		Check:    check,
		Severity: severity,
		Message:  fmt.Sprintf(format, a...),
	})
}

func (r *Report) counts() (errors, warnings int) {
	for _, res := range r.Results {
		switch res.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		}
	}
	return
}

func (r *Report) finalize() {
	errors, _ := r.counts()
	r.Pass = errors == 0
}

// render writes the report as Markdown, suitable for pasting into e.g. a
// GitHub PR comment. Multi-line messages show their first line in the table
// and the rest in a <details> block below it.
func (r *Report) render(w io.Writer) {
	fmt.Fprintf(w, "\n### %s\n\n", r.Module)
	fmt.Fprintln(w, "|    | Check | Comment |")
	fmt.Fprintln(w, "|----|-------|---------|")
	type detail struct {
		icon, check, body string
	}
	var details []detail
	for _, res := range r.Results {
		icon := "🟢"
		switch res.Severity {
		case SeverityWarning:
			icon = "🟡"
		case SeverityError:
			icon = "🔴"
		}
		message := res.Message
		if first, rest, found := strings.Cut(message, "\n"); found {
			message = strings.TrimSuffix(strings.TrimSpace(first), ":")
			details = append(details, detail{icon, res.Check, rest})
		}
		fmt.Fprintf(w, "| %s | %s | %s |\n", icon, res.Check, markdownCell(message))
	}
	errors, warnings := r.counts()
	status := "✅ **PASS**"
	if errors > 0 {
		status = "❌ **FAIL**"
	}
	fmt.Fprintf(w, "\n%s (%d error(s), %d warning(s))\n", status, errors, warnings)
	for _, d := range details {
		fmt.Fprintf(w, "\n<details>\n<summary>%s %s</summary>\n\n```\n%s\n```\n\n</details>\n",
			d.icon, d.check, strings.TrimSpace(d.body))
	}
	for _, res := range r.Results {
		if res.Check == "build" && res.Severity != SeverityOK {
			fmt.Fprintf(w, "\nTo get some help fixing Hugo deprecation warnings or errors, see [myhugofixer](https://github.com/bep/myhugofixer).\n")
			break
		}
	}
}

// markdownCell makes s safe for use in a Markdown table cell.
func markdownCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
