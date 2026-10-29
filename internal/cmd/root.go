/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

// Package cmd wires up the raynet CLI's cobra command tree.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/raynetcrm/raynet-cli/internal/client"
	"github.com/raynetcrm/raynet-cli/internal/config"
	"github.com/raynetcrm/raynet-cli/internal/version"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// app is one invocation of the CLI: its I/O, its environment seams and the
// global flag values. The command tree is built fresh from an app on every
// execution (newRootCmd), so no command or flag state lives in package-level
// globals. Tests construct an app with their own writers and seams (see
// harness_test.go).
type app struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	// isTerminal reports whether stdout is an interactive terminal; it
	// switches styled tables, jq highlighting and other terminal-only output.
	isTerminal func() bool
	// isInteractive reports whether stdin is an interactive terminal, so a
	// command may prompt for a missing value; otherwise it must fail instead.
	isInteractive func() bool
	// stdinTerminal reports whether stdin is a real terminal device, which
	// huh needs to show its screens and to hide an API key or bearer token
	// while typed.
	stdinTerminal func() bool
	// accessible reports whether the user asked for accessible prompts
	// ($RAYNET_CLI_ACCESSIBLE): plain line prompts for screen readers.
	accessible func() bool
	// openBrowser opens a URL in the user's web browser.
	openBrowser func(url string) error

	flags globalFlags
}

// globalFlags are the root command's persistent flags.
type globalFlags struct {
	// profile selects a named profile from the config file (see
	// internal/config.Profile). Falls back to $RAYNET_CLI_PROFILE, then the
	// config's default, stripe-cli style — a profile is independent of host,
	// so two profiles can target the same host with different credentials.
	profile string
	// host is only used by the ad-hoc credential path (--api-key/--token);
	// selecting a *stored* profile is done via --profile.
	host    string
	json    bool
	jq      string
	verbose bool

	// Ad-hoc credential overrides (stripe-cli style): when --api-key or
	// --token is set, they take precedence over any stored `auth login`
	// config for this invocation, letting a single command run without
	// logging in first.
	apiKey       string
	username     string
	token        string
	instanceName string
	instanceID   string
}

// newApp returns an app wired to the real process: os streams, the real
// terminal checks, $RAYNET_CLI_ACCESSIBLE and the OS browser.
func newApp() *app {
	stdinTerminal := func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	return &app{
		stdin:         os.Stdin,
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		isTerminal:    func() bool { return term.IsTerminal(int(os.Stdout.Fd())) },
		isInteractive: stdinTerminal,
		stdinTerminal: stdinTerminal,
		accessible:    func() bool { return os.Getenv("RAYNET_CLI_ACCESSIBLE") != "" },
		openBrowser:   openInBrowser,
	}
}

// Execute runs the CLI with the process arguments; called from
// cmd/raynet/main.go. Errors are already printed on stderr.
func Execute() error {
	return newApp().execute(os.Args[1:])
}

// execute builds a fresh command tree and runs it with args. A failure is
// rendered on stderr (see renderError) and returned, so the caller only has
// to exit non-zero.
func (a *app) execute(args []string) error {
	root := newRootCmd(a)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		a.renderError(err)
	}
	return err
}

// newRootCmd builds the whole command tree for one execution.
func newRootCmd(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:           "raynet",
		Short:         "Command-line client for the RAYNET CRM REST API",
		Long:          rootLong(),
		Example:       "  raynet auth login\n  raynet company list --fulltext acme\n  raynet company create --help\n  raynet help filtering",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(a.stdin)
	root.SetOut(a.stdout)
	root.SetErr(a.stderr)

	f := &a.flags
	pf := root.PersistentFlags()
	pf.StringVar(&f.profile, "profile", "", "named profile to use (defaults to $RAYNET_CLI_PROFILE, then the configured default)")
	pf.StringVar(&f.host, "host", "", "RAYNET CRM host to use with --api-key/--token (ad-hoc credentials, bypassing stored profiles)")
	pf.BoolVar(&f.json, "json", false, "print raw JSON instead of a table")
	pf.StringVar(&f.jq, "jq", "", "filter JSON output through a jq expression (requires jq on PATH)")
	pf.BoolVarP(&f.verbose, "verbose", "v", false, "print each request line and status to stderr")

	pf.StringVar(&f.apiKey, "api-key", "", "ad-hoc API key, bypassing stored \"auth login\" credentials for this call (apikey auth)")
	pf.StringVar(&f.username, "username", "", "ad-hoc username, used with --api-key")
	pf.StringVar(&f.token, "token", "", "ad-hoc bearer token, bypassing stored \"auth login\" credentials for this call")
	pf.StringVar(&f.instanceName, "instance-name", "", "ad-hoc instance name, used with --api-key")
	pf.StringVar(&f.instanceID, "instance-id", "", "ad-hoc instance ID, alternative to --instance-name")

	root.AddCommand(
		newAuthCmd(a),
		newAPICmd(a),
		newCodelistCmd(a),
	)
	root.AddCommand(newEntityCmds(a)...)
	root.AddCommand(helpTopics()...)
	for _, c := range root.Commands() {
		rejectUnknownSubcommands(c)
	}
	return root
}

