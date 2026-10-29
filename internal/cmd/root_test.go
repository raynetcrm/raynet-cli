/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"testing"

	"github.com/raynetcrm/raynet-cli/internal/client"
	"github.com/raynetcrm/raynet-cli/internal/config"
)

// resolve runs resolveClient for an app with the given global flags; every
// call starts from a fresh app, so no flag state is shared between tests.
func resolve(f globalFlags) (*client.Client, error) {
	a := newApp()
	a.flags = f
	return a.resolveClient()
}

func TestResolveClient_AdHocBearerOverride(t *testing.T) {

	c, err := resolve(globalFlags{host: "http://localhost:8080", token: "adhoc-tok"})
	if err != nil {
		t.Fatalf("resolveClient() error = %v", err)
	}
	if c.Profile.AuthType != config.AuthBearer || c.Profile.Token != "adhoc-tok" {
		t.Errorf("Profile = %+v, want bearer/adhoc-tok", c.Profile)
	}
	if c.BaseURL != "http://localhost:8080/api/v2" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
}

func TestResolveClient_AdHocAPIKeyOverride(t *testing.T) {

	c, err := resolve(globalFlags{host: "app.raynet.cz", apiKey: "key123", username: "alice", instanceName: "my-crm"})
	if err != nil {
		t.Fatalf("resolveClient() error = %v", err)
	}
	if c.Profile.AuthType != config.AuthAPIKey || c.Profile.APIKey != "key123" || c.Profile.Username != "alice" {
		t.Errorf("Profile = %+v, want apikey/key123/alice", c.Profile)
	}
}

func TestResolveClient_AdHocHostWithScheme(t *testing.T) {

	c, err := resolve(globalFlags{host: "http://localhost:8082", token: "adhoc-tok"})
	if err != nil {
		t.Fatalf("resolveClient() error = %v", err)
	}
	if c.BaseURL != "http://localhost:8082/api/v2" {
		t.Errorf("BaseURL = %q, want http://localhost:8082/api/v2", c.BaseURL)
	}
}

func TestResolveClient_AdHocHTTPRejectedForNonLocalHost(t *testing.T) {

	if _, err := resolve(globalFlags{host: "http://app.raynet.cz", token: "adhoc-tok"}); err == nil {
		t.Error("resolveClient() expected an error for http on a non-local host")
	}
}

func TestResolveClient_AdHocRequiresHost(t *testing.T) {

	if _, err := resolve(globalFlags{token: "adhoc-tok"}); err == nil {
		t.Error("resolveClient() expected an error when --token is set without --host")
	}
}

func TestResolveClient_FallsBackToStoredConfig(t *testing.T) {
	path := t.TempDir() + "/config.yml"
	t.Setenv("RAYNET_CLI_CONFIG", path)
	t.Setenv("RAYNET_CLI_PROFILE", "")

	if err := config.Save(&config.Config{
		Profiles: map[string]config.Profile{
			"app.raynet.cz": {Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "stored-tok"},
		},
		Default: "app.raynet.cz",
	}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	c, err := resolve(globalFlags{})
	if err != nil {
		t.Fatalf("resolveClient() error = %v", err)
	}
	if c.Profile.Token != "stored-tok" {
		t.Errorf("Profile.Token = %q, want stored-tok", c.Profile.Token)
	}
}

// TestResolveClient_ProfileFlagSelectsAmongMultiple exercises the actual
// point of named profiles: two profiles targeting the same host, selected
// explicitly via --profile rather than by host.
func TestResolveClient_ProfileFlagSelectsAmongMultiple(t *testing.T) {
	path := t.TempDir() + "/config.yml"
	t.Setenv("RAYNET_CLI_CONFIG", path)
	t.Setenv("RAYNET_CLI_PROFILE", "")

	if err := config.Save(&config.Config{
		Profiles: map[string]config.Profile{
			"acme":        {Host: "app.raynet.cz", AuthType: config.AuthAPIKey, Username: "a", APIKey: "ak", InstanceName: "acme-instance"},
			"widgets-inc": {Host: "app.raynet.cz", AuthType: config.AuthAPIKey, Username: "w", APIKey: "wk", InstanceName: "widgets-instance"},
		},
		Default: "acme",
	}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	c, err := resolve(globalFlags{profile: "widgets-inc"})
	if err != nil {
		t.Fatalf("resolveClient() error = %v", err)
	}
	if c.Profile.InstanceName != "widgets-instance" {
		t.Errorf("InstanceName = %q, want widgets-instance", c.Profile.InstanceName)
	}
}

func TestResolveClient_ProfileEnvVarFallback(t *testing.T) {
	path := t.TempDir() + "/config.yml"
	t.Setenv("RAYNET_CLI_CONFIG", path)
	t.Setenv("RAYNET_CLI_PROFILE", "widgets-inc")

	if err := config.Save(&config.Config{
		Profiles: map[string]config.Profile{
			"acme":        {Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "acme-tok"},
			"widgets-inc": {Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "widgets-tok"},
		},
		Default: "acme",
	}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	c, err := resolve(globalFlags{})
	if err != nil {
		t.Fatalf("resolveClient() error = %v", err)
	}
	if c.Profile.Token != "widgets-tok" {
		t.Errorf("Profile.Token = %q, want widgets-tok ($RAYNET_CLI_PROFILE should win over default)", c.Profile.Token)
	}
}
