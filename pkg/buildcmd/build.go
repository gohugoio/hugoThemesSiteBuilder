package buildcmd

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

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
	client := c.rootConfig.Client

	if err := client.CreateThemesConfig(); err != nil {
		return err
	}

	if true {
		return nil
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

	if err := client.WriteThemesContent(mmap); err != nil {
		return err
	}

	return nil

}
