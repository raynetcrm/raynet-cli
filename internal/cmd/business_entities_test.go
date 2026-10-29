/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// businessEntities are the business entity rows (#8) as the REST API
// documents them: path segment, aliases, web-app name, the actions the API
// supports and the nested collections.
var businessEntities = []struct {
	use     string
	path    string
	aliases []string
	webName string
	actions []string
	nested  []string
}{
	{"lead", "lead", nil, "Lead", []string{"lock", "unlock", "convert", "tag", "untag"}, nil},
	{"business-case", "businessCase", []string{"businessCase", "bc"}, "BusinessCase", []string{"lock", "unlock", "invalidate", "revalidate", "tag", "untag"}, []string{"item"}},
	{"offer", "offer", nil, "Offer", []string{"lock", "unlock", "invalidate", "revalidate", "sync", "unsync", "tag", "untag"}, []string{"item"}},
	{"sales-order", "salesOrder", []string{"salesOrder", "so"}, "SalesOrder", []string{"lock", "unlock", "invalidate", "revalidate", "sync", "unsync", "tag", "untag"}, []string{"item"}},
	{"project", "project", nil, "Project", []string{"lock", "unlock", "invalidate", "revalidate", "tag", "untag"}, nil},
	{"invoice", "invoiceLight", []string{"invoiceLight"}, "Invoice", []string{"lock", "unlock", "tag", "untag"}, nil},
	{"product", "product", nil, "Product", []string{"invalidate", "revalidate", "tag", "untag"}, nil},
	{"price-list", "priceList", []string{"priceList", "pl"}, "PriceList", []string{"lock", "unlock", "tag", "untag"}, nil},
}

func TestBusinessEntity_ListFetchesFirstPageByNameAndAlias(t *testing.T) {
	for _, e := range businessEntities {
		for _, name := range append([]string{e.use}, e.aliases...) {
			t.Run(name, func(t *testing.T) {
				h := newHarness(t)
				h.api.reply("GET", "/"+e.path+"/", 200, `{"success":true,"totalCount":1,"data":[{"id":42}]}`)

				res := h.run(name, "list")

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				req := h.api.lastRequest()
				if req.method != "GET" || req.path != "/"+e.path+"/" {
					t.Errorf("request = %s %s, want GET /%s/", req.method, req.path, e.path)
				}
				if got := req.query.Get("limit"); got != "30" {
					t.Errorf("limit = %q, want 30", got)
				}
				if !strings.Contains(res.stdout, "42") {
					t.Errorf("stdout = %q, want the record row", res.stdout)
				}
			})
		}
	}
}

func TestBusinessEntity_CRUDAndExternalID(t *testing.T) {
	for _, e := range businessEntities {
		tests := []struct {
			name       string
			args       []string
			wantMethod string
			wantPath   string
			wantBody   any
		}{
			{"view ext", []string{"view", "ext:erp-17"}, "GET", "/" + e.path + "/ext/erp-17/", nil},
			{"create", []string{"create", "-f", "name=Acme"}, "PUT", "/" + e.path + "/", map[string]any{"name": "Acme"}},
			{"update", []string{"update", "42", "-F", "owner=7"}, "POST", "/" + e.path + "/42/", map[string]any{"owner": float64(7)}},
			{"delete", []string{"delete", "42", "--yes"}, "DELETE", "/" + e.path + "/42/", nil},
		}
		for _, tt := range tests {
			t.Run(e.use+" "+tt.name, func(t *testing.T) {
				h := newHarness(t)
				h.api.reply(tt.wantMethod, tt.wantPath, 200, `{"success":true,"data":{"id":42}}`)

				res := h.run(append([]string{e.use}, tt.args...)...)

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				req := h.api.lastRequest()
				if req.method != tt.wantMethod || req.path != tt.wantPath {
					t.Errorf("request = %s %s, want %s %s", req.method, req.path, tt.wantMethod, tt.wantPath)
				}
				if got := req.json(t); !reflect.DeepEqual(got, tt.wantBody) {
					t.Errorf("body = %v, want %v", got, tt.wantBody)
				}
			})
		}
	}
}

