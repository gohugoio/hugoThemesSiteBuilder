package buildcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
	"github.com/peterbourgon/ff/v3/ffcli"
	"gopkg.in/yaml.v3"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/rootcmd"
)

// Config for the get subcommand.
type Config struct {
	rootConfig *rootcmd.Config
}

// New returns a usable ffcli.Command for the get subcommand.
func New(rootConfig *rootcmd.Config) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
	}

	fs := flag.NewFlagSet(rootcmd.CommandName+" build", flag.ExitOnError)

	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "build",
		ShortUsage: rootcmd.CommandName + " build [flags] <action>",
		ShortHelp:  "Build re-creates the themes site's content based on themes.txt and go.mod.",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	const configAll = "config.json"
	client := &buildClient{Client: c.rootConfig.Client}

	if err := client.CreateThemesConfig(); err != nil {
		return err
	}

	if !client.OutFileExists("go.mod") {
		// Initialize the Hugo Module
		if err := client.InitModule(configAll); err != nil {
			return err
		}
	}

	mmap, err := client.GetHugoModulesMap(configAll)
	if err != nil {
		return err
	}

	if err := client.writeThemesContent(mmap); err != nil {
		return err
	}

	return nil

}

type buildClient struct {
	*client.Client

	mmap client.ModulesMap
}

func (c *buildClient) writeThemesContent(mm client.ModulesMap) error {
	githubrepos, err := c.GetGitHubRepos(mm)
	if err != nil {
		return err
	}
	maxStars := 0
	for _, ghRepo := range githubrepos {
		if ghRepo.Stars > maxStars {
			maxStars = ghRepo.Stars
		}
	}

	contentDir := c.JoinOutPath("site", "content")
	client.CheckErr(os.RemoveAll(contentDir))
	client.CheckErr(os.MkdirAll(contentDir, 0777))

	for k, m := range mm {

		themeName := strings.ToLower(path.Base(k))

		themeDir := filepath.Join(contentDir, "themes", themeName)
		client.CheckErr(os.MkdirAll(themeDir, 0777))

		copyIfExists := func(sourcePath, targetPath string) {
			fs, err := os.Open(filepath.Join(m.Dir, sourcePath))
			if err != nil {
				return
			}
			defer fs.Close()
			targetFilename := filepath.Join(themeDir, targetPath)
			client.CheckErr(os.MkdirAll(filepath.Dir(targetFilename), 0777))
			ft, err := os.Create(targetFilename)
			client.CheckErr(err)
			defer ft.Close()

			_, err = io.Copy(ft, fs)
			client.CheckErr(err)
		}

		fixReadMeContent := func(s string) string {
			// Tell Hugo not to process shortcode samples
			s = regexp.MustCompile(`(?s){\{%([^\/].*?)%\}\}`).ReplaceAllString(s, `{{%/*$1*/%}}`)
			s = regexp.MustCompile(`(?s){\{<([^\/].*?)>\}\}`).ReplaceAllString(s, `{{</*$1*/>}}`)

			return s
		}

		getReadMeContent := func() string {
			files, err := os.ReadDir(m.Dir)
			client.CheckErr(err)
			for _, fi := range files {
				if fi.IsDir() {
					continue
				}
				if strings.EqualFold(fi.Name(), "readme.md") {
					b, err := ioutil.ReadFile(filepath.Join(m.Dir, fi.Name()))
					client.CheckErr(err)
					return fixReadMeContent(string(b))
				}
			}
			return ""
		}

		title := strings.Title(themeName)
		readMeContent := getReadMeContent()
		ghRepo := githubrepos[m.Path]

		// 30 days.
		d30 := 30 * 24 * time.Hour
		const boost = 50

		// Higher is better.
		weight := maxStars + 500
		weight -= ghRepo.Stars
		// Boost themes updated recently.
		if !m.Time.IsZero() {
			// Add some weight to recently updated themes.
			age := time.Since(m.Time)
			if age < (3 * d30) {
				weight -= (boost * 2)
			} else if age < (6 * d30) {
				weight -= boost
			}
		}

		// Boost themes with a Hugo version indicator set that covers.
		// the current Hugo version.
		if m.HugoVersion.IsValid() {
			weight -= boost
		}

		// TODO(bep) we don't build any demo site anymore, but
		// we could and should probably build a simple site and
		// count warnings and error and use that to
		// either pull it down the list with weight or skip it.

		//c.Logf("Processing theme %q with weight %d", themeName, weight)

		// TODO1 tags, normalized.

		frontmatter := map[string]interface{}{
			"title":       title,
			"slug":        themeName,
			"aliases":     []string{"/" + themeName},
			"weight":      weight,
			"lastMod":     m.Time,
			"hugoVersion": m.HugoVersion,
			"meta":        m.Meta,
			"githubInfo":  ghRepo,
		}

		b, err := yaml.Marshal(frontmatter)
		client.CheckErr(err)

		content := fmt.Sprintf(`---
%s
---
%s
`, string(b), readMeContent)

		if err := ioutil.WriteFile(filepath.Join(themeDir, "index.md"), []byte(content), 0666); err != nil {
			return err
		}

		copyIfExists("images/tn.png", "tn-featured.png")
		copyIfExists("images/screenshot.png", "screenshot.png")

	}

	return nil
}
