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

func (r *Report) render(w io.Writer) {
	fmt.Fprintf(w, "\n%s\n%s\n", r.Module, strings.Repeat("-", len(r.Module)))
	for _, res := range r.Results {
		label := "ok"
		switch res.Severity {
		case SeverityWarning:
			label = "WARN"
		case SeverityError:
			label = "ERROR"
		}
		message := res.Message
		if strings.Contains(message, "\n") {
			message = "\n" + indent(message, "        ")
		}
		fmt.Fprintf(w, "  %-6s %-18s %s\n", label, res.Check, message)
	}
	errors, warnings := r.counts()
	status := "PASS"
	if errors > 0 {
		status = "FAIL"
	}
	fmt.Fprintf(w, "  RESULT %s (%d error(s), %d warning(s))\n", status, errors, warnings)
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
