/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/raynetcrm/raynet-cli/internal/config"
)

// assertNothingSaved fails the test if auth login wrote config.yml.
func assertNothingSaved(t *testing.T, h *harness) {
	t.Helper()
	if _, err := os.Stat(h.configPath); !os.IsNotExist(err) {
		t.Errorf("config.yml exists (stat err = %v), want nothing written", err)
	}
}

func TestAuthLogin_NonInteractiveMissingValueNamesFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"host", []string{"--auth-type", "bearer", "--token", "t"}, "missing --host"},
		{"auth type", []string{"--host", "app.raynet.cz"}, "missing --auth-type"},
		{"auth type with --token", []string{"--host", "app.raynet.cz", "--token", "t"}, "missing --auth-type"},
		{"token", []string{"--host", "app.raynet.cz", "--auth-type", "bearer"}, "missing --token"},
		{"username", []string{"--host", "app.raynet.cz", "--auth-type", "apikey", "--api-key", "k", "--instance-name", "acme"}, "missing --username"},
		{"api key", []string{"--host", "app.raynet.cz", "--auth-type", "apikey", "--username", "alice", "--instance-name", "acme"}, "missing --api-key"},
		{"instance", []string{"--host", "app.raynet.cz", "--auth-type", "apikey", "--username", "alice", "--api-key", "k"}, "missing --instance-name or --instance-id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.stdin = "should-not-be-read\n"

			res := h.runRaw(append([]string{"auth", "login"}, tt.args...)...)

			if res.err == nil || res.err.Error() != tt.want {
				t.Fatalf("err = %v, want %q", res.err, tt.want)
			}
			if res.stderr != "Error: "+tt.want+"\n" {
				t.Errorf("stderr = %q, want only the error, no prompts", res.stderr)
			}
			if n := len(h.api.requests()); n != 0 {
				t.Errorf("requests = %d, want none", n)
			}
			assertNothingSaved(t, h)
		})
	}
}

// securityInfoAcme is GET /security/info for user alice on instance acme.
const securityInfoAcme = `{"username":"alice","userAccountId":7,"X-Instance-Name":"acme","X-Instance-Id":"abc123"}`

func TestAuthLogin_APIKeyWithInstanceName(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)

	res := h.runRaw("auth", "login", "--host", h.api.hostFlag(), "--auth-type", "apikey",
		"--username", "alice", "--api-key", "k-1", "--instance-name", "acme")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	user, key, ok := (&http.Request{Header: req.header}).BasicAuth()
	if req.path != "/security/info" || !ok || user != "alice" || key != "k-1" || req.header.Get("X-Instance-Name") != "acme" {
		t.Errorf("check = %s %s, basic auth %q/%q, X-Instance-Name %q; want GET /security/info as alice/k-1 on acme",
			req.method, req.path, user, key, req.header.Get("X-Instance-Name"))
	}
	want := config.Profile{Host: h.api.profile().Host, Scheme: "http", AuthType: config.AuthAPIKey,
		Username: "alice", APIKey: "k-1", InstanceName: "acme"}
	if got := h.loadConfig().Profiles["acme"]; got != want {
		t.Errorf("profile acme = %+v, want %+v", got, want)
	}
	wantOut := `Logged in as profile "acme": ` + h.api.profile().Host + " as alice (instance: acme), set as default.\n"
	if res.stdout != wantOut {
		t.Errorf("stdout = %q, want %q", res.stdout, wantOut)
	}
}

func TestAuthLogin_APIKeyWithInstanceID(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)

	res := h.runRaw("auth", "login", "--host", h.api.hostFlag(), "--auth-type", "apikey",
		"--username", "alice", "--api-key", "k-1", "--instance-id", "abc123")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if got := h.api.lastRequest().header.Get("X-Instance-Id"); got != "abc123" {
		t.Errorf("check X-Instance-Id = %q, want abc123", got)
	}
	want := config.Profile{Host: h.api.profile().Host, Scheme: "http", AuthType: config.AuthAPIKey,
		Username: "alice", APIKey: "k-1", InstanceID: "abc123", WebInstanceName: "acme"}
	if got := h.loadConfig().Profiles["acme"]; got != want {
		t.Errorf("profile acme = %+v, want %+v", got, want)
	}
}

func TestAuthLogin_BearerTokenStoredAndSentAsGiven(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)
	token := "YWJjMTIzOmFsaWNlOmstMQ==.x+/y"

	res := h.runRaw("auth", "login", "--host", h.api.hostFlag(), "--auth-type", "bearer", "--token", token)

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if got := h.api.lastRequest().header.Get("Authorization"); got != "Bearer "+token {
		t.Errorf("check Authorization = %q, want %q", got, "Bearer "+token)
	}
	want := config.Profile{Host: h.api.profile().Host, Scheme: "http", AuthType: config.AuthBearer,
		Token: token, WebInstanceName: "acme"}
	if got := h.loadConfig().Profiles["acme"]; got != want {
		t.Errorf("profile acme = %+v, want %+v", got, want)
	}

	h.api.reply("GET", "/company/3/", 200, `{"data":{"id":3}}`)
	if res := h.runRaw("company", "view", "3"); res.err != nil {
		t.Fatalf("view err = %v", res.err)
	}
	if got := h.api.lastRequest().header.Get("Authorization"); got != "Bearer "+token {
		t.Errorf("after login Authorization = %q, want %q", got, "Bearer "+token)
	}
}

