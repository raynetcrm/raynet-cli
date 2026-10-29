/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

// In-process test harness: runs the whole command tree with arguments
// against an httptest server standing in for the REST API, reached through
// the ad-hoc --host/--token flags. Tests assert only external behaviour: the
// requests the fake API received, stdout/stderr, the returned error and any
// URL the CLI tried to open in a browser.
//
//	h := newHarness(t)
//	h.api.reply("GET", "/company/42/", 200, `{"data":{"id":42}}`)
//	res := h.run("company", "view", "42")
//	req := h.api.lastRequest()  // req.method, req.path, req.query, req.json(t)

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/raynetcrm/raynet-cli/internal/config"
)

// apiPrefix is the REST API base path the client prepends; recorded request
// paths and registered routes are relative to it, e.g. "/company/".
const apiPrefix = "/api/v2"

// apiRequest is one request the fake API received.
type apiRequest struct {
	method string
	path   string // without apiPrefix, e.g. "/company/42/"
	query  url.Values
	header http.Header
	body   []byte
}

// json decodes the request body as JSON (nil for an empty body).
func (r apiRequest) json(t *testing.T) any {
	t.Helper()
	if len(r.body) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(r.body, &v); err != nil {
		t.Fatalf("request body of %s %s is not JSON: %v\n%s", r.method, r.path, err, r.body)
	}
	return v
}

// apiHandler computes a fake API response for a request: status and raw body.
type apiHandler func(r apiRequest) (status int, body string)

type apiRoute struct {
	method, path string
	handler      apiHandler
}

// fakeAPI is an httptest server standing in for the RAYNET CRM REST API. It
// records every request and answers registered routes; anything else gets a
// 404 with an empty body, as the real API does for unknown routes.
type fakeAPI struct {
	t      *testing.T
	server *httptest.Server

	mu       sync.Mutex
	routes   []apiRoute
	received []apiRequest
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{t: t}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		f.t.Errorf("fake API: reading request body: %v", err)
	}
	req := apiRequest{
		method: r.Method,
		path:   strings.TrimPrefix(r.URL.Path, apiPrefix),
		query:  r.URL.Query(),
		header: r.Header.Clone(),
		body:   body,
	}

	f.mu.Lock()
	f.received = append(f.received, req)
	var handler apiHandler
	for _, rt := range f.routes {
		if rt.method == req.method && rt.path == req.path {
			handler = rt.handler
			break
		}
	}
	f.mu.Unlock()

	if handler == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	status, respBody := handler(req)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, respBody)
}

// handle registers a handler for method + path (path without apiPrefix,
// query excluded). The first registered match wins.
func (f *fakeAPI) handle(method, path string, h apiHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes = append(f.routes, apiRoute{method: method, path: path, handler: h})
}

// reply registers a canned response for method + path.
func (f *fakeAPI) reply(method, path string, status int, body string) {
	f.handle(method, path, func(apiRequest) (int, string) { return status, body })
}

// requests returns every request received so far, in order.
func (f *fakeAPI) requests() []apiRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]apiRequest(nil), f.received...)
}

// lastRequest returns the most recent request, failing the test if none.
func (f *fakeAPI) lastRequest() apiRequest {
	f.t.Helper()
	reqs := f.requests()
	if len(reqs) == 0 {
		f.t.Fatal("fake API received no requests")
	}
	return reqs[len(reqs)-1]
}

// hostFlag is the --host value that reaches this fake API.
func (f *fakeAPI) hostFlag() string {
	return f.server.URL
}

// profile returns a bearer-token profile that reaches this fake API, for
// tests that go through stored config (see harness.storeProfile).
func (f *fakeAPI) profile() config.Profile {
	u, err := url.Parse(f.server.URL)
	if err != nil {
		f.t.Fatalf("fake API URL: %v", err)
	}
	return config.Profile{Host: u.Host, Scheme: u.Scheme, AuthType: config.AuthBearer, Token: "profile-token"}
}

// harness runs the raynet command tree in-process. Each run builds a fresh
// command tree, so flag state never leaks between runs.
type harness struct {
	t   *testing.T
	api *fakeAPI
	// terminal simulates stdout being an interactive terminal.
	terminal bool
	// interactive simulates stdin being an interactive terminal, so commands
	// may prompt; off (the default), they must not.
	interactive bool
	// stdinTerminal simulates stdin being a real terminal device, which the
	// wizard needs to hide an API key or bearer token while typed. stdin
	// stays a plain reader, so huh can't actually read hidden input.
	stdinTerminal bool
	// accessible simulates $RAYNET_CLI_ACCESSIBLE being set.
	accessible bool
	// stdin is what the command reads from standard input.
	stdin string
	// configPath is the temporary config file ($RAYNET_CLI_CONFIG).
	configPath string
}

// runResult is what one CLI invocation produced.
type runResult struct {
	stdout string
	stderr string
	err    error
	// browserURLs are the URLs the command asked to open in a browser.
	browserURLs []string
}

// newHarness creates a harness with its own fake API and an empty temporary
// config file, so no test touches the real user config.
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		t:          t,
		api:        newFakeAPI(t),
		configPath: filepath.Join(t.TempDir(), "config.yml"),
	}
	t.Setenv("RAYNET_CLI_CONFIG", h.configPath)
	t.Setenv("RAYNET_CLI_PROFILE", "")
	return h
}

// storeProfile saves p under name in the harness's config file, making it the
// default profile when it is the first one.
func (h *harness) storeProfile(name string, p config.Profile) {
	h.t.Helper()
	cfg := h.loadConfig()
	cfg.Profiles[name] = p
	if cfg.Default == "" {
		cfg.Default = name
	}
	if err := config.Save(cfg); err != nil {
		h.t.Fatalf("config.Save() error = %v", err)
	}
}

// loadConfig reads the harness's config file.
func (h *harness) loadConfig() *config.Config {
	h.t.Helper()
	cfg, err := config.Load()
	if err != nil {
		h.t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}

// run executes raynet with args plus ad-hoc credentials pointing at the fake
// API (--host <fake> --token test-token).
func (h *harness) run(args ...string) runResult {
	h.t.Helper()
	return h.runRaw(append(args, "--host", h.api.hostFlag(), "--token", "test-token")...)
}

// runAPIKey executes raynet with args plus ad-hoc API-key credentials
// pointing at the fake API (username alice, API key "key", instance acme),
// for commands that need basic auth rather than a bearer token.
func (h *harness) runAPIKey(args ...string) runResult {
	h.t.Helper()
	return h.runRaw(append(args, "--host", h.api.hostFlag(), "--username", "alice",
		"--api-key", "key", "--instance-name", "acme")...)
}

// runRaw executes raynet with exactly args: no ad-hoc credentials are added,
// so stored profiles (see configPath) are used.
func (h *harness) runRaw(args ...string) runResult {
	h.t.Helper()
	var stdout, stderr bytes.Buffer
	res := runResult{}
	a := &app{
		stdin:         strings.NewReader(h.stdin),
		stdout:        &stdout,
		stderr:        &stderr,
		isTerminal:    func() bool { return h.terminal },
		isInteractive: func() bool { return h.interactive },
		stdinTerminal: func() bool { return h.stdinTerminal },
		accessible:    func() bool { return h.accessible },
		openBrowser: func(u string) error {
			res.browserURLs = append(res.browserURLs, u)
			return nil
		},
	}
	res.err = a.execute(args)
	res.stdout = stdout.String()
	res.stderr = stderr.String()
	return res
}
