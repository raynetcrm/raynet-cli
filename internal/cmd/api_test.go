/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// apiBody runs `raynet api` with args against a POST route and returns the
// JSON body the fake API received, re-encoded canonically (sorted keys).
func apiBody(t *testing.T, h *harness, args ...string) string {
	t.Helper()
	h.api.reply("POST", "/company/5/", 200, `{"success":true}`)
	res := h.run(append([]string{"api", "/company/5/", "-X", "POST"}, args...)...)
	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	out, err := json.Marshal(h.api.lastRequest().json(t))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out)
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAPI_RawFieldsAreStrings(t *testing.T) {
	h := newHarness(t)
	got := apiBody(t, h, "-f", "rating=1", "-f", "lock=true", "-f", "note=null")
	if want := `{"lock":"true","note":"null","rating":"1"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestAPI_TypedFields(t *testing.T) {
	h := newHarness(t)
	file := writeTemp(t, "note.txt", "line one\nline two")
	got := apiBody(t, h,
		"-F", "yes=true", "-F", "no=false", "-F", "nothing=null",
		"-F", "count=3", "-F", "price=12.5", "-F", "name=Acme",
		"-F", "note=@"+file)
	want := `{"count":3,"name":"Acme","no":false,"note":"line one\nline two","nothing":null,"price":12.5,"yes":true}`
	if got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestAPI_TypedFieldMissingFile(t *testing.T) {
	h := newHarness(t)
	res := h.run("api", "/company/", "-X", "PUT", "-F", "note=@/nonexistent/file")
	if res.err == nil {
		t.Fatal("expected an error for a missing @file")
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("requests = %d, want none", n)
	}
}

func TestAPI_BracketKeys(t *testing.T) {
	h := newHarness(t)
	got := apiBody(t, h,
		"-f", "name=Acme",
		"-F", "addresses[][address][city]=Brno",
		"-F", "addresses[][address][zip]=60200",
		"-F", "owner[id]=7",
		"-f", "tags[]=vip")
	want := `{"addresses":[{"address":{"city":"Brno","zip":60200}}],"name":"Acme","owner":{"id":7},"tags":["vip"]}`
	if got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestAPI_InputFileWithFlagsOnTop(t *testing.T) {
	h := newHarness(t)
	file := writeTemp(t, "body.json", `{"name":"Old","rating":"A","owner":{"id":1,"x":true}}`)
	got := apiBody(t, h, "--input", file, "-f", "name=New", "-F", "owner[id]=2")
	if want := `{"name":"New","owner":{"id":2,"x":true},"rating":"A"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestAPI_InputFromStdin(t *testing.T) {
	h := newHarness(t)
	h.stdin = `[{"id":1},{"id":2}]`
	got := apiBody(t, h, "--input", "-")
	if want := `[{"id":1},{"id":2}]`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestAPI_InputMustBeObjectWhenFieldsGiven(t *testing.T) {
	h := newHarness(t)
	h.stdin = `[1]`
	res := h.run("api", "/company/", "-X", "PUT", "--input", "-", "-f", "a=b")
	if res.err == nil {
		t.Fatal("expected an error combining a non-object --input with fields")
	}
}

func TestAPI_InvalidInputJSON(t *testing.T) {
	h := newHarness(t)
	h.stdin = `{not json`
	res := h.run("api", "/company/", "-X", "PUT", "--input", "-")
	if res.err == nil {
		t.Fatal("expected an error for invalid --input JSON")
	}
}

func TestAPI_DeleteSendsFieldsAsQueryAndInputAsBody(t *testing.T) {
	h := newHarness(t)
	h.api.reply("DELETE", "/offer/1/item/", 200, `{"success":true}`)
	h.stdin = `{"ids":[3,4]}`

	res := h.run("api", "/offer/1/item/", "-X", "DELETE", "-F", "force=true", "--input", "-")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.query.Get("force") != "true" {
		t.Errorf("query = %v, want force=true", req.query)
	}
	if string(req.body) != `{"ids":[3,4]}` {
		t.Errorf("body = %s, want the --input body", req.body)
	}
}

func TestAPI_GetSendsTypedFieldsAndBracketKeysAsQuery(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/", 200, `{"data":[]}`)

	res := h.run("api", "/company/", "-F", "limit=2", "-f", "name[LIKE]=Acme%")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.query.Get("limit") != "2" || req.query.Get("name[LIKE]") != "Acme%" {
		t.Errorf("query = %v, want limit=2 name[LIKE]=Acme%%", req.query)
	}
	if len(req.body) != 0 {
		t.Errorf("body = %q, want none", req.body)
	}
}

func TestAPI_FieldWithoutEquals(t *testing.T) {
	h := newHarness(t)
	res := h.run("api", "/company/", "-f", "noequalsign")
	if res.err == nil {
		t.Fatal("expected an error for a field without '='")
	}
}
