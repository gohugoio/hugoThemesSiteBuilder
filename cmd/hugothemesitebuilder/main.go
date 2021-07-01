package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/buildcmd"
	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/rootcmd"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

var version = "v0.5"

func main() {
	var (
		rootCommand, rootConfig = rootcmd.New()
		buildCommand            = buildcmd.New(rootConfig)

		versionCommand = &ffcli.Command{
			Name:       "version",
			ShortUsage: rootcmd.CommandName + " version",
			ShortHelp:  "Print this program's version",
			Exec: func(context.Context, []string) error {
				fmt.Println(version)
				return nil
			},
		}
	)

	rootCommand.Subcommands = []*ffcli.Command{
		buildCommand,
		versionCommand,
	}

	rootCommand.Options = []ff.Option{
		// ff.WithEnvVarPrefix(""),
	}

	if err := rootCommand.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error during Parse: %v\n", err)
		os.Exit(1)
	}

	logWriter := io.Discard
	if !rootConfig.Quiet {
		logWriter = os.Stdout
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get current dir: %v\n", err)
		os.Exit(1)
	}

	if !filepath.IsAbs(rootConfig.Out) {
		rootConfig.Out = filepath.Join(wd, rootConfig.Out)
	}

	os.Chdir(rootConfig.Out)
	defer func() {
		os.Chdir(wd)
	}()

	client, err := client.New(logWriter, rootConfig.Out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: Failed to create client: %v\n", err)
		os.Exit(1)
	}

	client.Logf("Writing output files to %q", rootConfig.Out)

	if os.Getenv("NETLIFY") == "true" && os.Getenv("PULL_REQUEST") != "" && os.Getenv("DEPLOY_PRIME_URL") != "" {
		client.Logf("Running on Netlify (and not Cloudflare :-))")
	}

	rootConfig.Client = client

	if err := os.MkdirAll(rootConfig.Out, 0777); err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "error: Failed to create output folder %q: %v\n", rootConfig.Out, err)
		os.Exit(1)
	}

	defer client.TimeTrack(time.Now(), "Total")

	if err := rootCommand.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
