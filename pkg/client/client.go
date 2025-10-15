package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"
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

	badHostsInit sync.Once
	badHosts     map[string]bool
}

func (c *Client) IsBadURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return true
	}

	c.badHostsInit.Do(func() {
		c.badHosts = make(map[string]bool)
		f, err := os.Open(filepath.Join(c.outDir, "badhosts.txt"))
		if err != nil {
			panic(fmt.Sprintf("failed to open badhosts.txt: %s", err))
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		counter := 0
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) != 2 {
				continue
			}
			c.badHosts[strings.ToLower(parts[1])] = true
			counter++

		}

		c.Logf("Loaded %d bad hosts", counter)
	})

	host := strings.ToLower(u.Host)

	return c.badHosts[host] || c.badHosts[strings.TrimPrefix(host, "www.")]
}

func (c *Client) GetHugoModulesMap(config string) (ModulesMap, error) {
	defer c.TimeTrack(time.Now(), "Got Hugo Modules")
	b := &bytes.Buffer{}
	if err := c.runHugo(b, "--config", config, "config", "mounts"); err != nil {
		return nil, err
	}

	mmap := make(ModulesMap)
	dec := json.NewDecoder(b)

	c.Logf("Get Hugo Modules, config %q", config)

	for dec.More() {
		var m Module
		if derr := dec.Decode(&m); derr != nil {
			b, _ := io.ReadAll(dec.Buffered())
			return nil, fmt.Errorf("failed to decode module config: %s\n%s", derr, b)
		}

		if m.Owner == modPath {
			mmap[m.Path] = m
		}
	}

	for k, v := range mmap {

		// Read any theme.toml into .Meta
		filename := filepath.Join(v.Dir, "theme.toml")
		if _, err := os.Stat(filename); err == nil {
			b, err := os.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("failed to read %q: %s", filename, err)
			}
			if err := toml.Unmarshal(b, &v.Meta); err != nil {
				c.Logf("warn: failed to parse theme.toml for theme %q: %s", k, err)
			}
			mmap[k] = v
		}
	}

	return mmap, nil
}

// Logf logs to the configured log writer.
func (c *Client) Logf(format string, a ...interface{}) {
	fmt.Fprintf(c.logWriter, format+"\n", a...)
}

func (c *Client) InitModule() error {
	return c.RunHugo("mod", "init", modPath)
}

func (c *Client) OutFileExists(name string) bool {
	filename := filepath.Join(c.outDir, name)
	_, err := os.Stat(filename)
	return err == nil
}

func (c *Client) RemoveGoModAndGoSum() {
	goModFilename := filepath.Join(c.outDir, "go.mod")
	goSumFilename := filepath.Join(c.outDir, "go.sum")
	os.Remove(goModFilename)
	os.Remove(goSumFilename)
}

func (c *Client) RunHugo(arg ...string) error {
	return c.runHugo(nil, arg...)
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
				"ignoreConfig":  true,
				"noMounts":      true,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	config["module"] = map[string]interface{}{
		"hugoVersion": map[string]interface{}{
			"min": "0.115.0",
		},
		"imports": imports,
	}

	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(c.outDir, "config.json"), b, 0o666)
}

func (c *Client) JoinOutPath(elem ...string) string {
	return filepath.Join(append([]string{c.outDir}, elem...)...)
}

func (c *Client) TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Fprintf(c.logWriter, "%s in %v ms\n", name, int(1000*elapsed.Seconds()))
}

const gitHubReposCacheFilename = "githubrepos.json"

// GetGitHubRepos will first look in the chache folder for GitHub repo
// information for mods. If not found, it will ask GitHub and then store
// it in the cache.
//
// If you start with an empty cache, you will need to set a GITHUB_TOKEN environment variable.
func (c *Client) GetGitHubRepos(mods ModulesMap, cleanCache bool) (map[string]GitHubRepo, error) {
	c.Logf("Get GitHub repos")
	defer c.TimeTrack(time.Now(), "Got GitHub repos")
	cacheFilename := filepath.Join(c.outDir, cacheDir, gitHubReposCacheFilename)
	if cleanCache {
		os.Remove(cacheFilename)
	}
	b, err := os.ReadFile(cacheFilename)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		// Fetch the github repos and store in cache.
		m, err := c.fetchGitHubRepos(mods)
		if err != nil {
			return nil, err
		}

		if len(m) > 0 {
			b, err := json.MarshalIndent(m, "", "  ")
			if err != nil {
				return nil, err
			}
			CheckErr(os.MkdirAll(filepath.Dir(cacheFilename), 0o777))
			CheckErr(os.WriteFile(cacheFilename, b, 0o666))
		}

		return m, nil
	}

	m := make(map[string]GitHubRepo)

	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	return m, nil
}

func (c *Client) fetchGitHubRepo(m Module) (GitHubRepo, error) {
	var repo GitHubRepo

	const githubdotcom = "github.com"

	if !strings.HasPrefix(m.Path, githubdotcom) {
		return repo, nil
	}
	repoPath := strings.TrimPrefix(m.PathRepo(), githubdotcom)
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
	errCount := 0
	for _, m := range mods {
		repo, err := c.fetchGitHubRepo(m)
		if err != nil {
			if errCount > 5 {
				return repos, err
			}
			errCount++
			c.Logf("warning: %s", err)
			continue
		}
		repos[m.Path] = repo
	}

	return repos, nil
}

func (c *Client) runHugo(w io.Writer, arg ...string) error {
	env := os.Environ()

	arg = append(arg, "--quiet")

	if w == nil {
		w = os.Stdout
	}

	var errBuf bytes.Buffer
	stderr := io.MultiWriter(os.Stderr, &errBuf)

	cmd := exec.Command("hugo", arg...)
	cmd.Dir = c.outDir
	cmd.Env = env
	cmd.Stdout = w
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("hugo command failed: %s\n%s", err, errBuf.String())
	}
	return nil
}

type GitHubRepo struct {
	ID          int       `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	HTMLURL     string    `json:"html_url"`
	Stars       int       `json:"stargazers_count"`
}

func (g GitHubRepo) IsZero() bool {
	return g.HTMLURL == ""
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

func (m Module) PathWithoutVersion() string {
	return PathWithoutVersion(m.Path)
}

// PathRepo returns the root path to the repository.
func (m Module) PathRepo() string {
	slashCount := 0
	p := m.PathWithoutVersion()
	idx := strings.IndexFunc(p, func(r rune) bool {
		if r == '/' {
			slashCount++
		}
		return slashCount > 2
	})

	if slashCount < 3 {
		return p
	}

	return p[:idx]
}

// HugoVersion holds Hugo binary version requirements for a module.
type HugoVersion struct {
	Min      string
	Max      string
	Extended bool
}

type ModulesMap map[string]Module

var pathVersionRe = regexp.MustCompile(`/v\d+$`)

func PathWithoutVersion(s string) string {
	return pathVersionRe.ReplaceAllString(s, "")
}

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