func TestBusinessEntity_UpdateResolvesExternalID(t *testing.T) {
	for _, e := range businessEntities {
		t.Run(e.use, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("GET", "/"+e.path+"/ext/erp-17/", 200, `{"success":true,"data":{"id":42}}`)
			h.api.reply("POST", "/"+e.path+"/42/", 200, `{"success":true}`)

			res := h.run(e.use, "update", "ext:erp-17", "-f", "description=x")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			if req := h.api.lastRequest(); req.method != "POST" || req.path != "/"+e.path+"/42/" {
				t.Errorf("request = %s %s, want POST /%s/42/", req.method, req.path, e.path)
			}
		})
	}
}

// subcommands returns the names listed under "Available Commands:" in help.
func subcommands(help string) []string {
	var names []string
	in := false
	for _, line := range strings.Split(help, "\n") {
		switch {
		case strings.HasPrefix(line, "Available Commands:"):
			in = true
		case in && strings.TrimSpace(line) == "":
			in = false
		case in:
			names = append(names, strings.Fields(line)[0])
		}
	}
	return names
}

func TestBusinessEntity_OffersExactlyTheActionsAndNestedCollectionsItsAPISupports(t *testing.T) {
	for _, e := range businessEntities {
		t.Run(e.use, func(t *testing.T) {
			h := newHarness(t)

			res := h.run(e.use, "--help")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			want := slices.Concat([]string{"list", "view", "create", "update", "delete"}, e.actions, e.nested)
			got := subcommands(res.stdout)
			slices.Sort(want)
			slices.Sort(got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("subcommands = %v, want %v", got, want)
			}
		})
	}
}

func TestBusinessEntity_ActionsCallTheirEndpoints(t *testing.T) {
	endpoints := map[string]struct{ method, suffix string }{
		"lock":       {"POST", "lock"},
		"unlock":     {"POST", "unlock"},
		"invalidate": {"POST", "invalid"},
		"revalidate": {"POST", "valid"},
		"sync":       {"POST", "sync"},
		"unsync":     {"DELETE", "sync"},
		"tag":        {"PUT", "tag/"},
		"untag":      {"DELETE", "tag/"},
		"convert":    {"POST", "convert/"},
	}
	for _, e := range businessEntities {
		for _, act := range e.actions {
			t.Run(e.use+" "+act, func(t *testing.T) {
				h := newHarness(t)
				ep := endpoints[act]
				wantPath := "/" + e.path + "/42/" + ep.suffix
				h.api.reply(ep.method, wantPath, 200, `{"success":true}`)

				args := []string{e.use, act, "42"}
				switch act {
				case "tag", "untag":
					args = append(args, "vip")
				case "convert":
					args = append(args, "-F", "company=7")
				}
				res := h.run(args...)

				if res.err != nil {
					t.Fatalf("err = %v", res.err)
				}
				req := h.api.lastRequest()
				if req.method != ep.method || req.path != wantPath {
					t.Errorf("request = %s %s, want %s %s", req.method, req.path, ep.method, wantPath)
				}
			})
		}
	}
}

func TestLeadConvert_SendsBodyToConvertEndpoint(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.api.reply("POST", "/lead/42/convert/", 200, ``)

	res := h.run("lead", "convert", "42", "-F", "company=101", "-F", "person=205")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	req := h.api.lastRequest()
	if req.method != "POST" || req.path != "/lead/42/convert/" {
		t.Errorf("request = %s %s, want POST /lead/42/convert/", req.method, req.path)
	}
	want := map[string]any{"company": float64(101), "person": float64(205)}
	if got := req.json(t); !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	if res.stderr != "Converted lead 42\n" {
		t.Errorf("stderr = %q, want %q", res.stderr, "Converted lead 42\n")
	}
}

