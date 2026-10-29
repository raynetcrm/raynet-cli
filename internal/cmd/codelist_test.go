/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"reflect"
	"strings"
	"testing"
)

// allCodelists is every Picklists-tagged codelist in the REST API's OpenAPI
// spec, the independent source the catalog is checked against.
var allCodelists = []string{
	"leadCategory", "companyCategory", "contactSource", "employeesNumber", "legalForm", "paymentTerm",
	"companyTurnover", "economyActivity", "companyClassification1", "companyClassification2",
	"companyClassification3", "territory", "personCategory", "personClassification1",
	"personClassification2", "personClassification3", "language", "maritalStatus", "productCategory",
	"productLine", "businessCaseCategory", "businessCaseRelationshipCategory", "businessCaseType",
	"businessCasePhase", "businessCaseClassification1", "businessCaseClassification2",
	"businessCaseClassification3", "currency", "losingCategory", "telType", "leadPhase", "offerCategory",
	"salesOrderCategory", "projectCategory", "projectRelationshipCategory", "priceListCategory",
	"offerStatus", "salesOrderStatus", "projectStatus", "taxRate", "gdprFormAgreement",
	"activityCategory", "documentCategory", "invoiceCategory",
}

func TestCodelist_PrintsEveryCodelistName(t *testing.T) {
	h := newHarness(t)

	res := h.run("codelist")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	got := strings.Fields(res.stdout)
	if len(got) != len(allCodelists) {
		t.Errorf("printed %d names, want %d:\n%s", len(got), len(allCodelists), res.stdout)
	}
	printed := map[string]bool{}
	for _, n := range got {
		printed[n] = true
	}
	for _, n := range allCodelists {
		if !printed[n] {
			t.Errorf("codelist %q not printed", n)
		}
	}
	if reqs := h.api.requests(); len(reqs) != 0 {
		t.Errorf("made %d API requests, want none (the catalog is static)", len(reqs))
	}
}

func TestCodelistList_GetsEntriesAsTable(t *testing.T) {
	for _, args := range [][]string{
		{"codelist", "companyCategory"},
		{"codelist", "companyCategory", "list"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("GET", "/companyCategory/", 200,
				`{"success":"true","totalCount":1,"data":[{"id":3,"code01":"VIP","code02":"B780FF","sequenceNumber":1}]}`)

			res := h.run(args...)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			req := h.api.lastRequest()
			if req.method != "GET" || req.path != "/companyCategory/" || len(req.query) != 0 {
				t.Errorf("request = %s %s?%s, want GET /companyCategory/ without query", req.method, req.path, req.query.Encode())
			}
			want := "id\tcode01\tcode02\tsequenceNumber\n3\tVIP\tB780FF\t1\n"
			if res.stdout != want {
				t.Errorf("stdout = %q, want %q", res.stdout, want)
			}
		})
	}
}

