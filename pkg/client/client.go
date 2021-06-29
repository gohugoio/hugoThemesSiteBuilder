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
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
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
	c.Logf("Get Hugo Modules, config %q", config)
	defer c.TimeTrack(time.Now(), "Got Hugo Modules")
	b := &bytes.Buffer{}
	if err := c.runHugo(b, "--config", config, "config", "mounts", "-v"); err != nil {
		return nil, err
	}

	mmap := make(ModulesMap)
	dec := json.NewDecoder(b)

	for dec.More() {
		var m Module
		if derr := dec.Decode(&m); derr != nil {
			b, _ := io.ReadAll(dec.Buffered())
			return nil, fmt.Errorf("failed to decode config: %s (buf: %q)", derr, b)
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

// CreateThemesConfig reads themes.txt and creates a config.json
// suitable for Hugo. Note that we're only using that config to
// get the full module listing.
func (c *Client) CreateThemesConfig() error {
	// This looks a little funky, but we want the themes.txt to be
	// easily visible for users to add to in the root of the project.
	f, err := os.Open(filepath.Join(c.outDir, "../../..", "themes.txt"))
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
		"hugoVersion": map[string]interface{}{
			"min": "0.84.2", // The noMounts config option was added in this version.
		},
		"imports": imports,
	}

	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(c.outDir, "config.json"), b, 0666)

}

func (c *Client) JoinOutPath(elem ...string) string {
	return filepath.Join(append([]string{c.outDir}, elem...)...)
}

func (c *Client) TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Fprintf(c.logWriter, "%s in %v ms\n", name, int(1000*elapsed.Seconds()))
}

const cacheFileSuffix = "githubrepos.json"

// GetGitHubRepos will first look in the chache folder for GitHub repo
// information for mods. If not found, it will ask GitHub and then store
// it in the cache.
//
// If you start with an empty cache, you will need to set a GITHUB_TOKEN environment variable.
func (c *Client) GetGitHubRepos(mods ModulesMap) (map[string]GitHubRepo, error) {
	c.Logf("Get GitHub repos")
	defer c.TimeTrack(time.Now(), "Got GitHub repos")
	cacheFiles := c.getGithubReposCacheFilesSorted()
	m := make(map[string]GitHubRepo)
	for _, cacheFile := range cacheFiles {
		cacheFilename := filepath.Join(c.outDir, cacheDir, cacheFile)
		b, err := ioutil.ReadFile(cacheFilename)
		if err != nil {
			return nil, err
		}

		m2 := make(map[string]GitHubRepo)
		if err = json.Unmarshal(b, &m2); err != nil {
			return nil, err
		}

		for k, v := range m2 {
			m[k] = v

		}
	}

	missing := ModulesMap{}
	for k, v := range mods {
		if _, found := m[k]; !found {
			missing[k] = v
		}
	}

	if len(missing) > 0 {
		cacheNum := 0
		if len(cacheFiles) > 0 {
			last := cacheFiles[len(cacheFiles)-1]
			cacheNum, _ = strconv.Atoi(last[:strings.Index(last, ".")])
			cacheNum++
		}
		nextCacheFilename := filepath.Join(c.outDir, cacheDir, fmt.Sprintf("%0*d.%s", 4, cacheNum, cacheFileSuffix))
		m2, err := c.fetchGitHubRepos(mods)
		if err != nil {
			return nil, err
		}

		if len(m2) > 0 {
			b, err := json.Marshal(m2)
			if err != nil {
				return nil, err
			}

			for k, v := range m2 {
				m[k] = v
			}

			CheckErr(os.MkdirAll(filepath.Dir(nextCacheFilename), 0777))
			CheckErr(ioutil.WriteFile(nextCacheFilename, b, 0666))
		}
	}

	return m, nil

}

func (c *Client) getGithubReposCacheFilesSorted() []string {

	fis, err := os.ReadDir(filepath.Join(c.outDir, cacheDir))
	CheckErr(err)

	var entries []string

	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}

		if strings.HasSuffix(fi.Name(), cacheFileSuffix) {
			entries = append(entries, fi.Name())
		}
	}

	sort.Strings(entries)

	return entries

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
		return repo, fmt.Errorf("failed to get GitHub repo for %q: %s. Set GITHUB_TOKEN if you get rate limiting errors.", apiURL, err)
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

	arg = append(arg, "--quiet")

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
	HugoVersion HugoVersion            `json:"hugoVersion"`
	Meta        map[string]interface{} `json:"meta"`
}

// HugoVersion holds Hugo binary version requirements for a module.
type HugoVersion struct {
	Min      string
	Max      string
	Extended bool
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

func CheckErr(err error) {
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
