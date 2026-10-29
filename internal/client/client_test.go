/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/raynetcrm/raynet-cli/internal/config"
)

func TestNew_BaseURL(t *testing.T) {
	tests := []struct {
		name string
		p    config.Profile
		want string
	}{
		{"defaults to https", config.Profile{Host: "app.raynet.cz"}, "https://app.raynet.cz/api/v2"},
		{"honors http scheme", config.Profile{Host: "localhost:8080", Scheme: "http"}, "http://localhost:8080/api/v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.p)
			if c.BaseURL != tt.want {
				t.Errorf("BaseURL = %q, want %q", c.BaseURL, tt.want)
			}
		})
	}
}

func TestDo_APIKeyAuth(t *testing.T) {
	var gotUser, gotPass string
	var gotOK bool
	var gotInstanceName string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, gotOK = r.BasicAuth()
		gotInstanceName = r.Header.Get("X-Instance-Name")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(config.Profile{
		Host:         server.URL[len("http://"):],
		AuthType:     config.AuthAPIKey,
		Username:     "alice",
		APIKey:       "secret",
		InstanceName: "my-crm",
		Scheme:       "http",
	})

	resp, err := c.Do(http.MethodGet, "/company/", nil, nil)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !gotOK || gotUser != "alice" || gotPass != "secret" {
		t.Errorf("BasicAuth = (%q, %q, %v), want (alice, secret, true)", gotUser, gotPass, gotOK)
	}
	if gotInstanceName != "my-crm" {
		t.Errorf("X-Instance-Name = %q, want my-crm", gotInstanceName)
	}
}

func TestDo_BearerAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(config.Profile{
		Host:     server.URL[len("http://"):],
		AuthType: config.AuthBearer,
		Token:    "tok123",
		Scheme:   "http",
	})

	resp, err := c.Do(http.MethodGet, "/company/", nil, nil)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if gotAuth != "Bearer tok123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok123")
	}
}

func TestDo_MissingCredentialsErrors(t *testing.T) {
	tests := []struct {
		name string
		p    config.Profile
	}{
		{"apikey without instance", config.Profile{AuthType: config.AuthAPIKey, Username: "u", APIKey: "k"}},
		{"apikey without username/key", config.Profile{AuthType: config.AuthAPIKey, InstanceName: "x"}},
		{"bearer without token", config.Profile{AuthType: config.AuthBearer}},
		{"unknown auth type", config.Profile{AuthType: "carrier-pigeon"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.p.Host = "example.invalid"
			c := New(tt.p)
			if _, err := c.Do(http.MethodGet, "/company/", nil, nil); err == nil {
				t.Error("Do() expected an error for incomplete credentials")
			}
		})
	}
}

func TestDecodeJSON(t *testing.T) {
	t.Run("decodes a 2xx body", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: jsonBody(`{"id": 1}`)}

		var out map[string]int
		if err := DecodeJSON(resp, &out); err != nil {
			t.Fatalf("DecodeJSON() error = %v", err)
		}
		if out["id"] != 1 {
			t.Errorf("out[id] = %d, want 1", out["id"])
		}
	})

	t.Run("errors on non-2xx status, including body in the message", func(t *testing.T) {
		resp := &http.Response{StatusCode: 404}
		resp.Body = jsonBody(`{"type":"NotFound","message":"no such company"}`)

		err := DecodeJSON(resp, nil)
		if err == nil {
			t.Fatal("DecodeJSON() expected an error for a 404")
		}
		if !strings.Contains(err.Error(), "no such company") {
			t.Errorf("error = %v, want it to include the response body", err)
		}
	})
}

func TestDecodeListResponse(t *testing.T) {
	t.Run("direct array", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: jsonBody(`[{"id":1},{"id":2}]`)}
		got, err := DecodeListResponse(resp)
		if err != nil {
			t.Fatalf("DecodeListResponse() error = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2", len(got))
		}
	})

	t.Run("wrapped in a data field", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: jsonBody(`{"data":[{"id":1}],"total":1}`)}
		got, err := DecodeListResponse(resp)
		if err != nil {
			t.Fatalf("DecodeListResponse() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
	})

	t.Run("object with no known list field returns empty", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: jsonBody(`{"foo":"bar"}`)}
		got, err := DecodeListResponse(resp)
		if err != nil {
			t.Fatalf("DecodeListResponse() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len(got) = %d, want 0", len(got))
		}
	})
}

func TestGetAllPages_WalksOffsetUntilShortPage(t *testing.T) {
	// Serve exactly MaxPageSize items on the first page, then a short
	// second page, to confirm offset/limit paging advances correctly and
	// stops as soon as a page comes back short.
	const secondPageSize = 3

	var gotOffsets []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		gotOffsets = append(gotOffsets, offset)

		w.Header().Set("Content-Type", "application/json")
		n := MaxPageSize
		if offset != "0" {
			n = secondPageSize
		}
		items := make([]int, n)
		data, _ := json.Marshal(items)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	c := New(config.Profile{Host: server.URL[len("http://"):], AuthType: config.AuthBearer, Token: "t", Scheme: "http"})

	all, err := c.GetAllPages("/company/", nil)
	if err != nil {
		t.Fatalf("GetAllPages() error = %v", err)
	}
	if want := MaxPageSize + secondPageSize; len(all) != want {
		t.Errorf("len(all) = %d, want %d", len(all), want)
	}
	if len(gotOffsets) != 2 || gotOffsets[0] != "0" || gotOffsets[1] != strconv.Itoa(MaxPageSize) {
		t.Errorf("offsets requested = %v, want [0 %d]", gotOffsets, MaxPageSize)
	}
}

// jsonBody wraps a string as an io.ReadCloser suitable for http.Response.Body.
func jsonBody(s string) *nopCloserReader {
	return &nopCloserReader{strings.NewReader(s)}
}

type nopCloserReader struct{ *strings.Reader }

func (nopCloserReader) Close() error { return nil }

func TestDo_RedirectIsAnError(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/acme/login" {
			_, _ = w.Write([]byte("<html>login</html>"))
			return
		}
		http.Redirect(w, r, "/acme/login", http.StatusFound)
	}))
	defer server.Close()

	c := New(config.Profile{Host: server.URL[len("http://"):], Scheme: "http", AuthType: config.AuthBearer, Token: "tok"})

	_, err := c.Do(http.MethodGet, "/security/info", nil, nil)

	if err == nil {
		t.Fatal("err = nil, want an error for the redirect")
	}
	for _, want := range []string{"/acme/login", "not the RAYNET CRM REST API"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
	if hits != 1 {
		t.Errorf("server hits = %d, want 1: the redirect must not be followed", hits)
	}
}
