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
	"sort"
	"strings"
	"time"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
	"github.com/peterbourgon/ff/v3/ffcli"
	"gopkg.in/yaml.v3"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/rootcmd"
)

// Config for the get subcommand.
type Config struct {
	// Do not delete the old /content folder
	// This is useful when doing theme edits. Hugo gets confused
	// when the entire content vanishes.
	noClean    bool
	rootConfig *rootcmd.Config
}

// New returns a usable ffcli.Command for the get subcommand.
func New(rootConfig *rootcmd.Config) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
	}

	fs := flag.NewFlagSet(rootcmd.CommandName+" build", flag.ExitOnError)
	fs.BoolVar(&cfg.noClean, "noClean", false, "do not clean out /content before building")

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

	if err := client.writeThemesContent(mmap, c.noClean); err != nil {
		return err
	}

	return nil

}

type buildClient struct {
	*client.Client

	mmap client.ModulesMap
}

func (c *buildClient) writeThemesContent(mm client.ModulesMap, noClean bool) error {
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
	if !noClean {
		client.CheckErr(os.RemoveAll(contentDir))
	}
	client.CheckErr(os.MkdirAll(contentDir, 0777))

	type themeWarning struct {
		theme string
		warn  warning
	}

	themeWarningsAll := make(map[themeWarning]bool)

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
			// s = regexp.MustCompile(`(?s)github\.com\/(.*?)\/blob\/master\/images/raw\.githubusercontent\.com`).ReplaceAllString(s, `/$1/master/images/`)

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

		var title string
		if mn, ok := m.Meta["name"]; ok {
			title = mn.(string)
		} else {
			title = strings.Title(themeName)
		}
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
		// TODO(bep) I removed Hugo as a dependency,
		// compared this against HUGO_VERSION somehow.
		/*if m.HugoVersion.IsValid() {
			weight -= boost
		}*/

		// TODO(bep) we don't build any demo site anymore, but
		// we could and should probably build a simple site and
		// count warnings and error and use that to
		// either pull it down the list with weight or skip it.

		// Add warnings for old themes, bad URLs etc.

		draft := false

		lastMod := m.Time

		if warn, found := checkLastMod(lastMod); found {
			if warn.level == errorLevelBlock {
				draft = true
			}
			themeWarningsAll[themeWarning{theme: k, warn: warn}] = true
		}

		for _, metaSiteKey := range []string{"demosite", "homepage"} {
			// TODO(bep) author sites + redirects?
			if s, found := m.Meta[metaSiteKey]; found {
				if c.IsBadURL(s.(string)) {
					themeWarningsAll[themeWarning{theme: k, warn: themeWarningBadURL}] = true

					// Remove it from the map.
					delete(m.Meta, metaSiteKey)
				}
			}
		}

		var themeWarnings []string
		for v, _ := range themeWarningsAll {
			if v.theme != k {
				continue
			}
			themeWarnings = append(themeWarnings, v.warn.message)
		}
		sort.Strings(themeWarnings)

		frontmatter := map[string]interface{}{
			"draft":         draft,
			"title":         title,
			"slug":          themeName,
			"aliases":       []string{"/" + themeName},
			"weight":        weight,
			"lastMod":       lastMod,
			"hugoVersion":   m.HugoVersion,
			"modulePath":    k,
			"meta":          m.Meta,
			"githubInfo":    ghRepo,
			"themeWarnings": themeWarnings,
			"tags":          normalizeTags(m.Meta["tags"]),
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

	var warnCount, blockedCount int

	for w, _ := range themeWarningsAll {
		if w.warn.level == errorLevelWarn {
			warnCount++
		} else {
			blockedCount++
		}
	}

	if warnCount > 0 {
		fmt.Printf("\n%d warnings were applied to the themes. See below.\n", warnCount)
	}

	if blockedCount > 0 {
		fmt.Printf("\n%d themes were blocked (draft=true). See below.", blockedCount)
	}

	fmt.Println()

	for w, _ := range themeWarningsAll {
		levelString := "warning"
		if w.warn.level == errorLevelBlock {
			levelString = "block"
		}
		fmt.Printf("%s: %s: %s\n", levelString, w.theme, w.warn.message)
	}

	return nil
}

func checkLastMod(lastMod time.Time) (warn warning, found bool) {
	if !lastMod.IsZero() {
		age := time.Since(lastMod)
		ageYears := age.Hours() / 24 / 365
		if ageYears > 2 {
			warn = themeWarningOld
			found = true
		}
	}
	return
}

type errorLevel int

const (
	errorLevelWarn errorLevel = iota + 1
	errorLevelBlock
)

type warning struct {
	message string
	level   errorLevel
}

func (w warning) IsZero() bool {
	return w.message == ""
}

var (
	// Not updated for the last 2 years.
	themeWarningOld = warning{
		level:   errorLevelWarn,
		message: "This theme has not been updated for more than 2 years.",
	}

	themeWarningBadURL = warning{
		level:   errorLevelWarn,
		message: "This theme links to one or more blocked or non-existing sites.",
	}
)

func normalizeTags(in interface{}) []string {
	if in == nil {
		return nil
	}

	tagsin := in.([]interface{})
	var tagsout []string

	for _, tag := range tagsin {
		normalized := normalizeTag(tag.(string))
		if normalized != "" {
			tagsout = append(tagsout, normalized)
		}
	}

	return uniqueStringsSorted(tagsout)
}

var goodTags = map[string]bool{
	"api":          true,
	"blog":         true,
	"bootstrap":    true,
	"company":      true,
	"dark":         true,
	"ecommerce":    true,
	"gallery":      true,
	"green":        true,
	"light":        true,
	"multilingual": true,
	"newsletter":   true,
	"portfolio":    true,
	"white":        true,
	"agency":       true,
	"personal":     true,
	"archives":     true,
	"book":         true,
	"church":       true,
	"education":    true,
	"magazine":     true,
	"responsive":   true,
	"pink":         true,
}

func normalizeTag(s string) string {
	s = strings.ToLower(s)

	if goodTags[s] {
		return s
	}

	ca := func(candidates ...string) bool {
		for _, candidate := range candidates {
			if strings.Contains(s, candidate) {
				return true
			}
		}
		return false
	}

	if ca("docs", "document") {
		return "docs"
	}

	if ca("blog") {
		return "blog"
	}

	if ca("contact") {
		return "contact"
	}

	if ca("bootstrap") {
		return "bootstrap"
	}

	return ""

}

/*
All tags currently in use:

 API
Academic
Academicons
AlexFinn
Blog
Bootstrap
Bootstrap v4
CSS Grid
Clean
Company
Contact Form
Creative Tim
Custom Themes
Dark
DevFest
Disqus
Docs
Documentation
Ecommerce
Elate
Fancybox
Font Awesome
Fontawesome
Gallery
Google Analytics
Google News
Google analytics
Green
HTML5
Highlight.js
Hugo
Invision
Jquery
Lato
Light
Material Design
Minimal
Minimalist
Mobile
Modern
Multilingual
Netlify
Newsletter
Octopress
Open Graph
Pacman
Personal
Pink
Portfolio
Presentation
Product
Projects
Responsive
Roboto
Roboto Slab
Simple
Single Product
Skel
Slide
Sortable Tables
Stackbit
Starter
Staticman
Syntax Highlighting
Syntax highlighting
Table Of Contents
Tachyons
Tags
Technical
Themefisher
Twitter Cards
Typography
White
academic
accessibility
accessible
accordion
agency
agency-template
allegiant
amp
archives
articles
avatar
bang
beautiful
black white
blank
blog
blog, responsive, personal, bootstrap, disqus, google analytics, syntax highligting, font awesome, landing page, flexbox
blogdown
bluma
book
bookmarking
bootstrap
bootstrap4
bulma
business
card
cards
carousel
case study
catalogue
changelog
church
clean
clients
cms
collections
color configuration
colors
colour schemes
commento
comming-soon
company
conference
configurable
contact
contact form
contact-form
content management
cooking
copyright
core
creative
css grid
css only
custom themes
custom-design
custom-themes
customizable
cv
dark
dark mode
data files
debug
developer
development
devicon
disqus
doc
docs
document
documentation
donggeun
ecommerce
edidor
editor
education
elegant
experience
fancybox 3
faq
fast
feather
flat-ui
flex
flexbox
flip
font awesome
font-awesome
fontawesome
foundation
freelancer
freenlancer
fresh
gallery
gethugothemes
ghost
google adsense
google analytics
google fonts
google tag manager
google-analytics
gradients
graphcomment
graphical
grav
grid
hero
high contrast
highlight
highlight.js
highlighting
home
html5
html5up
hugo
hugo templates
hugo themes
hugo-templates
hugo-theme
hyde
i18n
icon
illustrations
images
informal
isso
jekyll
jekyll-now
jssocials
kube
l10n
lander
landing
landing page
landing-page
landingpage
launch page
learn
light
light mode
linkblog
lodi
lubang
lulab
magazine
marketing
masonry layout
material design
material-design
micro
microblog
mimimalist
minimal
minimalist
minimalistic
mobile
modern
modern design
monochromatic
monospace
monotone
motto
multi page
multilingual
multipage
neat
netlify
night-mode
no-javascript
nojs
normalize
offline
one page
one-page
onepage
opensource
page
pages
pagination
paper
parallax
personal
personal-website
photoblog
photography
pixel
plain
podcast
portfolio
post
postimage
posts
premium
presentation
privacy
product
product catalogue
products
professional
profile
programmer
projects
purecss
pygments
readable
reading
recipes
responsive
resume
retro
revealjs
rss
rstats
search
seo
sepia
services
share this
shopping
shortcuts
showcase
simple
simple page
single page
single product
single-page
singlepage
skills
slide
slider
social
social links
solarized
somratpro
spa
spectre css framework
speed-dial
starter
staticman
syntax highlighting
syntax sighlighting
syntax-highlighting
tachyons
tags
tailwindcss
technical
terminal
theme
themefisher
themes
timeline
two-column
typography
uicardio
university
unix
ux
w3css
website
white
widgets
wiki-like
wordpress
zerostatic
*/

func uniqueStringsSorted(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	ss := sort.StringSlice(s)
	ss.Sort()
	i := 0
	for j := 1; j < len(s); j++ {
		if !ss.Less(i, j) {
			continue
		}
		i++
		s[i] = s[j]
	}

	return s[:i+1]
}
