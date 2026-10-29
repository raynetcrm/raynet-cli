/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/raynetcrm/raynet-cli/internal/config"
)

// loginWizard runs `auth login` interactively with args, feeding stdin as
// the wizard's answers, one per line.
func (h *harness) loginWizard(stdin string, args ...string) runResult {
	h.t.Helper()
	h.interactive = true
	h.stdin = stdin
	return h.runRaw(append([]string{"auth", "login"}, args...)...)
}

func TestAuthLoginWizard_BearerTokenMethod(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)

	res := h.loginWizard("1\ntok-2\n\n", "--host", h.api.hostFlag())

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	for _, want := range []string{"Login method", "Bearer token", "Profile name"} {
		if !strings.Contains(res.stderr, want) {
			t.Errorf("stderr = %q, want the %q prompt", res.stderr, want)
		}
	}
	if strings.Contains(res.stderr, "default profile?") {
		t.Errorf("stderr = %q, want no make-default question for the first profile", res.stderr)
	}
	if strings.Contains(res.stdout, "Login method") {
		t.Errorf("stdout = %q, want no prompts", res.stdout)
	}
	if got := h.api.lastRequest().header.Get("Authorization"); got != "Bearer tok-2" {
		t.Errorf("check Authorization = %q, want Bearer tok-2", got)
	}
	cfg := h.loadConfig()
	if got := cfg.Profiles["acme"].Token; got != "tok-2" || cfg.Default != "acme" {
		t.Errorf("config = %+v, want profile acme (the suggested name) with tok-2, as default", cfg)
	}
}

func TestAuthLoginWizard_APIKeyMethods(t *testing.T) {
	t.Run("instance name", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)

		res := h.loginWizard("3\nacme\nalice\nk-1\n\n", "--host", h.api.hostFlag())

		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		req := h.api.lastRequest()
		user, key, _ := (&http.Request{Header: req.header}).BasicAuth()
		if user != "alice" || key != "k-1" || req.header.Get("X-Instance-Name") != "acme" {
			t.Errorf("check as %q/%q on %q, want alice/k-1 on acme", user, key, req.header.Get("X-Instance-Name"))
		}
		want := config.Profile{Host: h.api.profile().Host, Scheme: "http", AuthType: config.AuthAPIKey,
			Username: "alice", APIKey: "k-1", InstanceName: "acme"}
		if got := h.loadConfig().Profiles["acme"]; got != want {
			t.Errorf("profile acme = %+v, want %+v", got, want)
		}
	})

	t.Run("instance id", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)

		res := h.loginWizard("2\nabc123\nalice\nk-1\n\n", "--host", h.api.hostFlag())

		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		if !strings.Contains(res.stderr, "\nInstance ID \n") || strings.Contains(res.stderr, "\nInstance name \n") {
			t.Errorf("stderr = %q, want the instance ID question only", res.stderr)
		}
		want := config.Profile{Host: h.api.profile().Host, Scheme: "http", AuthType: config.AuthAPIKey,
			Username: "alice", APIKey: "k-1", InstanceID: "abc123", WebInstanceName: "acme"}
		if got := h.loadConfig().Profiles["acme"]; got != want {
			t.Errorf("profile acme = %+v, want %+v", got, want)
		}
	})
}

func TestAuthLoginWizard_HostOther(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)
	localhost := strings.Replace(h.api.hostFlag(), "127.0.0.1", "localhost", 1)
	other := len(config.KnownHosts) + 1

	res := h.loginWizard(fmt.Sprintf("%d\nhttp://app.raynet.cz\n%s\n1\ntok\n\n", other, localhost))

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	for _, want := range append(config.KnownHosts, "Other…", "must use https") {
		if !strings.Contains(res.stderr, want) {
			t.Errorf("stderr = %q, want %q", res.stderr, want)
		}
	}
	if n := len(h.api.requests()); n != 1 {
		t.Errorf("requests = %d, want only the check against localhost", n)
	}
	got := h.loadConfig().Profiles["acme"]
	if got.Scheme != "http" || !strings.HasPrefix(got.Host, "localhost:") {
		t.Errorf("profile = %+v, want http on localhost", got)
	}
}

