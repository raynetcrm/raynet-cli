/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package config

import (
	"path/filepath"
	"testing"
)

// withTempConfig points $RAYNET_CLI_CONFIG at a fresh file in t.TempDir() for
// the duration of the test.
func withTempConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("RAYNET_CLI_CONFIG", path)
	return path
}

func TestPath_EnvOverride(t *testing.T) {
	want := withTempConfig(t)
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoad_MissingFileReturnsEmptyConfig(t *testing.T) {
	withTempConfig(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profiles == nil {
		t.Error("Load() on a missing file should still initialize Profiles")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("Load() on a missing file should have no profiles, got %v", cfg.Profiles)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	withTempConfig(t)

	cfg := &Config{
		Profiles: map[string]Profile{
			"app.raynet.cz": {
				Host:         "app.raynet.cz",
				AuthType:     AuthAPIKey,
				Username:     "alice",
				APIKey:       "secret",
				InstanceName: "my-crm",
			},
			"local": {
				Host:     "localhost:8080",
				AuthType: AuthBearer,
				Token:    "tok",
				Scheme:   "http",
			},
		},
		Default: "app.raynet.cz",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Default != cfg.Default {
		t.Errorf("Default = %q, want %q", got.Default, cfg.Default)
	}
	if len(got.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(got.Profiles))
	}
	if got.Profiles["app.raynet.cz"] != cfg.Profiles["app.raynet.cz"] {
		t.Errorf("Profiles[app.raynet.cz] = %+v, want %+v", got.Profiles["app.raynet.cz"], cfg.Profiles["app.raynet.cz"])
	}
	if got.Profiles["local"] != cfg.Profiles["local"] {
		t.Errorf("Profiles[local] = %+v, want %+v", got.Profiles["local"], cfg.Profiles["local"])
	}
}

// TestSaveLoad_TwoProfilesSameHost is the whole point of profiles being
// independent of host: two customer instances on the same server host, each
// with their own credentials.
func TestSaveLoad_TwoProfilesSameHost(t *testing.T) {
	withTempConfig(t)

	cfg := &Config{
		Profiles: map[string]Profile{
			"acme": {
				Host: "app.raynet.cz", AuthType: AuthAPIKey,
				Username: "acme-user", APIKey: "acme-key", InstanceName: "acme-instance",
			},
			"widgets-inc": {
				Host: "app.raynet.cz", AuthType: AuthAPIKey,
				Username: "widgets-user", APIKey: "widgets-key", InstanceName: "widgets-instance",
			},
		},
		Default: "acme",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Profiles["acme"].Host != "app.raynet.cz" || got.Profiles["widgets-inc"].Host != "app.raynet.cz" {
		t.Fatalf("expected both profiles to target app.raynet.cz, got %+v", got.Profiles)
	}
	if got.Profiles["acme"].InstanceName == got.Profiles["widgets-inc"].InstanceName {
		t.Error("expected the two profiles to keep distinct instance names")
	}
}

func TestProfile_GetScheme(t *testing.T) {
	tests := []struct {
		name string
		p    Profile
		want string
	}{
		{"defaults to https", Profile{}, "https"},
		{"explicit http", Profile{Scheme: "http"}, "http"},
		{"explicit https", Profile{Scheme: "https"}, "https"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.GetScheme(); got != tt.want {
				t.Errorf("GetScheme() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_ActiveProfile(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"app.raynet.cz": {Host: "app.raynet.cz", AuthType: AuthAPIKey},
		},
		Default: "app.raynet.cz",
	}

	t.Run("uses default when override is empty", func(t *testing.T) {
		name, p, err := cfg.ActiveProfile("")
		if err != nil {
			t.Fatalf("ActiveProfile() error = %v", err)
		}
		if name != "app.raynet.cz" {
			t.Errorf("name = %q, want app.raynet.cz", name)
		}
		if p.AuthType != AuthAPIKey {
			t.Errorf("AuthType = %q, want %q", p.AuthType, AuthAPIKey)
		}
	})

	t.Run("override wins over default", func(t *testing.T) {
		cfg := &Config{
			Profiles: map[string]Profile{
				"acme":    {Host: "app.raynet.cz", AuthType: AuthAPIKey},
				"widgets": {Host: "eu.raynetcrm.com", AuthType: AuthBearer},
			},
			Default: "acme",
		}
		name, p, err := cfg.ActiveProfile("widgets")
		if err != nil {
			t.Fatalf("ActiveProfile() error = %v", err)
		}
		if name != "widgets" {
			t.Errorf("name = %q, want widgets", name)
		}
		if p.AuthType != AuthBearer {
			t.Errorf("AuthType = %q, want %q", p.AuthType, AuthBearer)
		}
	})

	t.Run("errors when no profile and no default", func(t *testing.T) {
		empty := &Config{Profiles: map[string]Profile{}}
		if _, _, err := empty.ActiveProfile(""); err == nil {
			t.Error("ActiveProfile() expected an error when no profile/default is set")
		}
	})

	t.Run("errors when profile is not configured", func(t *testing.T) {
		if _, _, err := cfg.ActiveProfile("unknown-profile"); err == nil {
			t.Error("ActiveProfile() expected an error for an unconfigured profile")
		}
	})
}
