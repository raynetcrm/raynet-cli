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

const offerWithItems = `{"success":true,"data":{"id":31,"name":"Q1","items":[
	{"id":5,"name":"Teambuilding","priceListItem":{"product":{"code":"T101"}},"count":2,"unit":"pcs","price":122,"discountPercent":0,"taxRate":21},
	{"id":6,"name":"Catering","count":1,"unit":"pcs","price":50,"discountPercent":10,"taxRate":21}]}}`

func TestItemList_ReadsItemsFromParentView(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/offer/31/", 200, offerWithItems)

	res := h.run("offer", "item", "list", "31")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "GET" || req.path != "/offer/31/" {
		t.Errorf("request = %s %s, want GET /offer/31/", req.method, req.path)
	}
	lines := strings.Split(strings.TrimRight(res.stdout, "\n"), "\n")
	want := []string{
		"id\tname\tpriceListItem.product.code\tcount\tunit\tprice\tdiscountPercent\ttaxRate",
		"5\tTeambuilding\tT101\t2\tpcs\t122\t0\t21",
		"6\tCatering\t\t1\tpcs\t50\t10\t21",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("stdout =\n%s\nwant\n%s", res.stdout, strings.Join(want, "\n"))
	}
}

// businessDocuments are the entities carrying line items: command and REST
// path segment.
var businessDocuments = []struct{ use, path string }{
	{"business-case", "businessCase"},
	{"offer", "offer"},
	{"sales-order", "salesOrder"},
}

func TestItem_MutationsSendMethodPathAndBody(t *testing.T) {
	for _, d := range businessDocuments {
		tests := []struct {
			name       string
			args       []string
			wantMethod string
			wantPath   string
			wantBody   any
		}{
			{"add", []string{"add", "31", "-f", "productCode=A001", "-F", "count=2"}, "PUT", "/" + d.path + "/31/item/",
				map[string]any{"productCode": "A001", "count": float64(2)}},
			{"update", []string{"update", "31", "5", "-F", "discountPercent=10.5"}, "POST", "/" + d.path + "/31/item/5/",
				map[string]any{"discountPercent": 10.5}},
			{"delete one", []string{"delete", "31", "5", "--yes"}, "DELETE", "/" + d.path + "/31/item/5/", nil},
			{"delete several", []string{"delete", "31", "5", "6", "7", "--yes"}, "DELETE", "/" + d.path + "/31/item/",
				map[string]any{"ids": []any{float64(5), float64(6), float64(7)}}},
		}
		for _, tt := range tests {
			t.Run(d.use+" "+tt.name, func(t *testing.T) {
				h := newHarness(t)
				h.api.reply(tt.wantMethod, tt.wantPath, 200, `{"success":true}`)

				res := h.run(append([]string{d.use, "item"}, tt.args...)...)

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				reqs := h.api.requests()
				if len(reqs) != 1 {
					t.Fatalf("got %d requests, want 1", len(reqs))
				}
				req := reqs[0]
				if req.method != tt.wantMethod || req.path != tt.wantPath {
					t.Errorf("request = %s %s, want %s %s", req.method, req.path, tt.wantMethod, tt.wantPath)
				}
				if got := req.json(t); !reflect.DeepEqual(got, tt.wantBody) {
					t.Errorf("body = %#v, want %#v", got, tt.wantBody)
				}
				if !strings.Contains(res.stdout, `"success"`) {
					t.Errorf("stdout = %q, want the API response", res.stdout)
				}
			})
		}
	}
}

func TestItem_ListAvailableOnEveryBusinessDocument(t *testing.T) {
	for _, d := range businessDocuments {
		t.Run(d.use, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("GET", "/"+d.path+"/31/", 200, offerWithItems)

			res := h.run(d.use, "item", "list", "31")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if !strings.Contains(res.stdout, "Catering") {
				t.Errorf("stdout = %q, want the items", res.stdout)
			}
		})
	}
}

func TestItemList_ExternalIDReadsExtView(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/offer/ext/erp-17/", 200, offerWithItems)

	res := h.run("offer", "item", "list", "ext:erp-17", "--columns", "id,name")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if want := "id\tname\n5\tTeambuilding\n6\tCatering\n"; res.stdout != want {
		t.Errorf("stdout = %q, want %q", res.stdout, want)
	}
}