func TestAuthLoginWizard_FlagsSkipTheirSteps(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  config.Profile
	}{
		{"--auth-type apikey --instance-id", []string{"--auth-type", "apikey", "--instance-id", "abc123"}, "alice\nk-1\n",
			config.Profile{AuthType: config.AuthAPIKey, Username: "alice", APIKey: "k-1", InstanceID: "abc123", WebInstanceName: "acme"}},
		{"--instance-name and --username", []string{"--instance-name", "acme", "--username", "alice"}, "k-1\n",
			config.Profile{AuthType: config.AuthAPIKey, Username: "alice", APIKey: "k-1", InstanceName: "acme"}},
		{"--auth-type bearer", []string{"--auth-type", "bearer"}, "tok\n",
			config.Profile{AuthType: config.AuthBearer, Token: "tok", WebInstanceName: "acme"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("GET", "/security/info", 200, securityInfoAcme)

			res := h.loginWizard(tt.stdin, append([]string{"--host", h.api.hostFlag(), "--profile", "dev"}, tt.args...)...)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			for _, skipped := range []string{"Host", "Login method", "Profile name"} {
				if strings.Contains(res.stderr, skipped) {
					t.Errorf("stderr = %q, want no %q question", res.stderr, skipped)
				}
			}
			tt.want.Host, tt.want.Scheme = h.api.profile().Host, "http"
			if got := h.loadConfig().Profiles["dev"]; got != tt.want {
				t.Errorf("profile dev = %+v, want %+v", got, tt.want)
			}
		})
	}

	t.Run("--auth-type apikey asks between the API-key methods", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)

		res := h.loginWizard("1\nabc123\nalice\nk-1\n", "--host", h.api.hostFlag(), "--profile", "dev", "--auth-type", "apikey")

		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		if !strings.Contains(res.stderr, "Login method") || strings.Contains(res.stderr, "Bearer token") {
			t.Errorf("stderr = %q, want a method menu without the bearer token", res.stderr)
		}
		if got := h.loadConfig().Profiles["dev"].InstanceID; got != "abc123" {
			t.Errorf("instance id = %q, want abc123", got)
		}
	})
}

func TestAuthLoginWizard_FailedCheckAsksSaveAnyway(t *testing.T) {
	t.Run("yes saves", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 401, `{"message":"Unauthorized"}`)

		res := h.loginWizard("y\ndev\n", "--host", h.api.hostFlag(), "--auth-type", "bearer", "--token", "bad")

		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		if !strings.Contains(res.stderr, "Save anyway?") {
			t.Errorf("stderr = %q, want the save-anyway question", res.stderr)
		}
		cfg := h.loadConfig()
		if cfg.Profiles["dev"].Token != "bad" || cfg.Default != "dev" {
			t.Errorf("config = %+v, want profile dev saved as default", cfg)
		}
		if !strings.Contains(res.stdout, "not verified") {
			t.Errorf("stdout = %q, want a not-verified note", res.stdout)
		}
	})

	t.Run("no saves nothing", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 401, `{"message":"Unauthorized"}`)

		res := h.loginWizard("n\n", "--host", h.api.hostFlag(), "--auth-type", "bearer", "--token", "bad")

		if res.err == nil || !strings.Contains(res.err.Error(), "nothing saved") {
			t.Fatalf("err = %v, want a nothing-saved error", res.err)
		}
		if res.stdout != "" {
			t.Errorf("stdout = %q, want nothing", res.stdout)
		}
		assertNothingSaved(t, h)
	})
}

func TestAuthLoginWizard_ProfileName(t *testing.T) {
	login := func(h *harness, stdin string) runResult {
		t.Helper()
		res := h.loginWizard(stdin, "--host", h.api.hostFlag(), "--auth-type", "bearer", "--token", "new", "--default")
		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		return res
	}

	t.Run("suggestion changed", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)

		res := login(h, "work\n")

		if !strings.Contains(res.stderr, "Profile name") {
			t.Errorf("stderr = %q, want the profile name question", res.stderr)
		}
		if cfg := h.loadConfig(); len(cfg.Profiles) != 1 || cfg.Profiles["work"].Token != "new" {
			t.Errorf("profiles = %+v, want only work", cfg.Profiles)
		}
	})

	t.Run("existing profile replaced on yes", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)
		h.storeProfile("acme", config.Profile{Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "old"})

		res := login(h, "\ny\n")

		if !strings.Contains(res.stderr, `Profile "acme" exists. Replace it?`) {
			t.Errorf("stderr = %q, want the replace question", res.stderr)
		}
		if got := h.loadConfig().Profiles["acme"].Token; got != "new" {
			t.Errorf("acme token = %q, want new", got)
		}
	})

	t.Run("existing profile kept on no, another name asked", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)
		h.storeProfile("acme", config.Profile{Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "old"})

		login(h, "\nn\nacme-2\n")

		cfg := h.loadConfig()
		if cfg.Profiles["acme"].Token != "old" || cfg.Profiles["acme-2"].Token != "new" {
			t.Errorf("profiles = %+v, want acme kept and acme-2 added", cfg.Profiles)
		}
	})

	t.Run("--profile replaces without asking", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)
		h.storeProfile("acme", config.Profile{Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "old"})

		res := h.loginWizard("", "--profile", "acme", "--host", h.api.hostFlag(), "--auth-type", "bearer", "--token", "new")

		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		if res.stderr != "" {
			t.Errorf("stderr = %q, want no questions", res.stderr)
		}
		if got := h.loadConfig().Profiles["acme"].Token; got != "new" {
			t.Errorf("acme token = %q, want new", got)
		}
	})
}

