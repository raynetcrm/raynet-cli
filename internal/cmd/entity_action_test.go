/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestEntityAction_SendsMethodAndPath(t *testing.T) {
	tests := []struct {
		action     string
		wantMethod string
		wantSuffix string
	}{
		{"lock", "POST", "lock"},
		{"unlock", "POST", "unlock"},
		{"invalidate", "POST", "invalid"},
		{"revalidate", "POST", "valid"},
	}
	for _, entity := range []string{"company", "person"} {
		for _, tt := range tests {
			t.Run(entity+" "+tt.action, func(t *testing.T) {
				h := newHarness(t)
				wantPath := "/" + entity + "/42/" + tt.wantSuffix
				h.api.reply(tt.wantMethod, wantPath, 200, `{"success":true}`)

				res := h.run(entity, tt.action, "42")

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				req := h.api.lastRequest()
				if req.method != tt.wantMethod || req.path != wantPath {
					t.Errorf("request = %s %s, want %s %s", req.method, req.path, tt.wantMethod, wantPath)
				}
				if req.json(t) != nil {
					t.Errorf("body = %s, want none", req.body)
				}
				if res.stdout != "{\n  \"success\": true\n}\n" {
					t.Errorf("stdout = %q, want the response JSON", res.stdout)
				}
				if res.stderr != "" {
					t.Errorf("stderr = %q, want nothing outside a terminal", res.stderr)
				}
			})
		}
	}
}

func TestEntityAction_TagSendsJoinedNames(t *testing.T) {
	tests := []struct {
		action, method string
	}{
		{"tag", "PUT"},
		{"untag", "DELETE"},
	}
	for _, entity := range []string{"company", "person"} {
		for _, tt := range tests {
			t.Run(entity+" "+tt.action, func(t *testing.T) {
				h := newHarness(t)
				wantPath := "/" + entity + "/42/tag/"
				h.api.reply(tt.method, wantPath, 200, `{"success":true}`)

				res := h.run(entity, tt.action, "42", "vip", "hot")

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				req := h.api.lastRequest()
				if req.method != tt.method || req.path != wantPath {
					t.Errorf("request = %s %s, want %s %s", req.method, req.path, tt.method, wantPath)
				}
				if got, want := req.json(t), map[string]any{"tag": "vip,hot"}; !reflect.DeepEqual(got, want) {
					t.Errorf("body = %v, want %v", got, want)
				}
			})
		}
	}
}

func TestEntityAction_TagRequiresTagNames(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "tag", "42")

	if res.err == nil {
		t.Fatal("err = nil, want an argument error")
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("fake API received %d requests, want none", n)
	}
}

func TestEntityAction_ResolvesExternalID(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/ext/erp-17/", 200, `{"success":true,"data":{"id":42}}`)
	h.api.reply("POST", "/company/42/lock", 200, `{"success":true}`)

	res := h.run("company", "lock", "ext:erp-17")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "POST" || req.path != "/company/42/lock" {
		t.Errorf("request = %s %s, want POST /company/42/lock", req.method, req.path)
	}
}

func TestEntityAction_TerminalConfirmsOnStderr(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.api.reply("PUT", "/person/42/tag/", 201, `{"success":true}`)

	res := h.run("person", "tag", "42", "vip")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stderr != "Tagged person 42\n" {
		t.Errorf("stderr = %q, want %q", res.stderr, "Tagged person 42\n")
	}
}

func TestEntityAction_EmptyResponsePrintsNothing(t *testing.T) {
	h := newHarness(t)
	h.api.reply("POST", "/company/42/unlock", 200, ``)

	res := h.run("company", "unlock", "42")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want nothing", res.stdout)
	}
}

func TestEntityAction_WithBodyAcceptsBodyFlags(t *testing.T) {
	withEntityRow(t, entitySpec{
		use: "deal", short: "test record", path: "deal", verbs: verbView,
		actions: []action{{use: "convert", short: "Convert a %s", done: "Converted", method: "POST", suffix: "convert/", body: true}},
	})
	h := newHarness(t)
	h.stdin = `{"companyName":"Acme","owner":1}`
	h.api.reply("POST", "/deal/42/convert/", 200, `{"success":true}`)

	res := h.run("deal", "convert", "42", "--input", "-", "-F", "owner=7", "-f", "note=x")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "POST" || req.path != "/deal/42/convert/" {
		t.Errorf("request = %s %s, want POST /deal/42/convert/", req.method, req.path)
	}
	want := map[string]any{"companyName": "Acme", "owner": float64(7), "note": "x"}
	if got := req.json(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestEntityAction_WithoutBodyRejectsBodyFlags(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "lock", "42", "-f", "x=y")

	if res.err == nil {
		t.Fatal("err = nil, want an unknown flag error")
	}
}

func TestEntityAction_OnlyRowActionsAreBuilt(t *testing.T) {
	withEntityRow(t, entitySpec{
		use: "thing", short: "test record", path: "thing", verbs: verbsCRUD,
		actions: []action{actLock, actUnlock},
	})
	h := newHarness(t)

	res := h.run("thing", "--help")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	var got []string
	for _, line := range strings.Split(res.stdout, "\n") {
		if fields := strings.Fields(line); strings.HasPrefix(line, "  ") && len(fields) > 0 {
			got = append(got, fields[0])
		}
	}
	for _, name := range []string{"lock", "unlock"} {
		if !slices.Contains(got, name) {
			t.Errorf("thing help lists %v, missing %s", got, name)
		}
	}
	for _, name := range []string{"invalidate", "revalidate", "tag", "untag"} {
		if slices.Contains(got, name) {
			t.Errorf("thing help lists %v, want no %s", got, name)
		}
	}
}