func TestItemList_NoItems(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/offer/31/", 200, `{"success":true,"data":{"id":31,"items":[]}}`)

	res := h.run("offer", "item", "list", "31")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if res.stdout != "No records found.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
}

func TestItem_ExternalIDIsResolvedFirst(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/offer/ext/erp-17/", 200, `{"success":true,"data":{"id":31}}`)
	h.api.reply("POST", "/offer/31/item/5/", 200, `{"success":true}`)

	res := h.run("offer", "item", "update", "ext:erp-17", "5", "-F", "count=3")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "POST" || req.path != "/offer/31/item/5/" {
		t.Errorf("request = %s %s, want POST /offer/31/item/5/", req.method, req.path)
	}
}

func TestItemDelete_NonTerminalWithoutYesRefuses(t *testing.T) {
	for _, ids := range [][]string{{"5"}, {"5", "6"}} {
		t.Run(strings.Join(ids, ","), func(t *testing.T) {
			h := newHarness(t)

			res := h.run(append([]string{"offer", "item", "delete", "31"}, ids...)...)

			if res.err == nil || !strings.Contains(res.err.Error(), "--yes") {
				t.Errorf("err = %v, want a refusal mentioning --yes", res.err)
			}
			if n := len(h.api.requests()); n != 0 {
				t.Errorf("got %d requests, want none", n)
			}
		})
	}
}

func TestItemDelete_RejectsNonNumericIDsInBulk(t *testing.T) {
	h := newHarness(t)

	res := h.run("offer", "item", "delete", "31", "5", "x", "--yes")

	if res.err == nil || !strings.Contains(res.err.Error(), `"x"`) {
		t.Errorf("err = %v, want an invalid id error", res.err)
	}
	if n := len(h.api.requests()); n != 0 {
		t.Errorf("got %d requests, want none", n)
	}
}

func TestItemDelete_TerminalPrompts(t *testing.T) {
	tests := []struct {
		answer       string
		wantPrompt   string
		wantRequests int
		wantStderr   string
	}{
		{"y\n", "Delete items 5, 6 from offer 31? [y/N] ", 1, "Deleted items 5, 6 from offer 31"},
		{"\n", "Delete items 5, 6 from offer 31? [y/N] ", 0, "Cancelled."},
	}
	for _, tt := range tests {
		t.Run(strings.TrimSpace(tt.answer), func(t *testing.T) {
			h := newHarness(t)
			h.terminal = true
			h.stdin = tt.answer
			h.api.reply("DELETE", "/offer/31/item/", 200, `{"success":true}`)

			res := h.run("offer", "item", "delete", "31", "5", "6")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if !strings.HasPrefix(res.stderr, tt.wantPrompt) {
				t.Errorf("stderr = %q, want prompt %q", res.stderr, tt.wantPrompt)
			}
			if !strings.Contains(res.stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want %q", res.stderr, tt.wantStderr)
			}
			if n := len(h.api.requests()); n != tt.wantRequests {
				t.Errorf("got %d requests, want %d", n, tt.wantRequests)
			}
		})
	}
}

func TestItemAdd_TerminalConfirms(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.api.reply("PUT", "/offer/31/item/", 201, `{"success":true,"data":{"id":9}}`)

	res := h.run("offer", "item", "add", "31", "-f", "name=Setup")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if !strings.Contains(res.stderr, "Added item 9 to offer 31") {
		t.Errorf("stderr = %q, want a confirmation", res.stderr)
	}
}

func TestItemAddUpdateHelp_ListsFieldReference(t *testing.T) {
	tests := []struct {
		verb      string
		want      []string
		wantLacks string
	}{
		// product, productCode and priceList can't change on update.
		{"add", []string{"discountPercent", "productCode", "Examples:", "raynet offer item add 123"}, ""},
		{"update", []string{"discountPercent", "Examples:", "raynet offer item update 123 5"}, "productCode"},
	}
	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			h := newHarness(t)

			res := h.run("offer", "item", tt.verb, "--help")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			for _, want := range tt.want {
				if !strings.Contains(res.stdout, want) {
					t.Errorf("help lacks %q:\n%s", want, res.stdout)
				}
			}
			if tt.wantLacks != "" && strings.Contains(res.stdout, tt.wantLacks) {
				t.Errorf("help mentions %q:\n%s", tt.wantLacks, res.stdout)
			}
		})
	}
}