func TestAuthLoginWizard_MakeDefault(t *testing.T) {
	setup := func(t *testing.T) *harness {
		t.Helper()
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)
		h.storeProfile("first", config.Profile{Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "t"})
		return h
	}
	login := func(t *testing.T, h *harness, stdin string, extra ...string) runResult {
		t.Helper()
		res := h.loginWizard(stdin, append([]string{"--profile", "second", "--host", h.api.hostFlag(),
			"--auth-type", "bearer", "--token", "t"}, extra...)...)
		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		return res
	}

	t.Run("default answer is no", func(t *testing.T) {
		h := setup(t)

		res := login(t, h, "\n")

		if !strings.Contains(res.stderr, `Make "second" the default profile? [y/N]`) {
			t.Errorf("stderr = %q, want the make-default question, defaulting to no", res.stderr)
		}
		if got := h.loadConfig().Default; got != "first" {
			t.Errorf("default = %q, want first", got)
		}
	})

	t.Run("yes makes it the default", func(t *testing.T) {
		h := setup(t)

		login(t, h, "y\n")

		if got := h.loadConfig().Default; got != "second" {
			t.Errorf("default = %q, want second", got)
		}
	})

	t.Run("--default skips the question", func(t *testing.T) {
		h := setup(t)

		res := login(t, h, "", "--default")

		if res.stderr != "" {
			t.Errorf("stderr = %q, want no questions", res.stderr)
		}
		if got := h.loadConfig().Default; got != "second" {
			t.Errorf("default = %q, want second", got)
		}
	})
}

func TestAuthLoginWizard_TokenAloneDoesNotPickTheMethod(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)

	res := h.loginWizard("1\n", "--host", h.api.hostFlag(), "--profile", "dev", "--token", "tok")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if !strings.Contains(res.stderr, "Login method") {
		t.Errorf("stderr = %q, want the login method question", res.stderr)
	}
	if strings.Contains(res.stderr, "\nBearer token \n") {
		t.Errorf("stderr = %q, want no bearer token question: --token answers it", res.stderr)
	}
	if got := h.loadConfig().Profiles["dev"]; got.AuthType != config.AuthBearer || got.Token != "tok" {
		t.Errorf("profile dev = %+v, want bearer with tok", got)
	}
}

func TestAuthLoginWizard_AccessibleOnATerminal(t *testing.T) {
	t.Run("asks plain line prompts", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)
		h.stdinTerminal, h.accessible = true, true

		res := h.loginWizard("alice\n", "--host", h.api.hostFlag(), "--profile", "dev",
			"--instance-name", "acme", "--api-key", "k-1")

		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		if !strings.Contains(res.stderr, "Username") {
			t.Errorf("stderr = %q, want the username prompt", res.stderr)
		}
		if got := h.loadConfig().Profiles["dev"].Username; got != "alice" {
			t.Errorf("username = %q, want alice", got)
		}
	})

	t.Run("keeps the bearer token hidden", func(t *testing.T) {
		h := newHarness(t)
		h.stdinTerminal, h.accessible = true, true

		// The harness's stdin is no real terminal, so huh's hidden read
		// fails (silently, in accessible mode) and leaves the token empty,
		// where a plain line read would have taken "tok".
		res := h.loginWizard("tok\n", "--host", h.api.hostFlag(), "--profile", "dev", "--auth-type", "bearer")

		if res.err == nil || res.err.Error() != "missing --token" {
			t.Fatalf("err = %v, want missing --token: the hidden read needs a real terminal", res.err)
		}
		if n := len(h.api.requests()); n != 0 {
			t.Errorf("requests = %d, want no check", n)
		}
		assertNothingSaved(t, h)
	})
}

func TestAuthLoginWizard_MethodMenuOrder(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)

	res := h.loginWizard("1\ntok\n\n", "--host", h.api.hostFlag())

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	bearer := strings.Index(res.stderr, "Bearer token")
	id := strings.Index(res.stderr, "Instance ID + username + API key")
	name := strings.Index(res.stderr, "Instance name + username + API key")
	if bearer < 0 || id < 0 || name < 0 || bearer >= id || id >= name {
		t.Errorf("stderr = %q, want the methods in the order bearer token, instance ID, instance name", res.stderr)
	}
}
