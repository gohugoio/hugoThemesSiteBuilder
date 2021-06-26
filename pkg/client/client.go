package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/gohugoio/hugo/modules"
)

const (
	modPath  = "github.com/gohugoio/hugoThemesSiteBuilder/cmd/hugothemesitebuilder/build"
	cacheDir = "cache"
)

func New(logWriter io.Writer, outDir string) (*Client, error) {
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	return &Client{logWriter: logWriter, outDir: outDir}, nil
}

type Client struct {
	logWriter io.Writer
	outDir    string
}

func (c *Client) GetHugoModulesMap(config string) (ModulesMap, error) {
	b := &bytes.Buffer{}
	if err := c.runHugo(b, "--config", config, "config", "mounts", "-v"); err != nil {
		return nil, err
	}

	mmap := make(ModulesMap)
	dec := json.NewDecoder(b)

	for dec.More() {
		var m Module
		if derr := dec.Decode(&m); derr != nil {
			return nil, derr
		}

		if m.Owner == modPath {
			mmap[m.Path] = m
		}
	}

	return mmap, nil
}

// Logf logs to the configured log writer.
func (c *Client) Logf(format string, a ...interface{}) {
	fmt.Fprintf(c.logWriter, format+"\n", a...)
}

func (c *Client) InitModule(config string) error {
	return c.RunHugo("mod", "init", modPath, "--config", config)
}

func (c *Client) OutFileExists(name string) bool {
	filename := filepath.Join(c.outDir, name)
	_, err := os.Stat(filename)
	return err == nil
}

func (c *Client) RunHugo(arg ...string) error {
	return c.runHugo(io.Discard, arg...)
}

func (c *Client) CreateThemesConfig() error {
	f, err := os.Open(filepath.Join(c.outDir, "themes.txt"))
	if err != nil {
		return err
	}
	defer f.Close()

	config := make(map[string]interface{})
	var imports []map[string]interface{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") {
			imports = append(imports, map[string]interface{}{
				"path":          line,
				"ignoreImports": true,
				"noMounts":      true,
			})

		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	config["module"] = map[string]interface{}{
		"imports": imports,
	}

	b, err := json.Marshal(config)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(c.outDir, "config.json"), b, 0666)

}

func (c *Client) TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Fprintf(c.logWriter, "%s in %v ms\n", name, int(1000*elapsed.Seconds()))
}

