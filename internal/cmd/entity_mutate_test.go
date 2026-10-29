/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestEntityCreate_PutsBodyAndPrintsResponse(t *testing.T) {
	for _, entity := range []string{"company", "person"} {
		t.Run(entity, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("PUT", "/"+entity+"/", 201, `{"success":true,"data":{"id":123}}`)

			res := h.run(entity, "create", "-f", "name=Acme", "-F", "owner=7", "-F", "addresses[][address][city]=Brno")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			req := h.api.lastRequest()
			if req.method != "PUT" || req.path != "/"+entity+"/" {
				t.Errorf("request = %s %s, want PUT /%s/", req.method, req.path, entity)
			}
			want := map[string]any{
				"name":      "Acme",
				"owner":     float64(7),
				"addresses": []any{map[string]any{"address": map[string]any{"city": "Brno"}}},
			}
			if got := req.json(t); !reflect.DeepEqual(got, want) {
				t.Errorf("body = %v, want %v", got, want)
			}
			wantOut := "{\n  \"success\": true,\n  \"data\": {\n    \"id\": 123\n  }\n}\n"
			if res.stdout != wantOut {
				t.Errorf("stdout =\n%s\nwant\n%s", res.stdout, wantOut)
			}
			if res.stderr != "" {
				t.Errorf("stderr = %q, want nothing outside a terminal", res.stderr)
			}
		})
	}
}

func TestEntityCreate_TerminalConfirmsOnStderr(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.api.reply("PUT", "/company/", 201, `{"success":true,"data":{"id":123}}`)

	res := h.run("company", "create", "-f", "name=Acme")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stderr != "Created company 123\n" {
		t.Errorf("stderr = %q, want %q", res.stderr, "Created company 123\n")
	}
	if !strings.Contains(res.stdout, "123") {
		t.Errorf("stdout = %q, want the response JSON", res.stdout)
	}
}

// withEntityRow adds s to the entity spec table for the duration of the test.
func withEntityRow(t *testing.T, s entitySpec) {
	t.Helper()
	saved := entityTable
	entityTable = append(append([]entitySpec(nil), saved...), s)
	t.Cleanup(func() { entityTable = saved })
}

func TestEntityCreate_WithItemsSwitchesEndpoint(t *testing.T) {
	withEntityRow(t, entitySpec{use: "doc", short: "test document", path: "doc", verbs: verbsCRUD, withItems: true})
	withEntityRow(t, entitySpec{use: "plain", short: "test record", path: "plain", verbs: verbsCRUD})

	tests := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{"row allows it and body has items", []string{"doc", "create", "-f", "name=X", "-f", "items[][name]=Widget"}, "/doc/createWithItems"},
		{"row allows it but no items", []string{"doc", "create", "-f", "name=X"}, "/doc/"},
		{"row doesn't allow it", []string{"plain", "create", "-f", "items[][name]=Widget"}, "/plain/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("PUT", tt.wantPath, 201, `{"success":true,"data":{"id":1}}`)

			res := h.run(tt.args...)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if req := h.api.lastRequest(); req.method != "PUT" || req.path != tt.wantPath {
				t.Errorf("request = %s %s, want PUT %s", req.method, req.path, tt.wantPath)
			}
		})
	}
}

