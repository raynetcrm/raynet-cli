/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/raynetcrm/raynet-cli/internal/client"
	"github.com/raynetcrm/raynet-cli/internal/config"
	"github.com/spf13/cobra"
)

// loginOptions are the flags of `raynet auth login`.
type loginOptions struct {
	profile      string
	host         string
	authType     string
	username     string
	apiKey       string
	instanceName string
	instanceID   string
	token        string
	webURL       string
	setDefault   bool
}

func newAuthCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:     "auth",
		Short:   "Manage RAYNET CRM authentication",
		Example: "  raynet auth login\n  raynet auth status",
	}

	var o loginOptions
	login := &cobra.Command{
		Use:   "login",
		Short: "Store credentials as a named profile",
		Long: fmt.Sprintf(
			"Store credentials as a named profile, once GET /security/info accepts them.\n\n"+
				"Known hosts: %s\n\n"+
				"Auth types:\n"+
				"  apikey  username + API key, plus --instance-name or --instance-id\n"+
				"  bearer  a bearer token, which already carries the instance, username and API key\n\n"+
				"The profile is named after the instance the credentials resolve to, unless --profile is\n"+
				"given; an existing profile with that name is replaced. The first profile becomes the\n"+
				"default; a later one only with --default.\n\n"+
				"In a terminal, a wizard asks every step no flag answered: host, login method and its\n"+
				"fields (the API key and bearer token hidden), then, after the check, save anyway?,\n"+
				"the profile name, replace? and make default?. Set RAYNET_CLI_ACCESSIBLE=1 for plain\n"+
				"line-by-line prompts. Otherwise a missing value is an error naming its flag, and nothing\n"+
				"is saved when the credentials fail the check.",
			strings.Join(config.KnownHosts, ", "),
		),
		Example: "  raynet auth login\n" +
			"  raynet auth login --host app.raynet.cz --auth-type apikey --username jan@acme.cz --api-key \"$RAYNET_API_KEY\" --instance-name acme\n" +
			"  raynet auth login --host app.raynet.cz --auth-type apikey --username jan@acme.cz --api-key \"$RAYNET_API_KEY\" --instance-id 1a2b3c --default\n" +
			"  raynet auth login --profile acme-bearer --host app.raynet.cz --auth-type bearer --token \"$RAYNET_TOKEN\"",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.runAuthLogin(&o)
		},
	}
	login.Flags().StringVar(&o.profile, "profile", "", "name for this profile (defaults to the instance name the credentials resolve to)")
	login.Flags().StringVar(&o.host, "host", "", "host, e.g. app.raynet.cz")
	login.Flags().StringVar(&o.authType, "auth-type", "", "apikey or bearer")
	login.Flags().StringVar(&o.username, "username", "", "username (apikey)")
	login.Flags().StringVar(&o.apiKey, "api-key", "", "API key (apikey)")
	login.Flags().StringVar(&o.instanceName, "instance-name", "", "instance name (apikey)")
	login.Flags().StringVar(&o.instanceID, "instance-id", "", "instance ID, alternative to --instance-name (apikey)")
	login.Flags().StringVar(&o.token, "token", "", "bearer token, stored as given (bearer)")
	login.Flags().StringVar(&o.webURL, "web-url", "", "CRM web app base URL for --web links, when it differs from the API's scheme + host")
	login.Flags().BoolVar(&o.setDefault, "default", false, "make this the default profile (the first profile always is)")

	status := &cobra.Command{
		Use:     "status",
		Short:   "Show configured profiles",
		Example: "  raynet auth status",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.runAuthStatus()
		},
	}

	cfgCmd := &cobra.Command{
		Use:     "config",
		Short:   "Show the config file path",
		Example: "  raynet auth config",
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(a.stdout, path)
			return err
		},
	}

	token := &cobra.Command{
		Use:   "token",
		Short: "Mint a bearer token for the active credentials",
		Long: "Mint a bearer token for the active credentials (GET /security/bearertoken) and print it.\n\n" +
			"The token carries the instance, username and API key, and stays valid as long as the API key does.\n" +
			"Minting needs API-key credentials; a bearer token can't mint another.",
		Example: "  raynet auth token\n  raynet auth login --profile acme-bearer --host app.raynet.cz --auth-type bearer --token \"$(raynet auth token)\"",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.runAuthToken()
		},
	}

	c.AddCommand(login, status, cfgCmd, token)
	return c
}

// runAuthToken mints a bearer token: the bare token on stdout, or the API's
// response with --json/--jq.
func (a *app) runAuthToken() error {
	p, err := a.resolveProfile()
	if err != nil {
		return err
	}
	if p.AuthType != config.AuthAPIKey {
		return fmt.Errorf("auth token needs API key credentials (username + API key); the active credentials are %s", p.AuthType)
	}
	c, err := a.resolveClient()
	if err != nil {
		return err
	}
	resp, err := c.Do(http.MethodGet, "/security/bearertoken", nil, nil)
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := client.DecodeJSON(resp, &raw); err != nil {
		return err
	}
	if a.flags.json || a.flags.jq != "" {
		return a.printJSON(raw)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
		return fmt.Errorf("unexpected bearer token response: %s", raw)
	}
	_, err = fmt.Fprintln(a.stdout, out.Token)
	return err
}

