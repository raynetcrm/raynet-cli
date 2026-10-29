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

// activityTypes are the per-type activity commands and the REST path segment
// each one calls.
var activityTypes = []struct{ use, path string }{
	{"task", "task"},
	{"meeting", "meeting"},
	{"event", "event"},
	{"email", "email"},
	{"phone-call", "phoneCall"},
	{"letter", "letter"},
}

func TestActivityType_CRUDAndActionsCallTheTypesOwnPath(t *testing.T) {
	type call struct {
		args         []string
		method, path string
		body         any
	}
	for _, at := range activityTypes {
		p := "/" + at.path + "/"
		calls := []call{
			{[]string{"list"}, "GET", p, nil},
			{[]string{"view", "42"}, "GET", p + "42/", nil},
			{[]string{"create", "-f", "title=Call back"}, "PUT", p, map[string]any{"title": "Call back"}},
			{[]string{"update", "42", "-f", "status=COMPLETED"}, "POST", p + "42/", map[string]any{"status": "COMPLETED"}},
			{[]string{"delete", "42", "--yes"}, "DELETE", p + "42/", nil},
			{[]string{"lock", "42"}, "POST", p + "42/lock", nil},
			{[]string{"unlock", "42"}, "POST", p + "42/unlock", nil},
			{[]string{"tag", "42", "vip", "hot"}, "PUT", p + "42/tag/", map[string]any{"tag": "vip,hot"}},
			{[]string{"untag", "42", "vip"}, "DELETE", p + "42/tag/", map[string]any{"tag": "vip"}},
		}
		for _, c := range calls {
			t.Run(at.use+" "+c.args[0], func(t *testing.T) {
				h := newHarness(t)
				h.api.reply(c.method, c.path, 200, `{"success":true,"data":[]}`)

				res := h.run(append([]string{at.use}, c.args...)...)

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				req := h.api.lastRequest()
				if req.method != c.method || req.path != c.path {
					t.Errorf("request = %s %s, want %s %s", req.method, req.path, c.method, c.path)
				}
				if got := req.json(t); !reflect.DeepEqual(got, c.body) {
					t.Errorf("body = %v, want %v", got, c.body)
				}
			})
		}
	}
}

func TestActivityType_ViewByExternalID(t *testing.T) {
	for _, at := range activityTypes {
		t.Run(at.use, func(t *testing.T) {
			h := newHarness(t)
			want := "/" + at.path + "/ext/erp-17/"
			h.api.reply("GET", want, 200, `{"data":{"id":42}}`)

			res := h.run(at.use, "view", "ext:erp-17")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if req := h.api.lastRequest(); req.path != want {
				t.Errorf("path = %s, want %s", req.path, want)
			}
		})
	}
}

func TestPhoneCall_Aliases(t *testing.T) {
	for _, alias := range []string{"phoneCall", "pc"} {
		t.Run(alias, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("POST", "/phoneCall/42/lock", 200, `{"success":true}`)

			res := h.run(alias, "lock", "42")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if req := h.api.lastRequest(); req.path != "/phoneCall/42/lock" {
				t.Errorf("path = %s, want /phoneCall/42/lock", req.path)
			}
		})
	}
}

func TestActivityList_ShowsTypeColumnFromEntityName(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/activity/", 200, `{"success":true,"totalCount":3,"data":[
		{"id":1,"_entityName":"Task","title":"Send offer","status":"NEW","priority":"DEFAULT","company":{"id":3,"name":"Acme"}},
		{"id":1,"_entityName":"PhoneCall","title":"Follow up","status":"COMPLETED","priority":"CRITICAL","scheduledFrom":"2026-10-01 09:00"},
		{"id":5,"_entityName":"Meeting","title":"Kickoff","status":"SCHEDULED","priority":"MINOR"}]}`)

	res := h.run("activity", "list")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "GET" || req.path != "/activity/" {
		t.Errorf("request = %s %s, want GET /activity/", req.method, req.path)
	}
	want := "id\ttype\ttitle\tstatus\tpriority\tscheduledFrom\tcompany.name\n" +
		"1\ttask\tSend offer\tNEW\tDEFAULT\t\tAcme\n" +
		"1\tphone-call\tFollow up\tCOMPLETED\tCRITICAL\t2026-10-01 09:00\t\n" +
		"5\tmeeting\tKickoff\tSCHEDULED\tMINOR\t\t\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
}

func TestActivityList_JSONKeepsAPIRecords(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/activity/", 200, `{"success":true,"totalCount":1,"data":[{"id":1,"_entityName":"Task"}]}`)

	res := h.run("activity", "list", "--json")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if want := "[\n  {\n    \"_entityName\": \"Task\",\n    \"id\": 1\n  }\n]\n"; res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
}

func TestActivityList_UnknownEntityNameShowsAsIs(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/activity/", 200, `{"success":true,"totalCount":1,"data":[{"id":1,"_entityName":"Visit"}]}`)

	res := h.run("activity", "list", "--columns", "id,type")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if want := "id\ttype\n1\tVisit\n"; res.stdout != want {
		t.Errorf("stdout = %q, want %q", res.stdout, want)
	}
}

func TestActivity_IsListOnly(t *testing.T) {
	h := newHarness(t)

	res := h.run("activity", "--help")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	got := subcommands(res.stdout)
	if !reflect.DeepEqual(got, []string{"list"}) {
		t.Errorf("activity subcommands = %v, want [list]", got)
	}
	for _, verb := range []string{"view", "create", "update", "delete", "lock"} {
		h.run("activity", verb, "42")
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("API requests = %d, want 0", n)
	}
}

func TestBusinessCaseHelp_ValidTillIsUpdateOnly(t *testing.T) {
	h := newHarness(t)

	create := h.run("business-case", "create", "--help")
	update := h.run("business-case", "update", "--help")

	line := regexp.MustCompile(`(?m)^  validTill +date +closing date, yyyy-MM-dd$`)
	if line.MatchString(create.stdout) || strings.Contains(create.stdout, "validTill") {
		t.Errorf("create help lists validTill:\n%s", create.stdout)
	}
	if !line.MatchString(update.stdout) {
		t.Errorf("update help has no validTill line:\n%s", update.stdout)
	}
}