func TestLeadConvertHelp_ListsConvertFields(t *testing.T) {
	h := newHarness(t)

	res := h.run("lead", "convert", "--help")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	for _, pattern := range []string{
		`^  company +id +.*points to: company$`,
		`^  businessCase +id +.*points to: business-case$`,
	} {
		if !regexp.MustCompile("(?m)" + pattern).MatchString(res.stdout) {
			t.Errorf("help has no line matching %s:\n%s", pattern, res.stdout)
		}
	}
}

func TestBusinessDocument_CreateWithItemsUsesCreateWithItems(t *testing.T) {
	for _, e := range []struct{ use, path string }{
		{"business-case", "businessCase"},
		{"offer", "offer"},
		{"sales-order", "salesOrder"},
	} {
		t.Run(e.use, func(t *testing.T) {
			h := newHarness(t)
			wantPath := "/" + e.path + "/createWithItems"
			h.api.reply("PUT", wantPath, 201, `{"success":true,"data":{"id":1}}`)

			res := h.run(e.use, "create", "-f", "name=Deal", "-F", "company=7", "-f", "items[][productCode]=002", "-F", "items[][count]=7")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			req := h.api.lastRequest()
			if req.method != "PUT" || req.path != wantPath {
				t.Errorf("request = %s %s, want PUT %s", req.method, req.path, wantPath)
			}
			want := map[string]any{
				"name":    "Deal",
				"company": float64(7),
				"items":   []any{map[string]any{"productCode": "002", "count": float64(7)}},
			}
			if got := req.json(t); !reflect.DeepEqual(got, want) {
				t.Errorf("body = %v, want %v", got, want)
			}
		})
	}
}

func TestNonDocumentCreate_IgnoresItems(t *testing.T) {
	h := newHarness(t)
	h.api.reply("PUT", "/invoiceLight/", 201, `{"success":true,"data":{"id":1}}`)

	res := h.run("invoice", "create", "-f", "items[][name]=Line")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.path != "/invoiceLight/" {
		t.Errorf("request path = %s, want /invoiceLight/", req.path)
	}
}

func TestBusinessEntity_ViewWebOpensRecordByWebName(t *testing.T) {
	for _, e := range businessEntities {
		t.Run(e.use, func(t *testing.T) {
			h := newHarness(t)
			p := h.api.profile()
			p.WebURL = "http://localhost:3000"
			p.InstanceName = "acme"
			h.storeProfile("dev", p)

			res := h.runRaw(e.use, "view", "7", "--web")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			want := []string{"http://localhost:3000/acme/redir?en=" + e.webName + "&ei=7"}
			if !reflect.DeepEqual(res.browserURLs, want) {
				t.Errorf("browserURLs = %v, want %v", res.browserURLs, want)
			}
		})
	}
}

func TestBusinessEntity_CreateHelpListsFieldReference(t *testing.T) {
	tests := []struct {
		entity string
		lines  []string // regexps, each must match a whole help line
	}{
		{"lead", []string{
			`^  topic +string +required +subject$`,
			`^  priority +string +required +priority; values: MINOR, DEFAULT, CRITICAL$`,
			`^  tags +string +comma-separated tag names$`,
		}},
		{"business-case", []string{`^  company +id +required +client; points to: company$`}},
		{"offer", []string{`^  businessCase +id +required +business case; points to: business-case$`}},
		{"sales-order", []string{`^  salesOrderStatus +id +.*codelist: salesOrderStatus$`}},
		{"project", []string{`^  projectStatus +id +.*codelist: projectStatus$`}},
		{"invoice", []string{`^  taxPayer +string +required +.*values: YES, NO$`}},
		{"product", []string{`^  code +string +required +product code$`}},
		{"price-list", []string{`^  currency +id +required +currency; codelist: currency$`}},
	}
	for _, tt := range tests {
		t.Run(tt.entity, func(t *testing.T) {
			h := newHarness(t)

			res := h.run(tt.entity, "create", "--help")

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			for _, pattern := range tt.lines {
				if !regexp.MustCompile("(?m)" + pattern).MatchString(res.stdout) {
					t.Errorf("help has no line matching %s:\n%s", pattern, res.stdout)
				}
			}
		})
	}
}
