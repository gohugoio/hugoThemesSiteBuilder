package checkcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
)

const bitrotHeader = `Themes temporarily unpublished because they fail to build with a recent
version of Hugo. Managed by "hugothemesitebuilder check -bitrot"; themes
that start to behave again are moved back to themes.txt.`

// execBitrot checks all themes in themes.txt and themes.bitrot.txt and
// proposes (or, with -write, applies) moves between them: published themes
// that no longer build are quarantined, quarantined themes that build
// again are re-published.
func (c *Config) execBitrot(ctx context.Context) error {
	cl := c.rootConfig.Client

	active, err := cl.ReadThemesFile(client.ThemesTxt)
	if err != nil {
		return err
	}
	quarantined, err := cl.ReadThemesFile(client.ThemesBitrotTxt)
	if err != nil {
		return err
	}
	if len(active) == 0 {
		return errors.New("no themes found in themes.txt")
	}

	all := append(append([]string{}, active...), quarantined...)
	reports := c.checkThemes(ctx, all)
	reportsByModule := make(map[string]*Report)
	for _, r := range reports {
		reportsByModule[r.Module] = r
	}

	var toBitrot, toActive []string
	for _, p := range active {
		if reason := bitrotReason(reportsByModule[p]); reason != "" {
			toBitrot = append(toBitrot, p)
			fmt.Printf("quarantine  %s\n            %s\n", p, reason)
		}
	}
	for _, p := range quarantined {
		if bitrotReason(reportsByModule[p]) == "" {
			toActive = append(toActive, p)
			fmt.Printf("republish   %s\n", p)
		}
	}

	fmt.Printf("\nChecked %d themes (%d published, %d quarantined): %d to quarantine, %d to republish.\n",
		len(all), len(active), len(quarantined), len(toBitrot), len(toActive))

	if !c.write {
		fmt.Println("Dry run; pass -write to apply the moves.")
		return nil
	}

	if err := cl.UpdateThemesFile(client.ThemesTxt, "", toBitrot, toActive); err != nil {
		return err
	}
	if err := cl.UpdateThemesFile(client.ThemesBitrotTxt, bitrotHeader, toActive, toBitrot); err != nil {
		return err
	}
	fmt.Printf("Updated %s and %s.\n", client.ThemesTxt, client.ThemesBitrotTxt)

	return nil
}

// bitrotReason returns a short reason if the theme is considered bit
// rotted: it fails to resolve as a module or its demo site fails to build.
// Other check failures (images, npm, readme etc.) are not bitrot.
func bitrotReason(r *Report) string {
	if r == nil {
		return ""
	}
	for _, res := range r.Results {
		if res.Severity != SeverityError {
			continue
		}
		if res.Check == "module" || res.Check == "build" {
			return res.Check + ": " + firstLine(res.Message)
		}
	}
	return ""
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx != -1 {
		return s[:idx]
	}
	return s
}
