package rootcmd

import (
	"context"
	"flag"

	"github.com/gohugoio/hugoThemesSiteBuilder/pkg/client"
	"github.com/peterbourgon/ff/v3/ffcli"
)

// CommandName is the main command's binary name.
const CommandName = "hugothemesitebuilder"

type Config struct {
	Out   string
	Quiet bool

	Client *client.Client
}

// New constructs a usable ffcli.Command and an empty Config. The config
// will be set after a successful parse. The caller must
// initialize the config's client field.
func New() (*ffcli.Command, *Config) {
	var cfg Config

	fs := flag.NewFlagSet(CommandName, flag.ExitOnError)

	cfg.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       CommandName,
		ShortUsage: CommandName + " [flags] <subcommand> [flags] [<arg>...]",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}, &cfg
}

// RegisterFlags registers the flag fields into the provided flag.FlagSet. This
// helper function allows subcommands to register the root flags into their
// flagsets, creating "global" flags that can be passed after any subcommand at
// the commandline.
func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Out, "out", "build", "the output folder to write files to (will be created if it does not exist)")
	fs.BoolVar(&c.Quiet, "quiet", false, "only log errors")
}

// Exec function for this command.
func (c *Config) Exec(context.Context, []string) error {
	// The root command has no meaning, so if it gets executed,
	// display the usage text to the user instead.
	return flag.ErrHelp
}