func (c *Client) WriteThemesContent(mm ModulesMap) error {
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

	contentDir := filepath.Join(c.outDir, "site", "content")
	checkErr(os.RemoveAll(contentDir))
	checkErr(os.MkdirAll(contentDir, 0777))

	for k, m := range mm {

		themeName := strings.ToLower(path.Base(k))

		themeDir := filepath.Join(contentDir, "themes", themeName)
		checkErr(os.MkdirAll(themeDir, 0777))

		copyIfExists := func(sourcePath, targetPath string) {
			fs, err := os.Open(filepath.Join(m.Dir, sourcePath))
			if err != nil {
				return
			}
			defer fs.Close()
			targetFilename := filepath.Join(themeDir, targetPath)
			checkErr(os.MkdirAll(filepath.Dir(targetFilename), 0777))
			ft, err := os.Create(targetFilename)
			checkErr(err)
			defer ft.Close()

			_, err = io.Copy(ft, fs)
			checkErr(err)
		}

		fixReadMeContent := func(s string) string {
			// Tell Hugo not to process shortcode samples
			s = regexp.MustCompile(`(?s){\{%([^\/].*?)%\}\}`).ReplaceAllString(s, `{{%/*$1*/%}}`)
			s = regexp.MustCompile(`(?s){\{<([^\/].*?)>\}\}`).ReplaceAllString(s, `{{</*$1*/>}}`)

			return s
		}

		getReadMeContent := func() string {
			files, err := os.ReadDir(m.Dir)
			checkErr(err)
			for _, fi := range files {
				if fi.IsDir() {
					continue
				}
				if strings.EqualFold(fi.Name(), "readme.md") {
					b, err := ioutil.ReadFile(filepath.Join(m.Dir, fi.Name()))
					checkErr(err)
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

		c.Logf("Processing theme %q with weight %d", themeName, weight)

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
		checkErr(err)

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

		return nil

	}

	return nil
}

func (c *Client) GetGitHubRepos(mods ModulesMap) (map[string]GitHubRepo, error) {
	const cacheFile = "githubrepos.json"
	cacheFilename := filepath.Join(c.outDir, cacheDir, cacheFile)
	b, err := ioutil.ReadFile(cacheFilename)
	if err == nil {
		m := make(map[string]GitHubRepo)
		err := json.Unmarshal(b, &m)
		return m, err
	}

	m, err := c.fetchGitHubRepos(mods)
	if err != nil {
		return nil, err
	}

	b, err = json.Marshal(m)
	if err != nil {
		return nil, err
	}

	checkErr(os.MkdirAll(filepath.Dir(cacheFilename), 0777))

	return m, ioutil.WriteFile(cacheFilename, b, 0666)

}

func (c *Client) fetchGitHubRepo(m Module) (GitHubRepo, error) {
	var repo GitHubRepo

	const githubdotcom = "github.com"

	if !strings.HasPrefix(m.Path, githubdotcom) {
		return repo, nil
	}
	repoPath := strings.TrimPrefix(m.Path, githubdotcom)
	apiURL := "https://api.github.com/repos" + repoPath

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return repo, err
	}

	err = doGitHubRequest(req, &repo)
	if err != nil {
		return repo, fmt.Errorf("failed to get GitHub repo for %q: %s", apiURL, err)
	}
	return repo, nil
}

func (c *Client) fetchGitHubRepos(mods ModulesMap) (map[string]GitHubRepo, error) {
	repos := make(map[string]GitHubRepo)

	for _, m := range mods {
		repo, err := c.fetchGitHubRepo(m)
		if err != nil {
			return nil, err
		}
		repos[m.Path] = repo
	}

	return repos, nil
}

func (c *Client) runHugo(w io.Writer, arg ...string) error {
	env := os.Environ()
	setEnvVars(&env, "PWD", c.outDir) // Use the output dir as the Hugo root.

	cmd := exec.Command("hugo", arg...)
	cmd.Env = env
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

type GitHubRepo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Stars       int    `json:"stargazers_count"`
}

type Module struct {
	Path        string                 `json:"path"`
	Owner       string                 `json:"owner"`
	Version     string                 `json:"version"`
	Time        time.Time              `json:"time"`
	Dir         string                 `json:"dir"`
	HugoVersion modules.HugoVersion    `json:"hugoVersion"`
	Meta        map[string]interface{} `json:"meta"`
}

type ModulesMap map[string]Module

func setEnvVar(vars *[]string, key, value string) {
	for i := range *vars {
		if strings.HasPrefix((*vars)[i], key+"=") {
			(*vars)[i] = key + "=" + value
			return
		}
	}
	// New var.
	*vars = append(*vars, key+"="+value)
}

func setEnvVars(oldVars *[]string, keyValues ...string) {
	for i := 0; i < len(keyValues); i += 2 {
		setEnvVar(oldVars, keyValues[i], keyValues[i+1])
	}
}

func isError(resp *http.Response) bool {
	return resp.StatusCode < 200 || resp.StatusCode > 299
}

func addGitHubToken(req *http.Request) {
	gitHubToken := os.Getenv("GITHUB_TOKEN")
	if gitHubToken != "" {
		req.Header.Add("Authorization", "token "+gitHubToken)
	}
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func doGitHubRequest(req *http.Request, v interface{}) error {
	addGitHubToken(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if isError(resp) {
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("GitHub lookup failed: %s", string(b))
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
