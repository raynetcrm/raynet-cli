/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/raynetcrm/raynet-cli/internal/config"
)

func TestAPI_GetSendsFieldsAsQuery(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/", 200, `{"data":[]}`)

	res := h.run("api", "company/", "-f", "fulltext=acme", "-f", "limit=2")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "GET" || req.path != "/company/" {
		t.Errorf("request = %s %s, want GET /company/", req.method, req.path)
	}
	if req.query.Get("fulltext") != "acme" || req.query.Get("limit") != "2" {
		t.Errorf("query = %v, want fulltext=acme limit=2", req.query)
	}
	if len(req.body) != 0 {
		t.Errorf("body = %q, want none", req.body)
	}
	if res.stdout != "{\n  \"data\": []\n}\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
}

func TestAPI_PostSendsFieldsAsJSONBody(t *testing.T) {
	h := newHarness(t)
	h.api.reply("POST", "/company/5/", 200, `{"success":true}`)

	res := h.run("api", "/company/5/", "-X", "post", "-f", "name=RAYNET s.r.o.", "-f", "rating=A")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "POST" || req.path != "/company/5/" {
		t.Errorf("request = %s %s, want POST /company/5/", req.method, req.path)
	}
	want := map[string]any{"name": "RAYNET s.r.o.", "rating": "A"}
	if got := req.json(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	if got := req.header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestAPI_PaginateWalksPages(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/person/", 200, `{"data":[{"id":1},{"id":2}]}`)

	res := h.run("api", "/person/", "--paginate")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if q := h.api.lastRequest().query; q.Get("offset") != "0" || q.Get("limit") != "1000" {
		t.Errorf("query = %v, want offset=0 limit=1000", q)
	}
	want := "[\n  {\n    \"id\": 1\n  },\n  {\n    \"id\": 2\n  }\n]\n"
	if res.stdout != want {
		t.Errorf("stdout = %q, want %q", res.stdout, want)
	}
}

func TestAPI_PaginateRejectsNonGet(t *testing.T) {
	h := newHarness(t)

	res := h.run("api", "/person/", "--paginate", "-X", "DELETE")

	if res.err == nil || !strings.Contains(res.err.Error(), "--paginate") {
		t.Errorf("err = %v, want a --paginate error", res.err)
	}
	if len(h.api.requests()) != 0 {
		t.Errorf("requests = %d, want none", len(h.api.requests()))
	}
}

func TestSecurityCommandIsGone(t *testing.T) {
	h := newHarness(t)

	res := h.run("security", "info")

	if res.err == nil || !strings.Contains(res.err.Error(), "unknown command") {
		t.Errorf("err = %v, want unknown command", res.err)
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("requests = %d, want none", n)
	}
}

func TestUserAccountList_CallsUserAccountWithDefaultColumns(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/userAccount/", 200, `{"success":"true","totalCount":1,"data":[
		{"id":3,"username":"alice@example.com","userRole":"ADMIN","person":{"id":17,"fullName":"Alice Smith"}}]}`)

	res := h.run("user-account", "list")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "GET" || req.path != "/userAccount/" {
		t.Errorf("request = %s %s, want GET /userAccount/", req.method, req.path)
	}
	want := "id\tusername\tperson.fullName\tperson.id\tuserRole\n" +
		"3\talice@example.com\tAlice Smith\t17\tADMIN\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
}

func TestUserAccountView_CallsUserAccountDetail(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/userAccount/3/", 200, `{"success":"true","data":{"id":3}}`)

	res := h.run("user-account", "view", "3")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "GET" || req.path != "/userAccount/3/" {
		t.Errorf("request = %s %s, want GET /userAccount/3/", req.method, req.path)
	}
}

func TestUserAccountMe_ShowsSecurityInfo(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/info", 200, `{"username":"alice"}`)

	res := h.run("user-account", "me")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "GET" || req.path != "/security/info" {
		t.Errorf("request = %s %s, want GET /security/info", req.method, req.path)
	}
	if res.stdout != "{\n  \"username\": \"alice\"\n}\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
}

func TestAuthToken_PrintsMintedToken(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/bearertoken", 200, `{"token":"abc"}`)

	res := h.runAPIKey("auth", "token")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "GET" || req.path != "/security/bearertoken" {
		t.Errorf("request = %s %s, want GET /security/bearertoken", req.method, req.path)
	}
	if user, key, ok := (&http.Request{Header: req.header}).BasicAuth(); !ok || user != "alice" || key != "key" {
		t.Errorf("basic auth = %q/%q/%v, want alice/key", user, key, ok)
	}
	if res.stdout != "abc\n" {
		t.Errorf("stdout = %q, want the bare token", res.stdout)
	}
}

func TestAuthToken_JSONPrintsResponse(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/security/bearertoken", 200, `{"token":"abc"}`)

	res := h.runAPIKey("auth", "token", "--json")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stdout != "{\n  \"token\": \"abc\"\n}\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
}

func TestAuthToken_RefusesBearerCredentials(t *testing.T) {
	h := newHarness(t)

	res := h.run("auth", "token")

	if res.err == nil || !strings.Contains(res.err.Error(), "API key") {
		t.Errorf("err = %v, want an API-key-required error", res.err)
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("requests = %d, want none", n)
	}
}

func TestStoredProfile_UsedWithoutAdHocCredentials(t *testing.T) {
	h := newHarness(t)
	h.storeProfile("dev", h.api.profile())
	h.api.reply("GET", "/company/3/", 200, `{"data":{"id":3}}`)

	res := h.runRaw("company", "view", "3")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if got := h.api.lastRequest().header.Get("Authorization"); got != "Bearer profile-token" {
		t.Errorf("Authorization = %q, want the stored profile's token", got)
	}
}

func TestAuthStatus_ListsProfiles(t *testing.T) {
	h := newHarness(t)
	h.storeProfile("dev", config.Profile{Host: "localhost:8082", AuthType: config.AuthBearer, Token: "t"})

	res := h.runRaw("auth", "status")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stdout != "dev (default) — host: localhost:8082, auth: bearer\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
}

func TestAuthConfig_PrintsConfigPath(t *testing.T) {
	h := newHarness(t)

	res := h.runRaw("auth", "config")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stdout != h.configPath+"\n" {
		t.Errorf("stdout = %q, want %q", res.stdout, h.configPath+"\n")
	}
}

func TestHelp_GoesToStdout(t *testing.T) {
	h := newHarness(t)

	res := h.runRaw("company", "--help")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if !strings.Contains(res.stdout, "Work with companies") || !strings.Contains(res.stdout, "list") {
		t.Errorf("stdout = %q, want the company help", res.stdout)
	}
}

func TestUnknownSubcommand_Fails(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"activity", "view", "42"}, `unknown command "view" for "raynet activity"`},
		{[]string{"company", "frobnicate"}, `unknown command "frobnicate" for "raynet company"`},
		{[]string{"company", "address", "frobnicate", "1"}, `unknown command "frobnicate" for "raynet company address"`},
		{[]string{"auth", "frobnicate"}, `unknown command "frobnicate" for "raynet auth"`},
		{[]string{"codelist", "frobnicate"}, `unknown command "frobnicate" for "raynet codelist"`},
		{[]string{"codelist", "businessCaseType", "create", "--name", "A"}, `unknown command "create" for "raynet codelist businessCaseType"`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			h := newHarness(t)

			res := h.run(tt.args...)

			if res.err == nil || !strings.Contains(res.err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", res.err, tt.want)
			}
			if n := len(h.api.requests()); n != 0 {
				t.Errorf("requests = %d, want none", n)
			}
		})
	}
}

func TestBareGroupCommand_ShowsHelp(t *testing.T) {
	for _, args := range [][]string{{"company"}, {"company", "address"}, {"auth"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := newHarness(t)

			res := h.runRaw(args...)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if !strings.Contains(res.stdout, "Available Commands:") {
				t.Errorf("stdout = %q, want the help", res.stdout)
			}
		})
	}
}

func TestUserAccount_TakesAPINameAsAliasButNotUser(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/userAccount/", 200, `{"success":"true","totalCount":0,"data":[]}`)

	if res := h.run("userAccount", "list"); res.err != nil {
		t.Errorf("userAccount list: err = %v", res.err)
	}
	res := h.run("user", "list")

	if res.err == nil || !strings.Contains(res.err.Error(), `unknown command "user"`) {
		t.Errorf("err = %v, want unknown command", res.err)
	}
}
