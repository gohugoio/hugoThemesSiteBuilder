package buildcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bep/workers"
	"gopkg.in/yaml.v2"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rogpeppe/go-internal/semver"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/rootcmd"
)

// Config for the build subcommand.
type Config struct {
	// Do not delete the old /content folder
	// This is useful when doing theme edits. Hugo gets confused
	// when the entire content vanishes.
	noClean bool

	// Skip the final site build.
	skipSiteBuild bool

	// Set to true to remove the GitHub cache before building.
	cleanCache bool

	rootConfig *rootcmd.Config
}

// New returns a usable ffcli.Command for the get subcommand.
func New(rootConfig *rootcmd.Config) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
	}

	fs := flag.NewFlagSet(rootcmd.CommandName+" build", flag.ExitOnError)
	fs.BoolVar(&cfg.noClean, "noClean", false, "do not clean out /content before building")
	fs.BoolVar(&cfg.skipSiteBuild, "skipSiteBuild", false, "skip the final site build")
	fs.BoolVar(&cfg.cleanCache, "cleanCache", false, "clean the GitHub cache before building")
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
	contentDir := c.rootConfig.Client.JoinOutPath("site", "content")
	contentThemesDir := filepath.Join(contentDir, "themes")
	staticDir := c.rootConfig.Client.JoinOutPath("site", "static")
	configFile := c.rootConfig.Client.JoinOutPath(configAll)
	os.Remove(configFile)

	bc := &buildClient{
		Client:     c.rootConfig.Client,
		contentDir: contentDir,
		staticDir:  staticDir,
		w:          workers.New(4),
	}

	if !bc.OutFileExists("go.mod") {
		// Initialize the Hugo Module
		if err := bc.InitModule(); err != nil {
			return fmt.Errorf("failed to init module: %w", err)
		}
	}

	if err := bc.CreateThemesConfig(); err != nil {
		return err
	}

	var err error
	bc.mmap, err = bc.GetHugoModulesMap(configAll)
	if err != nil {
		return err
	}

	bc.initGhRepos(c.cleanCache)

	if c.skipSiteBuild {
		return nil
	}

	if !c.noClean {
		client.CheckErr(os.RemoveAll(contentThemesDir))
	}
	client.CheckErr(os.MkdirAll(contentThemesDir, 0o777))

	if err := bc.writeThemesContent(); err != nil {
		return err
	}

	return nil
}

type buildClient struct {
	*client.Client

	w *workers.Workers

	mu        sync.Mutex
	buildErrs []error

	mmap client.ModulesMap

	contentDir string
	staticDir  string

	// Loaded from GitHub
	ghReposInit sync.Once
	ghRepos     map[string]client.GitHubRepo
	maxStars    int
}

type asset struct {
	source string
	target string
}

func (c *buildClient) err(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buildErrs = append(c.buildErrs, err)
}

func (c *buildClient) initGhRepos(cleanCache bool) {
	c.ghReposInit.Do(func() {
		ghRepos, err := c.GetGitHubRepos(c.mmap, cleanCache)
		client.CheckErr(err)
		maxStars := 0
		for _, ghRepo := range ghRepos {
			if ghRepo.Stars > maxStars {
				maxStars = ghRepo.Stars
			}
		}
		c.maxStars = maxStars
		c.ghRepos = ghRepos
	})
}

func (c *buildClient) getGitHubRepo(path string) client.GitHubRepo {
	c.initGhRepos(false)
	return c.ghRepos[path]
}

func (c *buildClient) writeThemesContent() error {
	r, _ := c.w.Start(context.Background())

	for k, m := range c.mmap {
		k := k
		m := m
		r.Run(func() error {
			c.err(c.writeThemeContent(k, m))
			return nil
		})
	}

	err := r.Wait()

	c.Logf("Processed %d themes.", len(c.mmap))

	if err != nil {
	}

	if len(c.buildErrs) > 0 {
		for _, err := range c.buildErrs {
			fmt.Println("error:", err)
		}

		return errors.New("build failed")

	}

	return nil
}

func fixReadmeContent(s string) string {
	// Tell Hugo not to process shortcode samples
	s = regexp.MustCompile(`(?s){{%(/)?([^\/].*?)%}}`).ReplaceAllString(s, `{{%/*$1$2*/%}}`)
	s = regexp.MustCompile(`(?s){{<(/)?([^\/].*?)>}}`).ReplaceAllString(s, `{{</*$1$2*/>}}`)
	// s = regexp.MustCompile(`(?s)github\.com\/(.*?)\/blob\/master\/images/raw\.githubusercontent\.com`).ReplaceAllString(s, `/$1/master/images/`)

	return s
}