func TestAuthLogin_FailedCheckSavesNothing(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 401, `{"message":"Unauthorized"}`)

	res := h.runRaw("auth", "login", "--profile", "dev", "--host", h.api.hostFlag(),
		"--auth-type", "bearer", "--token", "bad")

	if res.err == nil || !strings.Contains(res.err.Error(), "could not be verified") {
		t.Fatalf("err = %v, want a could-not-be-verified error", res.err)
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want nothing", res.stdout)
	}
	assertNothingSaved(t, h)
}

func TestAuthLogin_RedirectingHostSavesNothing(t *testing.T) {
	h := newHarness(t)
	// A CRM web app sends API requests it doesn't recognise to its login page.
	h.api.reply("GET", "/security/info", 302, "")

	res := h.runRaw("auth", "login", "--profile", "dev", "--host", h.api.hostFlag(),
		"--auth-type", "bearer", "--token", "tok")

	if res.err == nil || !strings.Contains(res.err.Error(), "not the RAYNET CRM REST API") {
		t.Fatalf("err = %v, want an error saying the host is not the REST API", res.err)
	}
	assertNothingSaved(t, h)
}

func TestAuthLogin_SchemeOnlyFromHost(t *testing.T) {
	t.Run("--scheme is gone", func(t *testing.T) {
		h := newHarness(t)

		res := h.runRaw("auth", "login", "--host", "localhost:8080", "--scheme", "http",
			"--auth-type", "bearer", "--token", "t")

		if res.err == nil || !strings.Contains(res.err.Error(), "unknown flag: --scheme") {
			t.Errorf("err = %v, want unknown flag --scheme", res.err)
		}
		assertNothingSaved(t, h)
	})

	t.Run("http on localhost is accepted", func(t *testing.T) {
		h := newHarness(t)
		h.api.reply("GET", "/security/info", 200, securityInfoAcme)
		localhost := strings.Replace(h.api.hostFlag(), "127.0.0.1", "localhost", 1)

		res := h.runRaw("auth", "login", "--host", localhost, "--auth-type", "bearer", "--token", "t")

		if res.err != nil {
			t.Fatalf("err = %v", res.err)
		}
		got := h.loadConfig().Profiles["acme"]
		if got.Scheme != "http" || !strings.HasPrefix(got.Host, "localhost:") {
			t.Errorf("profile = %+v, want http on localhost", got)
		}
	})

	t.Run("http elsewhere is rejected", func(t *testing.T) {
		h := newHarness(t)

		res := h.runRaw("auth", "login", "--host", "http://app.raynet.cz", "--auth-type", "bearer", "--token", "t")

		if res.err == nil || !strings.Contains(res.err.Error(), "must use https") {
			t.Errorf("err = %v, want an https-only error", res.err)
		}
		if n := len(h.api.requests()); n != 0 {
			t.Errorf("requests = %d, want none", n)
		}
		assertNothingSaved(t, h)
	})
}

func TestAuthLogin_ProfileReplacedByName(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)
	h.storeProfile("acme", config.Profile{Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "old"})
	h.storeProfile("dev", config.Profile{Host: "app.raynet.cz", AuthType: config.AuthBearer, Token: "old-dev"})

	if res := h.runRaw("auth", "login", "--host", h.api.hostFlag(), "--auth-type", "bearer", "--token", "new"); res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res := h.runRaw("auth", "login", "--profile", "dev", "--host", h.api.hostFlag(), "--auth-type", "bearer", "--token", "new-dev"); res.err != nil {
		t.Fatalf("err = %v", res.err)
	}

	cfg := h.loadConfig()
	if len(cfg.Profiles) != 2 || cfg.Profiles["acme"].Token != "new" || cfg.Profiles["dev"].Token != "new-dev" {
		t.Errorf("profiles = %+v, want acme and dev replaced with the new tokens", cfg.Profiles)
	}
}

func TestAuthLogin_DefaultProfile(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, securityInfoAcme)
	login := func(name string, extra ...string) runResult {
		t.Helper()
		res := h.runRaw(append([]string{"auth", "login", "--profile", name, "--host", h.api.hostFlag(),
			"--auth-type", "bearer", "--token", "t"}, extra...)...)
		if res.err != nil {
			t.Fatalf("login %s err = %v", name, res.err)
		}
		return res
	}

	login("first")
	if got := h.loadConfig().Default; got != "first" {
		t.Errorf("after the first profile default = %q, want first", got)
	}

	res := login("second")
	if got := h.loadConfig().Default; got != "first" {
		t.Errorf("after a later profile default = %q, want first unchanged", got)
	}
	if strings.Contains(res.stdout, "default") {
		t.Errorf("stdout = %q, want no set-as-default note", res.stdout)
	}

	login("third", "--default")
	if got := h.loadConfig().Default; got != "third" {
		t.Errorf("after --default default = %q, want third", got)
	}
}

func TestAuthLogin_HelpUsesGlossaryTerms(t *testing.T) {
	h := newHarness(t)

	res := h.runRaw("auth", "login", "--help")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	for _, want := range []string{"API key", "bearer token"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("help lacks %q:\n%s", want, res.stdout)
		}
	}
	for _, gone := range []string{"--scheme", "defaults to --host", "defaults to the host"} {
		if strings.Contains(res.stdout, gone) {
			t.Errorf("help still mentions %q:\n%s", gone, res.stdout)
		}
	}
}
