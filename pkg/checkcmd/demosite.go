package checkcmd

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
)

//go:embed all:demosite
var demoSiteFS embed.FS

// buildDemoSite creates a small demo site in workDir that imports the given
// theme module, builds it with Hugo and reports errors and warnings.
func (c *Config) buildDemoSite(ctx context.Context, r *Report, workDir, modulePath string) {
	const check = "build"

	siteDir := filepath.Join(workDir, "demosite")
	sub, err := fs.Sub(demoSiteFS, "demosite")
	if err != nil {
		r.add(check, SeverityError, "failed to open embedded demo site: %s", err)
		return
	}
	if err := os.CopyFS(siteDir, sub); err != nil {
		r.add(check, SeverityError, "failed to create demo site: %s", err)
		return
	}

	config := map[string]interface{}{
		"baseURL": "https://example.org/",
		"title":   "Hugo Theme Check Demo",
		"module": map[string]interface{}{
			"imports": []map[string]interface{}{
				{"path": modulePath},
			},
		},
	}
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		r.add(check, SeverityError, "failed to marshal demo site config: %s", err)
		return
	}
	if err := os.WriteFile(filepath.Join(siteDir, "config.json"), b, 0o666); err != nil {
		r.add(check, SeverityError, "failed to write demo site config: %s", err)
		return
	}

	cl, err := client.New(c.logWriter(), siteDir)
	if err != nil {
		r.add(check, SeverityError, "failed to create client: %s", err)
		return
	}
	if err := cl.InitModule(); err != nil {
		r.add(check, SeverityError, "failed to init demo site module: %s", err)
		return
	}

	out, err := runHugoCapture(ctx, siteDir)
	if err != nil {
		r.add(check, SeverityError, "demo site build failed:\n%s", strings.Join(tailLines(out, 15), "\n"))
		return
	}

	message := "demo site built"
	if pages := parsePagesCount(out); pages > 0 {
		message += " (" + strconv.Itoa(pages) + " pages)"
	}
	if missing := missingLayoutKinds(out); len(missing) > 0 {
		r.add(check, SeverityError, "%s, but the theme has no layout for kind(s): %s", message, strings.Join(missing, ", "))
		return
	}
	if warnings := linesWithPrefix(out, "WARN"); len(warnings) > 0 {
		numWarnings := len(warnings)
		if numWarnings > 5 {
			warnings = warnings[:5]
		}
		r.add(check, SeverityWarning, "%s with %d warning(s):\n%s", message, numWarnings, strings.Join(warnings, "\n"))
		return
	}
	r.add(check, SeverityOK, "%s, no warnings", message)
}

func runHugoCapture(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hugo", args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func linesWithPrefix(s, prefix string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, prefix) {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func tailLines(s string, n int) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

var noLayoutRe = regexp.MustCompile(`found no layout file for "html" for kind "(\w+)"`)

// missingLayoutKinds returns the page kinds Hugo could not find a layout
// for. A theme that cannot render the basic page kinds is not functional.
func missingLayoutKinds(out string) []string {
	var kinds []string
	seen := make(map[string]bool)
	for _, match := range noLayoutRe.FindAllStringSubmatch(out, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			kinds = append(kinds, match[1])
		}
	}
	return kinds
}

var pagesCountRe = regexp.MustCompile(`(?m)^\s*Pages\s*│?\s*[|│]\s*(\d+)`)

// parsePagesCount extracts the page count from Hugo's build stats table,
// returning 0 if not found.
func parsePagesCount(s string) int {
	match := pagesCountRe.FindStringSubmatch(s)
	if match == nil {
		return 0
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return n
}
