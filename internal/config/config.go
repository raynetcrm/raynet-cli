/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

// Package config reads and writes the raynet-cli config file
// (~/.config/raynet-cli/config.yml), which stores one or more named profiles.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AuthType identifies which RAYNET CRM auth scheme a profile uses.
type AuthType string

// Supported auth schemes for a profile.
const (
	AuthAPIKey AuthType = "apikey"
	AuthBearer AuthType = "bearer"
)

// Profile holds one named set of credentials for a RAYNET CRM host. Profile
// names are independent of Host — two profiles can target the same host
// (e.g. two different customer instances both on app.raynet.cz,
// disambiguated by InstanceName/InstanceID), stripe-cli style.
type Profile struct {
	Host         string   `yaml:"host"`
	AuthType     AuthType `yaml:"authType"`
	Username     string   `yaml:"username,omitempty"`
	APIKey       string   `yaml:"apiKey,omitempty"`
	InstanceName string   `yaml:"instanceName,omitempty"`
	InstanceID   string   `yaml:"instanceId,omitempty"`
	Token        string   `yaml:"token,omitempty"`
	Scheme       string   `yaml:"scheme,omitempty"` // http or https; defaults to https
	// WebURL is the CRM web app's base URL for --web links, e.g.
	// http://localhost:3000 on a dev stack where the web app and the API
	// listen on different ports; empty means the API's scheme + host.
	WebURL string `yaml:"webUrl,omitempty"`
	// WebInstanceName is the instance name for --web links, learned by
	// `auth login` on bearer and instance-id profiles. It is never sent as
	// an auth header, so those profiles keep authenticating as configured.
	WebInstanceName string `yaml:"webInstanceName,omitempty"`
}

// GetScheme returns the scheme for this profile (http or https), defaulting to https.
func (p Profile) GetScheme() string {
	if p.Scheme != "" {
		return p.Scheme
	}
	return "https"
}

// Config is the on-disk shape of config.yml.
type Config struct {
	Profiles map[string]Profile `yaml:"profiles"`
	Default  string             `yaml:"default"`
}

// KnownHosts are the RAYNET CRM server hosts documented in openapi.yaml.
var KnownHosts = []string{
	"app.raynet.cz",
	"app.raynetcrm.sk",
	"app.raynetcrm.com",
	"eu.raynetcrm.com",
}

// Path returns the config file location, honoring $RAYNET_CLI_CONFIG for
// tests/overrides. Otherwise it's always ~/.config/raynet-cli/config.yml
// (honoring $XDG_CONFIG_HOME), on every OS including macOS — the same
// dotfile-style convention stripe-cli, gh, and glab use, deliberately not
// the OS-native os.UserConfigDir() location (~/Library/Application Support
// on macOS), since that's where developers using a terminal tool expect it.
func Path() (string, error) {
	if p := os.Getenv("RAYNET_CLI_CONFIG"); p != "" {
		return p, nil
	}
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "raynet-cli", "config.yml"), nil
}

// Load reads the config file, returning an empty Config if it doesn't exist yet.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return &cfg, nil
}

// Save writes the config file, creating its parent directory (mode 0700) and
// using mode 0600 since it holds credentials.
func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

// ActiveProfile resolves the profile to use: explicit override, else the config default.
func (c *Config) ActiveProfile(override string) (string, Profile, error) {
	name := override
	if name == "" {
		name = c.Default
	}
	if name == "" {
		return "", Profile{}, fmt.Errorf("no profile specified and no default set; run `raynet auth login` first")
	}
	p, ok := c.Profiles[name]
	if !ok {
		return "", Profile{}, fmt.Errorf("no profile named %q; run `raynet auth login --profile %s`", name, name)
	}
	return name, p, nil
}