func TestCodelistCreate_MapsNameAndColorToCodeFields(t *testing.T) {
	h := newHarness(t)
	h.api.reply("PUT", "/companyCategory/", 201, `{"success":"true","data":3}`)

	res := h.run("codelist", "companyCategory", "create", "--name", "VIP", "--color", "B780FF")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "PUT" || req.path != "/companyCategory/" {
		t.Errorf("request = %s %s, want PUT /companyCategory/", req.method, req.path)
	}
	want := map[string]any{"code01": "VIP", "code02": "B780FF"}
	if got := req.json(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	wantOut := "{\n  \"success\": \"true\",\n  \"data\": 3\n}\n"
	if res.stdout != wantOut {
		t.Errorf("stdout = %q, want %q", res.stdout, wantOut)
	}
}

func TestCodelistCreate_BodyFlagsAddFieldsAndNameWins(t *testing.T) {
	h := newHarness(t)
	h.api.reply("PUT", "/businessCasePhase/", 201, `{"success":"true","data":9}`)

	res := h.run("codelist", "businessCasePhase", "create",
		"-f", "code01=ignored", "-F", "probability=40", "--name", "Offer sent")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	want := map[string]any{"code01": "Offer sent", "probability": float64(40)}
	if got := h.api.lastRequest().json(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestCodelistCreate_TerminalConfirmsBareNumberID(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.api.reply("PUT", "/territory/", 201, `{"success":"true","data":17}`)

	res := h.run("codelist", "territory", "create", "--name", "South")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stderr != "Created territory entry 17\n" {
		t.Errorf("stderr = %q, want %q", res.stderr, "Created territory entry 17\n")
	}
}

func TestCodelistUpdate_PostsNameToEntry(t *testing.T) {
	h := newHarness(t)
	h.api.reply("POST", "/companyCategory/3/", 200, `{"success":true}`)

	res := h.run("codelist", "companyCategory", "update", "3", "--name", "Key account", "--color", "5E73E3")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "POST" || req.path != "/companyCategory/3/" {
		t.Errorf("request = %s %s, want POST /companyCategory/3/", req.method, req.path)
	}
	want := map[string]any{"code01": "Key account", "code02": "5E73E3"}
	if got := req.json(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestCodelistDelete_DeletesEntryWithYes(t *testing.T) {
	h := newHarness(t)
	h.api.reply("DELETE", "/taxRate/5/", 200, ``)

	res := h.run("codelist", "taxRate", "delete", "5", "--yes")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "DELETE" || req.path != "/taxRate/5/" {
		t.Errorf("request = %s %s, want DELETE /taxRate/5/", req.method, req.path)
	}
}

func TestCodelistDelete_RefusesWithoutConfirmation(t *testing.T) {
	h := newHarness(t)

	res := h.run("codelist", "territory", "delete", "5")

	if res.err == nil || !strings.Contains(res.err.Error(), "--yes") {
		t.Errorf("err = %v, want a refusal mentioning --yes", res.err)
	}
	if reqs := h.api.requests(); len(reqs) != 0 {
		t.Errorf("made %d API requests, want none", len(reqs))
	}
}

func TestCodelistDelete_TerminalAsksFirst(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.stdin = "n\n"

	res := h.run("codelist", "territory", "delete", "5")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if !strings.Contains(res.stderr, "Delete territory entry 5? [y/N]") {
		t.Errorf("stderr = %q, want the confirmation question", res.stderr)
	}
	if reqs := h.api.requests(); len(reqs) != 0 {
		t.Errorf("made %d API requests after declining, want none", len(reqs))
	}
}

func TestCodelist_OffersOnlySupportedVerbs(t *testing.T) {
	// validArgs would succeed if the verb existed, so only a missing verb fails.
	validArgs := map[string][]string{
		"create": {"--name", "x"},
		"update": {"5", "--name", "x"},
		"delete": {"5", "--yes"},
	}
	unsupported := map[string][]string{"businessCaseType": {"create", "update", "delete"}}
	for _, name := range []string{"businessCasePhase", "leadPhase", "offerStatus", "salesOrderStatus", "projectStatus", "taxRate"} {
		unsupported[name] = []string{"update"}
	}
	for name, verbs := range unsupported {
		for _, v := range verbs {
			t.Run(name+" "+v, func(t *testing.T) {
				h := newHarness(t)

				res := h.run(append([]string{"codelist", name, v}, validArgs[v]...)...)

				if res.err == nil {
					t.Errorf("err = nil, want %s to be unavailable", v)
				}
				if reqs := h.api.requests(); len(reqs) != 0 {
					t.Errorf("made %d API requests, want none", len(reqs))
				}
			})
		}
	}
	for _, name := range allCodelists {
		if _, ok := unsupported[name]; ok {
			continue
		}
		t.Run(name+" update", func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("POST", "/"+name+"/5/", 200, `{"success":true}`)

			if res := h.run("codelist", name, "update", "5", "--name", "x"); res.err != nil {
				t.Errorf("err = %v, want update to be offered", res.err)
			}
		})
	}
}
