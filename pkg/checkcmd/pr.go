package checkcmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
)

var prIDRe = regexp.MustCompile(`^\d+$`)

// expandPRArgs replaces any all-digits argument (a pull request number in
// the gohugoio/hugoThemesSiteBuilder repository) with the theme module
// path(s) that pull request adds to themes.txt. Module paths are passed
// through as-is.
func expandPRArgs(args []string) ([]string, error) {
	var expanded []string
	for _, arg := range args {
		if !prIDRe.MatchString(arg) {
			expanded = append(expanded, arg)
			continue
		}
		pr, err := strconv.Atoi(arg)
		if err != nil {
			return nil, err
		}
		themes, err := themesAddedInPR(pr)
		if err != nil {
			return nil, fmt.Errorf("PR %d: %s", pr, err)
		}
		if len(themes) == 0 {
			return nil, fmt.Errorf("PR %d does not add any lines to %s", pr, client.ThemesTxt)
		}
		fmt.Fprintf(os.Stderr, "PR %d adds: %s\n", pr, strings.Join(themes, ", "))
		expanded = append(expanded, themes...)
	}
	return expanded, nil
}

// themesAddedInPR returns the lines the given pull request adds to
// themes.txt.
func themesAddedInPR(pr int) ([]string, error) {
	files, err := client.GetPullRequestFiles(pr)
	if err != nil {
		return nil, err
	}
	var themes []string
	for _, f := range files {
		if f.Filename != client.ThemesTxt {
			continue
		}
		themes = append(themes, addedLines(f.Patch)...)
	}
	return themes, nil
}

// addedLines returns the non-empty, non-comment lines added in the given
// unified diff patch.
func addedLines(patch string) []string {
	var lines []string
	for _, line := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