// errNoHost is parseHost's error for input that names no host; each caller
// reports it in its own words.
var errNoHost = errors.New("no host")

// parseHost turns a --host value into a host and scheme ("" when none was
// given): a bare host or a URL, plain http only for localhost.
func parseHost(input string) (string, string, error) {
	host, scheme, err := parseHostInput(input)
	if err != nil {
		return "", "", err
	}
	if host == "" {
		return "", "", errNoHost
	}
	if err := validateHostScheme(host, scheme); err != nil {
		return "", "", err
	}
	return host, scheme, nil
}

// parseHostInput accepts either a bare host (app.raynet.cz, localhost:8080)
// or a URL with an explicit scheme (http://localhost:8080) and returns the
// bare host plus the scheme ("" when none was given).
func parseHostInput(input string) (string, string, error) {
	in := strings.TrimSpace(input)
	if i := strings.Index(in, "://"); i >= 0 {
		scheme := strings.ToLower(in[:i])
		if scheme != "http" && scheme != "https" {
			return "", "", fmt.Errorf("unknown scheme %q in %q, expected http:// or https://", scheme, input)
		}
		return strings.TrimRight(in[i+3:], "/"), scheme, nil
	}
	return in, "", nil
}

// validateHostScheme rejects plain http for non-local hosts: localhost
// development servers run without TLS, but real RAYNET CRM endpoints must
// be reached over https.
func validateHostScheme(host, scheme string) error {
	if scheme != "http" {
		return nil
	}
	h := host
	if strings.HasPrefix(h, "[") {
		if j := strings.Index(h, "]"); j > 0 {
			h = h[1:j]
		}
	} else if i := strings.LastIndex(h, ":"); i >= 0 {
		h = h[:i]
	}
	switch h {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return nil
	}
	return fmt.Errorf("host %q must use https (http is only allowed for localhost)", host)
}

// runAuthLogin stores a profile from o once the API accepted its
// credentials. Interactive, the wizard asks every question no flag
// answered; otherwise a missing value is an error naming its flag, and a
// failed check saves nothing.
func (a *app) runAuthLogin(o *loginOptions) error {
	var w *wizard
	if a.isInteractive() {
		w = a.newWizard()
		if err := w.credentials(o); err != nil {
			return err
		}
	}
	p, err := o.loginProfile()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	c := client.New(p)
	c.Verbose = a.flags.verbose
	c.Log = a.stderr
	info, err := fetchSecurityInfo(c)
	if err != nil {
		err = fmt.Errorf("credentials could not be verified, nothing saved: %w", err)
		if w == nil {
			return err
		}
		fmt.Fprintf(a.stderr, "The credentials could not be verified: %v\n", errors.Unwrap(err))
		save, askErr := w.confirm("Save anyway?")
		if askErr != nil {
			return askErr
		}
		if !save {
			return err
		}
		info = nil
	}

	suggested := p.InstanceName
	if info != nil {
		suggested = info.InstanceName
		// Bearer and instance-id profiles learn their instance name here,
		// so --web links need no extra request.
		if p.InstanceName == "" {
			p.WebInstanceName = info.InstanceName
		}
	}
	name := o.profile
	if name == "" && w != nil {
		if name, err = w.profileName(cfg, suggested); err != nil {
			return err
		}
	}
	if name == "" {
		name = suggested
	}
	if name == "" {
		return errors.New("the API reported no instance name to name the profile after; pass --profile")
	}

	cfg.Profiles[name] = p
	switch {
	case o.setDefault || cfg.Default == "" || cfg.Default == name:
		cfg.Default = name
	case w != nil:
		makeDefault, err := w.confirm(fmt.Sprintf("Make %q the default profile?", name))
		if err != nil {
			return err
		}
		if makeDefault {
			cfg.Default = name
		}
	}
	if err := config.Save(cfg); err != nil {
		return err
	}

	if info == nil {
		fmt.Fprintf(a.stdout, "Saved profile %q: %s, credentials not verified", name, p.Host)
	} else {
		fmt.Fprintf(a.stdout, "Logged in as profile %q: %s as %s (instance: %s)", name, p.Host, info.Username, info.InstanceName)
	}
	if cfg.Default == name {
		fmt.Fprint(a.stdout, ", set as default")
	}
	fmt.Fprintln(a.stdout, ".")
	return nil
}

// loginMethod is one of the three ways to log in. Each maps onto an auth
// type and needs its own login fields.
type loginMethod string

const (
	methodInstanceName loginMethod = "instance-name"
	methodInstanceID   loginMethod = "instance-id"
	methodBearer       loginMethod = "bearer"
)

// label is the method's choice in the wizard's menu.
func (m loginMethod) label() string {
	switch m {
	case methodInstanceName:
		return "Instance name + username + API key"
	case methodInstanceID:
		return "Instance ID + username + API key"
	default:
		return "Bearer token"
	}
}