func (c *buildClient) writeThemeContent(k string, m client.Module) error {
	re := regexp.MustCompile(`\/v\d+$`)
	themeName := strings.ToLower(path.Base(re.ReplaceAllString(k, "")))

	themeDir := filepath.Join(c.contentDir, "themes", themeName)
	client.CheckErr(os.MkdirAll(themeDir, 0o777))

	copyIfExists := func(sourcePath, targetPath string) error {
		fs, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		defer fs.Close()
		client.CheckErr(os.MkdirAll(filepath.Dir(targetPath), 0o777))
		ft, err := os.Create(targetPath)
		client.CheckErr(err)
		defer ft.Close()

		_, err = io.Copy(ft, fs)
		client.CheckErr(err)

		return nil
	}

	copyAsset := func(source, target string) error {
		var foundOne bool
		allowedExts := []string{".png", ".jpg"}
		for _, ext := range allowedExts {
			sourceFilename := filepath.Join(m.Dir, source) + ext
			targetFilename := filepath.Join(themeDir, target) + ext
			if err := copyIfExists(sourceFilename, targetFilename); err == nil {
				foundOne = true
				break
			}
		}
		if !foundOne {
			exts := strings.Join(allowedExts, ", ")
			return fmt.Errorf("copy asset: no %q of type [%s] found for theme %q", source, exts, k)
		}

		return nil
	}

	copyPlaceholder := func(source, target string) error {
		ext := ".png"
		sourceFilename := filepath.Join(c.staticDir, "placeholders", filepath.Base(source)) + ext
		targetFilename := filepath.Join(themeDir, target) + ext
		err := copyIfExists(sourceFilename, targetFilename)
		if err != nil {
			return err
		}

		return nil
	}

	getReadMeContent := func() string {
		/*files, err := os.ReadDir(m.Dir)
		client.CheckErr(err)
		for _, fi := range files {
			if fi.IsDir() {
				continue
			}
			if strings.EqualFold(fi.Name(), "readme.md") {
				b, err := os.ReadFile(filepath.Join(m.Dir, fi.Name()))
				client.CheckErr(err)
				return fixReadmeContent(string(b))
			}
		}*/
		return ""
	}

	thm := &theme{
		m:             m,
		name:          themeName,
		readMeContent: getReadMeContent(),
		ghRepo:        c.getGitHubRepo(m.Path),
	}

	thm.calculateWeight(c.maxStars)

	// TODO(bep) we don't build any demo site anymore, but
	// we could and should probably build a simple site and
	// count warnings and error and use that to
	// either pull it down the list with weight or skip it.

	// Add warnings for old themes, bad URLs etc.

	if warn, found := thm.checkLastMod(); found {
		if warn.level == errorLevelBlock {
			thm.draft = true
		}
		thm.warn(warn.message)
	}

	for _, metaSiteKey := range []string{"demosite", "homepage"} {
		// TODO(bep) author sites + redirects?
		if s, found := m.Meta[metaSiteKey]; found {
			if c.IsBadURL(s.(string)) {
				thm.warn(themeWarningBadURL.message)

				// Remove it from the map.
				delete(m.Meta, metaSiteKey)
			}
		}
	}

	sort.Strings(thm.themeWarnings)

	frontmatter := thm.toFrontMatter()

	b, err := yaml.Marshal(frontmatter)
	client.CheckErr(err)

	content := fmt.Sprintf(`---
%s
---
%s
`, string(b), thm.readMeContent)

	if err := os.WriteFile(filepath.Join(themeDir, "index.md"), []byte(content), 0o666); err != nil {
		return err
	}

	assets := []asset{
		{"images/tn", "tn-featured"},
		{"images/screenshot", "screenshot"},
	}
	for _, a := range assets {
		if err := copyAsset(a.source, a.target); err != nil {
			c.Logf("warn: using asset placeholder: %s", err)

			if err = copyPlaceholder(a.source, a.target); err != nil {
				return err
			}
		}
	}

	return nil
}

type theme struct {
	m    client.Module
	name string

	// Set when hosted on GitHub.
	ghRepo client.GitHubRepo

	readMeContent string

	themeWarnings []string

	// Calculated
	weight int
	draft  bool
}

func (t *theme) isVersioned() bool {
	return semver.IsValid(t.m.Version)
}

func (t *theme) warn(s string) {
	t.themeWarnings = append(t.themeWarnings, s)
}

var (
	// 30 days.
	d30 = 30 * 24 * time.Hour
	// 7 days.
	d7 = 7 * 24 * time.Hour
	// 1 year.
	d1y = 365 * 24 * time.Hour
)