func TestEntityUpdate_PostsBodyToRecord(t *testing.T) {
	for _, entity := range []string{"company", "person"} {
		t.Run(entity, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("POST", "/"+entity+"/42/", 200, `{"success":true}`)

			res := h.run(entity, "update", "42", "-f", "notice=hello", "-F", "keyman=true")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			req := h.api.lastRequest()
			if req.method != "POST" || req.path != "/"+entity+"/42/" {
				t.Errorf("request = %s %s, want POST /%s/42/", req.method, req.path, entity)
			}
			if got, want := req.json(t), map[string]any{"notice": "hello", "keyman": true}; !reflect.DeepEqual(got, want) {
				t.Errorf("body = %v, want %v", got, want)
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

func TestEntityUpdate_TerminalConfirmsOnStderr(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.api.reply("POST", "/company/42/", 200, `{"success":true}`)

	res := h.run("company", "update", "42", "-f", "notice=hello")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stderr != "Updated company 42\n" {
		t.Errorf("stderr = %q, want %q", res.stderr, "Updated company 42\n")
	}
}

func TestEntityUpdate_ExternalIDIsResolvedFirst(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/ext/erp-17/", 200, `{"success":true,"data":{"id":42,"name":"Acme"}}`)
	h.api.reply("POST", "/company/42/", 200, `{"success":true}`)

	res := h.run("company", "update", "ext:erp-17", "-f", "notice=hello")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	var got []string
	for _, r := range h.api.requests() {
		got = append(got, r.method+" "+r.path)
	}
	if want := []string{"GET /company/ext/erp-17/", "POST /company/42/"}; !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

func TestEntityUpdate_UnknownExternalIDFailsBeforeMutation(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "update", "ext:nope", "-f", "notice=hello")

	if res.err == nil || !strings.Contains(res.err.Error(), "404") {
		t.Errorf("err = %v, want the lookup's 404", res.err)
	}
	if n := len(h.api.requests()); n != 1 {
		t.Errorf("requests = %d, want only the lookup", n)
	}
}

func TestEntityDelete_NonTerminalWithoutYesRefuses(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "delete", "42")

	if res.err == nil || !strings.Contains(res.err.Error(), "--yes") {
		t.Errorf("err = %v, want a refusal mentioning --yes", res.err)
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("requests = %d, want none", n)
	}
}

func TestEntityDelete_YesDeletes(t *testing.T) {
	for _, entity := range []string{"company", "person"} {
		t.Run(entity, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("DELETE", "/"+entity+"/42/", 200, `{"success":true}`)

			res := h.run(entity, "delete", "42", "--yes")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			req := h.api.lastRequest()
			if req.method != "DELETE" || req.path != "/"+entity+"/42/" {
				t.Errorf("request = %s %s, want DELETE /%s/42/", req.method, req.path, entity)
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

func TestEntityDelete_ExternalIDIsResolvedFirst(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/person/ext/crm-9/", 200, `{"success":true,"data":{"id":42}}`)
	h.api.reply("DELETE", "/person/42/", 200, `{"success":true}`)

	res := h.run("person", "delete", "ext:crm-9", "--yes")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	var got []string
	for _, r := range h.api.requests() {
		got = append(got, r.method+" "+r.path)
	}
	if want := []string{"GET /person/ext/crm-9/", "DELETE /person/42/"}; !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

func TestEntityDelete_TerminalPrompts(t *testing.T) {
	tests := []struct {
		answer     string
		wantDelete bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"n\n", false},
		{"\n", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(strings.TrimSpace(tt.answer), func(t *testing.T) {
			h := newHarness(t)
			h.terminal = true
			h.stdin = tt.answer
			h.api.reply("DELETE", "/company/42/", 200, `{"success":true}`)

			res := h.run("company", "delete", "42")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if !strings.HasPrefix(res.stderr, "Delete company 42? [y/N] ") {
				t.Errorf("stderr = %q, want the prompt first", res.stderr)
			}
			deleted := len(h.api.requests()) == 1
			if deleted != tt.wantDelete {
				t.Errorf("deleted = %v, want %v", deleted, tt.wantDelete)
			}
			if tt.wantDelete && !strings.HasSuffix(res.stderr, "Deleted company 42\n") {
				t.Errorf("stderr = %q, want a confirmation", res.stderr)
			}
		})
	}
}

func TestEntityCreateUpdateHelp_ListsFieldReference(t *testing.T) {
	tests := []struct {
		entity string
		lines  []string // regexps, each must match a whole help line
	}{
		{"company", []string{
			`^  name +string +required +company name$`,
			`^  state +string +required +lifecycle state; values: A_POTENTIAL, B_ACTUAL, C_DEFERRED, D_UNATTRACTIVE$`,
			`^  owner +id +owner \(user's person id\); points to: user$`,
			`^  category +id +category; codelist: companyCategory$`,
			`^  birthday +date +birthday or anniversary, yyyy-MM-dd$`,
		}},
		{"person", []string{
			`^  lastName +string +required +last name$`,
			`^  gender +string +gender; values: MALE, FEMALE$`,
			`^  language +id +language; codelist: language$`,
			`^  keyman +bool +key person$`,
		}},
	}
	for _, tt := range tests {
		for _, verb := range []string{"create", "update"} {
			t.Run(tt.entity+" "+verb, func(t *testing.T) {
				h := newHarness(t)

				res := h.run(tt.entity, verb, "--help")

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				if !strings.Contains(res.stdout, "\nFields") {
					t.Errorf("help has no Fields section:\n%s", res.stdout)
				}
				for _, pattern := range tt.lines {
					if !regexp.MustCompile("(?m)" + pattern).MatchString(res.stdout) {
						t.Errorf("help has no line matching %s:\n%s", pattern, res.stdout)
					}
				}
				if len(h.api.requests()) != 0 {
					t.Errorf("help called the API")
				}
			})
		}
	}
}

func TestEntityCreateUpdateHelp_HasExamples(t *testing.T) {
	h := newHarness(t)

	create := h.run("company", "create", "--help")
	update := h.run("company", "update", "--help")

	if want := "raynet company create -f 'name=Acme Ltd' -f state=A_POTENTIAL -f rating=A -f role=A_SUBSCRIBER"; !strings.Contains(create.stdout, want) {
		t.Errorf("create help has no %q example:\n%s", want, create.stdout)
	}
	if want := "raynet company update ext:erp-17"; !strings.Contains(update.stdout, want) {
		t.Errorf("update help has no %q example:\n%s", want, update.stdout)
	}
}