func (m loginMethod) authType() config.AuthType {
	if m == methodBearer {
		return config.AuthBearer
	}
	return config.AuthAPIKey
}

// fields are the values the method needs, in the order the wizard asks.
func (m loginMethod) fields() []loginField {
	switch m {
	case methodInstanceName:
		return []loginField{fieldInstanceName, fieldUsername, fieldAPIKey}
	case methodInstanceID:
		return []loginField{fieldInstanceID, fieldUsername, fieldAPIKey}
	default:
		return []loginField{fieldToken}
	}
}

// loginField is one value a login method needs: the flag that supplies it
// and the wizard's question for it.
type loginField struct {
	flag   string
	title  string
	hidden bool // an API key or bearer token, hidden while typed
	value  func(o *loginOptions) *string
}

var (
	fieldInstanceName = loginField{"--instance-name", "Instance name", false, func(o *loginOptions) *string { return &o.instanceName }}
	fieldInstanceID   = loginField{"--instance-id", "Instance ID", false, func(o *loginOptions) *string { return &o.instanceID }}
	fieldUsername     = loginField{"--username", "Username", false, func(o *loginOptions) *string { return &o.username }}
	fieldAPIKey       = loginField{"--api-key", "API key", true, func(o *loginOptions) *string { return &o.apiKey }}
	fieldToken        = loginField{"--token", "Bearer token", true, func(o *loginOptions) *string { return &o.token }}
)

// methods returns the login methods o's flags leave open: one when
// --auth-type, --instance-name or --instance-id picks it, several to choose
// from, none for an unknown --auth-type (loginProfile reports it).
func (o *loginOptions) methods() []loginMethod {
	authType := config.AuthType(o.authType)
	switch {
	case authType == config.AuthBearer:
		return []loginMethod{methodBearer}
	case authType != config.AuthAPIKey && authType != "":
		return nil
	case o.instanceName != "":
		return []loginMethod{methodInstanceName}
	case o.instanceID != "":
		return []loginMethod{methodInstanceID}
	case authType == config.AuthAPIKey:
		return []loginMethod{methodInstanceID, methodInstanceName}
	}
	return []loginMethod{methodBearer, methodInstanceID, methodInstanceName}
}

// missing returns the fields of m that o has no value for yet.
func (o *loginOptions) missing(m loginMethod) []loginField {
	var out []loginField
	for _, f := range m.fields() {
		if *f.value(o) == "" {
			out = append(out, f)
		}
	}
	return out
}

// loginProfile builds the profile to log in with from o, failing on the
// first missing value with the flag that supplies it.
func (o *loginOptions) loginProfile() (config.Profile, error) {
	host, scheme, err := parseHost(o.host)
	if errors.Is(err, errNoHost) {
		return config.Profile{}, errors.New("missing --host")
	}
	if err != nil {
		return config.Profile{}, err
	}

	switch config.AuthType(o.authType) {
	case "":
		return config.Profile{}, errors.New("missing --auth-type")
	case config.AuthAPIKey, config.AuthBearer:
	default:
		return config.Profile{}, fmt.Errorf("unknown --auth-type %q, expected apikey or bearer", o.authType)
	}
	methods := o.methods()
	if len(methods) != 1 {
		return config.Profile{}, errors.New("missing --instance-name or --instance-id")
	}
	m := methods[0]
	if missing := o.missing(m); len(missing) > 0 {
		return config.Profile{}, fmt.Errorf("missing %s", missing[0].flag)
	}

	p := config.Profile{Host: host, Scheme: scheme, AuthType: m.authType(), WebURL: strings.TrimRight(o.webURL, "/")}
	if m == methodBearer {
		p.Token = o.token
	} else {
		p.Username, p.APIKey, p.InstanceName, p.InstanceID = o.username, o.apiKey, o.instanceName, o.instanceID
	}
	return p, nil
}

// securityInfo mirrors the response shape of GET /security/info.
type securityInfo struct {
	Username      string `json:"username"`
	UserAccountID int    `json:"userAccountId"`
	InstanceName  string `json:"X-Instance-Name"`
	InstanceID    string `json:"X-Instance-Id"`
}

// fetchSecurityInfo asks the API who and which instance c's credentials
// resolve to.
func fetchSecurityInfo(c *client.Client) (*securityInfo, error) {
	resp, err := c.Do(http.MethodGet, "/security/info", nil, nil)
	if err != nil {
		return nil, err
	}
	var info securityInfo
	if err := client.DecodeJSON(resp, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (a *app) runAuthStatus() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(a.stdout, "No profiles configured. Run `raynet auth login` to add one.")
		return nil
	}
	for name, p := range cfg.Profiles {
		marker := ""
		if name == cfg.Default {
			marker = " (default)"
		}
		fmt.Fprintf(a.stdout, "%s%s — host: %s, auth: %s\n", name, marker, p.Host, p.AuthType)
	}
	return nil
}