func (t *theme) calculateWeight(maxStars int) {
	const maxMaxStars = 3000
	t.weight = min(maxStars, maxMaxStars) + 500
	t.weight -= min(t.ghRepo.Stars, maxMaxStars)

	boostRecent := func(age, threshold time.Duration, boost int) {
		if age < threshold {
			t.weight -= boost
		}
	}

	// Boost themes versioned recently.
	if !t.m.Time.IsZero() && t.isVersioned() {
		// Add some weight to recently versioned themes.
		boostRecent(time.Since(t.m.Time), (1 * d30), 40)
	}

	if !t.ghRepo.IsZero() {
		// Boost themes created recently.
		boostRecent(time.Since(t.ghRepo.CreatedAt), (1 * d30), 100)
		boostRecent(time.Since(t.ghRepo.CreatedAt), (1 * d1y), 50)
		// Pull themes created within the last week the top.
		// Note that we currently only have that information for themes
		// hosted on GitHub.
		boostRecent(time.Since(t.ghRepo.CreatedAt), (1 * d7), 50000)
	}

	// Boost themes with a Hugo version indicator set that covers.
	// the current Hugo version.
	// TODO(bep) I removed Hugo as a dependency,
	// compared this against HUGO_VERSION somehow.
	/*if m.HugoVersion.IsValid() {
		weight -= boost
	}*/

	if t.weight < 0 {
		t.weight = 1
	}
}

func (t *theme) toFrontMatter() map[string]interface{} {
	var title string
	if mn, ok := t.m.Meta["name"]; ok {
		title = mn.(string)
	} else {
		title = strings.Title(t.name)
	}
	var description string
	if md, ok := t.m.Meta["description"]; ok {
		description = md.(string)
	} else {
		description = fmt.Sprintf("Theme %q for Hugo", title)
	}

	var htmlURL string
	var starCount int
	if !t.ghRepo.IsZero() {
		htmlURL = t.ghRepo.HTMLURL
		starCount = t.ghRepo.Stars
	} else {
		// Gitlab etc., assume the path is the base of the URL.
		htmlURL = fmt.Sprintf("https://%s", t.m.PathRepo())
	}

	var expiryDate time.Time
	if !t.m.Time.IsZero() {
		// 18 months since last version.
		expiryDate = t.m.Time.Add(18 * d30)
	} else {
		// In practice, this will never expire.
		expiryDate = time.Now().Add(22 * d30)
	}

	tags := normalizeTags(t.m.Meta["tags"])
	if starCount >= 750 {
		tags = append(tags, "popular")
	}

	return map[string]interface{}{
		"draft":         t.draft,
		"expiryDate":    expiryDate,
		"title":         title,
		"description":   description,
		"summary":       description,
		"slug":          t.name,
		"aliases":       []string{"/" + t.name},
		"weight":        t.weight,
		"lastMod":       t.m.Time,
		"hugoVersion":   t.m.HugoVersion,
		"modulePath":    t.m.Path,
		"htmlURL":       htmlURL,
		"meta":          t.m.Meta,
		"githubInfo":    t.ghRepo,
		"themeWarnings": t.themeWarnings,
		"tags":          tags,
	}
}

func (t *theme) checkLastMod() (warn warning, found bool) {
	lastMod := t.m.Time
	if !lastMod.IsZero() {
		age := time.Since(lastMod)
		ageYears := age.Hours() / 24 / 365
		if ageYears > 3 {
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

var goodTags = map[string]any{
	"agency":       true,
	"api":          true,
	"archive":      true,
	"archives":     "archive",
	"blog":         true,
	"book":         true,
	"bootstrap":    true,
	"business":     "company",
	"church":       true,
	"company":      true,
	"dark mode":    true,
	"dark":         true,
	"ecommerce":    true,
	"education":    true,
	"gallery":      true,
	"green":        true,
	"hero":         true,
	"light mode":   true,
	"light":        true,
	"magazine":     true,
	"multilingual": true,
	"newsletter":   true,
	"personal":     true,
	"pink":         true,
	"podcast":      true,
	"portfolio":    true,
	"sass":         true,
	"tailwind":     true,
	"tailwindcss":  "tailwind",
	"two-column":   true,
	"white":        "light",
}

func normalizeTag(s string) string {
	s = strings.ToLower(s)

	if v, found := goodTags[s]; found {
		switch vv := v.(type) {
		case string:
			return vv
		default:
			return s
		}
	}

	ca := func(candidates ...string) bool {
		for _, candidate := range candidates {
			if strings.Contains(s, candidate) {
				return true
			}
		}
		return false
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

	if ca("docs", "document") {
		return "docs"
	}

	if ca("landing") {
		return "landing"
	}

	if ca("one") {
		return "landing"
	}

	if ca("minimal") {
		return "minimal"
	}

	if ca("product") {
		return "ecommerce"
	}

	return ""
}

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