// rejectUnknownSubcommands makes every command under c that has subcommands
// (an entity, a nested collection, a codelist, auth) fail on an argument that
// names none of them. Cobra does this for the root command only; elsewhere it
// would print help and exit 0. A group command with no arguments still shows
// its help, unless it runs something itself (codelist, codelist <name>).
func rejectUnknownSubcommands(c *cobra.Command) {
	if !c.HasSubCommands() {
		return
	}
	c.Args = noSubcommand
	c.SuggestionsMinimumDistance = 2 // cobra's default, applied only to the root
	if !c.Runnable() {
		c.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
	}
	// Flags are parsed before Args is checked, so a flag of the unknown
	// subcommand ("codelist x create --name A") would otherwise be reported
	// as an unknown flag instead.
	group := c
	c.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		if c == group {
			if argErr := noSubcommand(c, c.Flags().Args()); argErr != nil {
				return argErr
			}
		}
		return err
	})
	for _, sub := range c.Commands() {
		rejectUnknownSubcommands(sub)
	}
}

// noSubcommand rejects positional arguments to a group command: the first one
// is an unknown subcommand.
func noSubcommand(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	msg := fmt.Sprintf("unknown command %q for %q", args[0], c.CommandPath())
	if s := c.SuggestionsFor(args[0]); len(s) > 0 {
		msg += "\n\nDid you mean this?\n\t" + strings.Join(s, "\n\t")
	}
	return errors.New(msg)
}

// resolveClient builds a client.Client for the current invocation. If
// --api-key or --token was passed, those ad-hoc credentials are used
// directly (stripe-cli style: flags always win, no `auth login` required).
// Otherwise it resolves a named profile — --profile, else $RAYNET_CLI_PROFILE,
// else the config's default — from stored config. Shared by every subcommand
// that talks to the API.
func (a *app) resolveClient() (*client.Client, error) {
	p, err := a.resolveProfile()
	if err != nil {
		return nil, err
	}
	c := client.New(p)
	c.Verbose = a.flags.verbose
	c.Log = a.stderr
	return c, nil
}

// resolveProfile returns the profile this invocation runs with: one built
// from the ad-hoc credential flags, else the selected stored profile (see
// resolveClient).
func (a *app) resolveProfile() (config.Profile, error) {
	var p config.Profile
	f := a.flags

	if f.apiKey != "" || f.token != "" {
		host, scheme, err := parseHost(f.host)
		if errors.Is(err, errNoHost) {
			return config.Profile{}, fmt.Errorf("--host is required when using --api-key/--token")
		}
		if err != nil {
			return config.Profile{}, err
		}
		p = config.Profile{Host: host, Scheme: scheme}
		switch {
		case f.token != "":
			p.AuthType = config.AuthBearer
			p.Token = f.token
		default:
			p.AuthType = config.AuthAPIKey
			p.Username = f.username
			p.APIKey = f.apiKey
			p.InstanceName = f.instanceName
			p.InstanceID = f.instanceID
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return config.Profile{}, err
		}
		profile := f.profile
		if profile == "" {
			profile = os.Getenv("RAYNET_CLI_PROFILE")
		}
		_, p, err = cfg.ActiveProfile(profile)
		if err != nil {
			return config.Profile{}, err
		}
	}
	return p, nil
}

// openInBrowser opens url with $BROWSER, else the OS default handler.
func openInBrowser(url string) error {
	var c *exec.Cmd
	switch {
	case os.Getenv("BROWSER") != "":
		c = exec.Command(os.Getenv("BROWSER"), url)
	case runtime.GOOS == "darwin":
		c = exec.Command("open", url)
	case runtime.GOOS == "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}
