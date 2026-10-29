/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"reflect"
	"testing"
)

func TestViewWeb_LearnsInstanceNameAndOpensRecordOnAPIHost(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, `{"username":"alice","X-Instance-Name":"acme"}`)

	res := h.run("company", "view", "42", "--web")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := []string{h.api.hostFlag() + "/acme/redir?en=Company&ei=42"}
	if !reflect.DeepEqual(res.browserURLs, want) {
		t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
	}
	for _, r := range h.api.requests() {
		if r.path == "/company/42/" {
			t.Errorf("--web fetched the record: %s %s", r.method, r.path)
		}
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want nothing", res.stdout)
	}
}

func TestViewWeb_UsesProfileWebURLAndStoredInstanceName(t *testing.T) {
	h := newHarness(t)
	p := h.api.profile()
	p.WebURL = "http://localhost:3000/"
	p.InstanceName = "acme"
	h.storeProfile("dev", p)

	res := h.runRaw("person", "view", "7", "--web")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := []string{"http://localhost:3000/acme/redir?en=Person&ei=7"}
	if !reflect.DeepEqual(res.browserURLs, want) {
		t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("requests = %d, want none (instance name and web URL are stored)", n)
	}
}

func TestViewWeb_StoredInstanceNameWithoutWebURLUsesAPIHost(t *testing.T) {
	h := newHarness(t)
	p := h.api.profile()
	p.InstanceName = "acme"
	h.storeProfile("prod", p)

	res := h.runRaw("company", "view", "42", "--web")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := []string{h.api.hostFlag() + "/acme/redir?en=Company&ei=42"}
	if !reflect.DeepEqual(res.browserURLs, want) {
		t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("requests = %d, want none", n)
	}
}

func TestViewWeb_ResolvesExternalID(t *testing.T) {
	h := newHarness(t)
	p := h.api.profile()
	p.InstanceName = "acme"
	h.storeProfile("prod", p)
	h.api.reply("GET", "/company/ext/erp-17/", 200, `{"success":true,"data":{"id":42,"name":"Acme"}}`)

	res := h.runRaw("company", "view", "ext:erp-17", "--web")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := []string{h.api.hostFlag() + "/acme/redir?en=Company&ei=42"}
	if !reflect.DeepEqual(res.browserURLs, want) {
		t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
	}
}

func TestListWeb_OpensListView(t *testing.T) {
	h := newHarness(t)
	p := h.api.profile()
	p.WebURL = "http://localhost:3000"
	p.InstanceName = "acme"
	h.storeProfile("dev", p)

	res := h.runRaw("person", "list", "--web")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := []string{"http://localhost:3000/acme/?view=ListView&en=Person"}
	if !reflect.DeepEqual(res.browserURLs, want) {
		t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("requests = %d, want none", n)
	}
}

func TestListWeb_LearnsInstanceName(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, `{"X-Instance-Name":"acme"}`)

	res := h.run("company", "list", "--web")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := []string{h.api.hostFlag() + "/acme/?view=ListView&en=Company"}
	if !reflect.DeepEqual(res.browserURLs, want) {
		t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
	}
}

func TestAuthLogin_StoresWebURLAndLearnedInstanceName(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, `{"username":"alice","X-Instance-Name":"acme","X-Instance-Id":"abc123"}`)

	res := h.runRaw("auth", "login", "--profile", "dev", "--host", h.api.hostFlag(),
		"--auth-type", "bearer", "--token", "tok-1", "--web-url", "http://localhost:3000")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	got := h.loadConfig().Profiles["dev"]
	if got.WebURL != "http://localhost:3000" || got.WebInstanceName != "acme" {
		t.Errorf("profile = %+v, want web URL http://localhost:3000 and instance name acme", got)
	}
}

func TestAuthLogin_InstanceIDProfileLearnsInstanceName(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, `{"username":"alice","X-Instance-Name":"acme","X-Instance-Id":"abc123"}`)

	res := h.runRaw("auth", "login", "--profile", "acme", "--host", h.api.hostFlag(),
		"--auth-type", "apikey", "--username", "alice", "--api-key", "k", "--instance-id", "abc123")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if got := h.api.lastRequest().header.Get("X-Instance-Id"); got != "abc123" {
		t.Errorf("verification X-Instance-Id = %q, want abc123", got)
	}
	got := h.loadConfig().Profiles["acme"]
	if got.WebInstanceName != "acme" || got.InstanceName != "" || got.InstanceID != "abc123" || got.WebURL != "" {
		t.Errorf("profile = %+v, want web instance name acme learned, id kept, no web URL", got)
	}

	h.api.reply("GET", "/company/3/", 200, `{"data":{"id":3}}`)
	res = h.runRaw("company", "view", "3")
	if res.err != nil {
		t.Fatalf("view err = %v", res.err)
	}
	hdr := h.api.lastRequest().header
	if hdr.Get("X-Instance-Id") != "abc123" || hdr.Get("X-Instance-Name") != "" {
		t.Errorf("after login X-Instance-Id = %q, X-Instance-Name = %q, want abc123 and none",
			hdr.Get("X-Instance-Id"), hdr.Get("X-Instance-Name"))
	}

	before := len(h.api.requests())
	res = h.runRaw("company", "view", "3", "--web")
	if res.err != nil {
		t.Fatalf("view --web err = %v", res.err)
	}
	if want := []string{h.api.hostFlag() + "/acme/redir?en=Company&ei=3"}; !reflect.DeepEqual(res.browserURLs, want) {
		t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
	}
	if n := len(h.api.requests()) - before; n != 0 {
		t.Errorf("--web made %d requests, want none (instance name is stored)", n)
	}
}
