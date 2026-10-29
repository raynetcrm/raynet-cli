/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestEntityList_FetchesFirst30RecordsAndPrintsDefaultColumns(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/", 200, `{"success":true,"totalCount":319,"data":[
		{"id":1,"name":"Acme","regNumber":"123","state":"B_ACTUAL","rating":"A","owner":{"id":5,"fullName":"Jan Novák"}},
		{"id":2,"name":"Globex","state":"A_POTENTIAL","rating":"C"}]}`)

	res := h.run("company", "list")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "GET" || req.path != "/company/" {
		t.Errorf("request = %s %s, want GET /company/", req.method, req.path)
	}
	if want := (url.Values{"offset": {"0"}, "limit": {"30"}}); !reflect.DeepEqual(req.query, want) {
		t.Errorf("query = %v, want %v", req.query, want)
	}
	if len(h.api.requests()) != 1 {
		t.Errorf("requests = %d, want 1", len(h.api.requests()))
	}
	want := "id\tname\tregNumber\tstate\trating\towner.fullName\n" +
		"1\tAcme\t123\tB_ACTUAL\tA\tJan Novák\n" +
		"2\tGlobex\t\tA_POTENTIAL\tC\t\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
	if res.stderr != "" {
		t.Errorf("stderr = %q, want nothing outside a terminal", res.stderr)
	}
}

func TestPersonList_PrintsDefaultColumns(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/person/", 200, `{"success":true,"totalCount":1,"data":[
		{"id":7,"firstName":"Jan","lastName":"Novák","contactInfo":{"email":"jan@example.com"},
		 "primaryRelationship":{"company":{"id":3,"name":"Acme"}},"owner":{"id":1,"fullName":"Eva Malá"}}]}`)

	res := h.run("person", "list")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "GET" || req.path != "/person/" {
		t.Errorf("request = %s %s, want GET /person/", req.method, req.path)
	}
	want := "id\tfirstName\tlastName\tprimaryRelationship.company.name\tcontactInfo.email\towner.fullName\n" +
		"7\tJan\tNovák\tAcme\tjan@example.com\tEva Malá\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
}

func TestEntityList_MapsListFlagsToQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want url.Values
	}{
		{"offset", []string{"--offset", "60"}, url.Values{"offset": {"60"}, "limit": {"30"}}},
		{"fulltext", []string{"--fulltext", "acme"}, url.Values{"offset": {"0"}, "limit": {"30"}, "fulltext": {"acme"}}},
		{"filters", []string{"--filter", "rowInfo.createdAt[GT]=2014-06-01 00:00", "--filter", "state=A_POTENTIAL"},
			url.Values{"offset": {"0"}, "limit": {"30"}, "rowInfo.createdAt[GT]": {"2014-06-01 00:00"}, "state": {"A_POTENTIAL"}}},
		{"sort ascending", []string{"--sort", "name"},
			url.Values{"offset": {"0"}, "limit": {"30"}, "sortColumn": {"name"}, "sortDirection": {"ASC"}}},
		{"sort descending", []string{"--sort", "-name"},
			url.Values{"offset": {"0"}, "limit": {"30"}, "sortColumn": {"name"}, "sortDirection": {"DESC"}}},
	}
	for _, entity := range []string{"company", "person"} {
		for _, tt := range tests {
			t.Run(entity+"/"+tt.name, func(t *testing.T) {
				h := newHarness(t)
				h.api.reply("GET", "/"+entity+"/", 200, `{"success":true,"totalCount":0,"data":[]}`)

				res := h.run(append([]string{entity, "list"}, tt.args...)...)

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				if got := h.api.lastRequest().query; !reflect.DeepEqual(got, tt.want) {
					t.Errorf("query = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func TestEntityList_RejectsMalformedFilter(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "list", "--filter", "broken")

	if res.err == nil || !strings.Contains(res.err.Error(), "--filter") {
		t.Errorf("err = %v, want a --filter error", res.err)
	}
	if len(h.api.requests()) != 0 {
		t.Errorf("requests = %d, want none", len(h.api.requests()))
	}
}

func TestEntityList_AllWalksEveryPage(t *testing.T) {
	h := newHarness(t)
	h.api.handle("GET", "/company/", func(r apiRequest) (int, string) {
		if r.query.Get("offset") == "0" {
			rows := make([]string, 1000)
			for i := range rows {
				rows[i] = fmt.Sprintf(`{"id":%d}`, i+1)
			}
			return 200, `{"totalCount":1001,"data":[` + strings.Join(rows, ",") + `]}`
		}
		return 200, `{"totalCount":1001,"data":[{"id":1001}]}`
	})

	res := h.run("company", "list", "--all", "--fulltext", "acme", "--json")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	reqs := h.api.requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want two pages", len(reqs))
	}
	for i, wantOffset := range []string{"0", "1000"} {
		q := reqs[i].query
		if q.Get("offset") != wantOffset || q.Get("limit") != "1000" || q.Get("fulltext") != "acme" {
			t.Errorf("page %d query = %v, want offset=%s limit=1000 fulltext=acme", i, q, wantOffset)
		}
	}
	var records []map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &records); err != nil {
		t.Fatalf("stdout is not a JSON array: %v", err)
	}
	if len(records) != 1001 {
		t.Errorf("records = %d, want 1001", len(records))
	}
}

func TestEntityList_AllAndOffsetAreExclusive(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "list", "--all", "--offset", "30")

	if res.err == nil {
		t.Error("err = nil, want an error for --all with --offset")
	}
	if len(h.api.requests()) != 0 {
		t.Errorf("requests = %d, want none", len(h.api.requests()))
	}
}

func TestEntityList_TerminalPrintsTotalCountOnStderr(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := newHarness(t)
	h.terminal = true
	h.api.reply("GET", "/company/", 200, `{"success":true,"totalCount":319,"data":[{"id":1,"name":"Acme"},{"id":2,"name":"Globex"}]}`)

	res := h.run("company", "list")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stderr != "2 of 319\n" {
		t.Errorf("stderr = %q, want %q", res.stderr, "2 of 319\n")
	}
	if strings.Contains(res.stdout, "of 319") {
		t.Errorf("stdout = %q, want no total count", res.stdout)
	}
}

func TestEntityList_NonTerminalPrintsNoTotalCount(t *testing.T) {
	for _, args := range [][]string{{"company", "list"}, {"company", "list", "--json"}} {
		h := newHarness(t)
		h.api.reply("GET", "/company/", 200, `{"success":true,"totalCount":319,"data":[{"id":1,"name":"Acme"}]}`)

		res := h.run(args...)

		if res.err != nil {
			t.Fatalf("%v: err = %v", args, res.err)
		}
		if res.stderr != "" || strings.Contains(res.stdout, "319") {
			t.Errorf("%v: stdout = %q, stderr = %q, want no total count anywhere", args, res.stdout, res.stderr)
		}
	}
}

func TestEntityList_ColumnsSelectsDottedPaths(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/", 200, `{"data":[
		{"id":1,"owner":{"id":5,"fullName":"Jan Novák"},"rowInfo.createdAt":"2026-01-02 10:00","primaryAddress":{"address":{"city":"Brno"}}}]}`)

	res := h.run("company", "list", "--columns", "id,owner.fullName,rowInfo.createdAt,primaryAddress.address.city")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := "id\towner.fullName\trowInfo.createdAt\tprimaryAddress.address.city\n" +
		"1\tJan Novák\t2026-01-02 10:00\tBrno\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
}

func TestEntityList_JSONPrintsRecords(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/", 200, `{"success":true,"totalCount":1,"data":[{"id":1,"name":"Acme"}]}`)

	res := h.run("company", "list", "--json")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := "[\n  {\n    \"id\": 1,\n    \"name\": \"Acme\"\n  }\n]\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", res.stdout, want)
	}
}

func TestEntityList_EmptyResult(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/person/", 200, `{"success":true,"totalCount":0,"data":[]}`)

	res := h.run("person", "list")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stdout != "No records found.\n" {
		t.Errorf("stdout = %q, want %q", res.stdout, "No records found.\n")
	}
}

func TestEntityList_TerminalRendersStyledTable(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	h := newHarness(t)
	h.terminal = true
	h.api.reply("GET", "/company/", 200, `{"data":[{"id":1,"name":"Acme"}]}`)

	res := h.run("company", "list")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if !strings.Contains(res.stdout, "│") || !strings.Contains(res.stdout, "Acme") {
		t.Errorf("stdout = %q, want a bordered table containing Acme", res.stdout)
	}
	if strings.Contains(res.stdout, "\t") {
		t.Errorf("stdout = %q, want no plain tab-separated output in a terminal", res.stdout)
	}
}

func TestEntityView_PrintsRecordJSON(t *testing.T) {
	for _, entity := range []string{"company", "person"} {
		t.Run(entity, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("GET", "/"+entity+"/42/", 200, `{"success":true,"data":{"id":42,"name":"Acme"}}`)

			res := h.run(entity, "view", "42")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if req := h.api.lastRequest(); req.method != "GET" || req.path != "/"+entity+"/42/" {
				t.Errorf("request = %s %s, want GET /%s/42/", req.method, req.path, entity)
			}
			want := "{\n  \"success\": true,\n  \"data\": {\n    \"id\": 42,\n    \"name\": \"Acme\"\n  }\n}\n"
			if res.stdout != want {
				t.Errorf("stdout =\n%s\nwant\n%s", res.stdout, want)
			}
		})
	}
}

func TestEntityView_ExternalIDLooksUpByExtID(t *testing.T) {
	for _, entity := range []string{"company", "person"} {
		t.Run(entity, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("GET", "/"+entity+"/ext/erp-17/", 200, `{"success":true,"data":{"id":42}}`)

			res := h.run(entity, "view", "ext:erp-17")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if req := h.api.lastRequest(); req.method != "GET" || req.path != "/"+entity+"/ext/erp-17/" {
				t.Errorf("request = %s %s, want GET /%s/ext/erp-17/", req.method, req.path, entity)
			}
			if len(h.api.requests()) != 1 {
				t.Errorf("requests = %d, want 1", len(h.api.requests()))
			}
		})
	}
}

func TestEntityView_EmptyExternalIDIsAnError(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "view", "ext:")

	if res.err == nil {
		t.Error("err = nil, want an error for an empty external id")
	}
	if len(h.api.requests()) != 0 {
		t.Errorf("requests = %d, want none", len(h.api.requests()))
	}
}

func TestEntityView_RequiresID(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "view")

	if res.err == nil {
		t.Fatal("err = nil, want an argument error")
	}
	if len(h.api.requests()) != 0 {
		t.Errorf("requests = %d, want none", len(h.api.requests()))
	}
}

func TestEntityView_NotFoundIsAnError(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "view", "999")

	if res.err == nil || !strings.Contains(res.err.Error(), "404") {
		t.Errorf("err = %v, want an HTTP 404 error", res.err)
	}
}

// Each run builds a fresh command tree, so a flag set in one run must not
// carry over into the next.
func TestRuns_DoNotShareFlagState(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/", 200, `{"data":[]}`)

	if res := h.run("company", "list", "--offset", "60", "--fulltext", "acme", "--json"); res.err != nil {
		t.Fatalf("first run: err = %v", res.err)
	}
	res := h.run("company", "list")
	if res.err != nil {
		t.Fatalf("second run: err = %v", res.err)
	}

	q := h.api.lastRequest().query
	if q.Get("offset") != "0" || q.Has("fulltext") {
		t.Errorf("second run query = %v, want offset=0 and no fulltext", q)
	}
	if res.stdout != "No records found.\n" {
		t.Errorf("second run stdout = %q, want the plain (non --json) output", res.stdout)
	}
}

func TestVerbose_LogsRequestAndStatusToStderr(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/5/", 200, `{"data":{}}`)

	res := h.run("company", "view", "5", "--verbose")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	wantReq := "> GET " + h.api.hostFlag() + "/api/v2/company/5/\n"
	if !strings.Contains(res.stderr, wantReq) || !strings.Contains(res.stderr, "< 200 OK") {
		t.Errorf("stderr = %q, want %q and %q", res.stderr, wantReq, "< 200 OK")
	}
	if strings.Contains(res.stdout, "> GET") {
		t.Errorf("stdout = %q, want no verbose output", res.stdout)
	}
}

func TestEntityList_LimitSetsPageSize(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/", 200, `{"success":"true","totalCount":0,"data":[]}`)

	res := h.run("company", "list", "--limit", "1000", "--offset", "2000")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if want := (url.Values{"offset": {"2000"}, "limit": {"1000"}}); !reflect.DeepEqual(h.api.lastRequest().query, want) {
		t.Errorf("query = %v, want %v", h.api.lastRequest().query, want)
	}
}

func TestEntityList_RejectsLimitOutsideAPIRange(t *testing.T) {
	for _, limit := range []string{"0", "-1", "1001"} {
		t.Run(limit, func(t *testing.T) {
			h := newHarness(t)

			res := h.run("company", "list", "--limit", limit)

			if res.err == nil || !strings.Contains(res.err.Error(), "--limit must be between 1 and 1000") {
				t.Errorf("err = %v, want a --limit range error", res.err)
			}
			if len(h.api.requests()) != 0 {
				t.Errorf("requests = %d, want none", len(h.api.requests()))
			}
		})
	}
}

func TestEntityList_AllAndLimitAreExclusive(t *testing.T) {
	h := newHarness(t)

	res := h.run("company", "list", "--all", "--limit", "100")

	if res.err == nil {
		t.Error("err = nil, want an error for --all with --limit")
	}
	if len(h.api.requests()) != 0 {
		t.Errorf("requests = %d, want none", len(h.api.requests()))
	}
}
