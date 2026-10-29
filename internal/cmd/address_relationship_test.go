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

const companyWithAddresses = `{"success":true,"data":{"id":12,"name":"ACME","addresses":[
	{"id":2,"primary":true,"contactAddress":false,"address":{"id":2,"name":"HQ","street":"Main 1","city":"Brno","zipCode":"602 00","country":"Czech Republic"},"contactInfo":{"email":"hq@acme.cz","tel1":"+420 1"}},
	{"id":3,"primary":false,"contactAddress":true,"address":{"id":3,"name":"Branch","city":"Ostrava"}}]}}`

func TestAddressList_ReadsAddressesFromCompanyView(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/12/", 200, companyWithAddresses)

	res := h.run("company", "address", "list", "12")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if reqs := h.api.requests(); len(reqs) != 1 || reqs[0].method != "GET" || reqs[0].path != "/company/12/" {
		t.Errorf("requests = %+v, want only GET /company/12/", reqs)
	}
	want := "id\tprimary\tcontactAddress\taddress.name\taddress.street\taddress.city\taddress.zipCode\taddress.country\tcontactInfo.email\tcontactInfo.tel1\n" +
		"2\ttrue\tfalse\tHQ\tMain 1\tBrno\t602 00\tCzech Republic\thq@acme.cz\t+420 1\n" +
		"3\tfalse\ttrue\tBranch\t\tOstrava\t\t\t\t\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
}

// nestedCase is one nested-collection invocation and the single request it
// must send.
type nestedCase struct {
	name       string
	args       []string
	wantMethod string
	wantPath   string
	wantBody   any
}

func runNestedCases(t *testing.T, tests []nestedCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply(tt.wantMethod, tt.wantPath, 200, `{"success":true}`)

			res := h.run(tt.args...)

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
		})
	}
}

func TestAddress_MutationsAndActionsSendMethodPathAndBody(t *testing.T) {
	runNestedCases(t, []nestedCase{
		{"add", []string{"company", "address", "add", "12", "-f", "address[name]=HQ", "-f", "address[country]=CZ", "-F", "territory=4"},
			"PUT", "/company/12/address/",
			map[string]any{"address": map[string]any{"name": "HQ", "country": "CZ"}, "territory": float64(4)}},
		{"update", []string{"company", "address", "update", "12", "3", "-f", "address[name]=Branch", "-f", "contactInfo[email]=b@acme.cz"},
			"POST", "/company/12/address/3/",
			map[string]any{"address": map[string]any{"name": "Branch"}, "contactInfo": map[string]any{"email": "b@acme.cz"}}},
		{"delete", []string{"company", "address", "delete", "12", "3", "--yes"}, "DELETE", "/company/12/address/3/", nil},
		{"set-primary", []string{"company", "address", "set-primary", "12", "3"}, "POST", "/company/12/address/3/setPrimary/", nil},
		{"set-contact", []string{"company", "address", "set-contact", "12", "3"}, "POST", "/company/12/address/3/setContact/", nil},
	})
}

func TestAddressAction_ExternalIDIsResolvedFirst(t *testing.T) {
	h := newHarness(t)
	h.terminal = true
	h.api.reply("GET", "/company/ext/erp-17/", 200, `{"success":true,"data":{"id":12}}`)
	h.api.reply("POST", "/company/12/address/3/setPrimary/", 200, `{"success":true}`)

	res := h.run("company", "address", "set-primary", "ext:erp-17", "3")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.method != "POST" || req.path != "/company/12/address/3/setPrimary/" {
		t.Errorf("request = %s %s, want POST /company/12/address/3/setPrimary/", req.method, req.path)
	}
	if !strings.Contains(res.stderr, "address 3 of company 12") {
		t.Errorf("stderr = %q, want a confirmation", res.stderr)
	}
}

func TestCompanyRelationshipList_UsesItsOwnEndpoint(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/12/relationship/", 200, `{"success":true,"totalCount":2,"data":[
		{"id":1,"notice":null,"companyJoin":{"id":40,"name":"Parent Ltd"},"companyJoinType":"slave"},
		{"id":2,"notice":"JV","companyJoin":{"id":41,"name":"Partner Ltd"},"companyJoinType":"free"}]}`)

	res := h.run("company", "relationship", "list", "12")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if reqs := h.api.requests(); len(reqs) != 1 || reqs[0].method != "GET" || reqs[0].path != "/company/12/relationship/" {
		t.Errorf("requests = %+v, want only GET /company/12/relationship/", reqs)
	}
	want := "id\tcompanyJoin.id\tcompanyJoin.name\tcompanyJoinType\tnotice\n" +
		"1\t40\tParent Ltd\tslave\t\n" +
		"2\t41\tPartner Ltd\tfree\tJV\n"
	if res.stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", res.stdout, want)
	}
}

func TestCompanyRelationshipList_ExternalIDIsResolvedFirst(t *testing.T) {
	h := newHarness(t)
	h.api.reply("GET", "/company/ext/erp-17/", 200, `{"success":true,"data":{"id":12}}`)
	h.api.reply("GET", "/company/12/relationship/", 200, `{"success":true,"totalCount":0,"data":[]}`)

	res := h.run("company", "relationship", "list", "ext:erp-17")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	if req := h.api.lastRequest(); req.path != "/company/12/relationship/" {
		t.Errorf("last request = %s %s, want GET /company/12/relationship/", req.method, req.path)
	}
	if res.stdout != "No records found.\n" {
		t.Errorf("stdout = %q", res.stdout)
	}
}

func TestCompanyRelationship_MutationsSendMethodPathAndBody(t *testing.T) {
	runNestedCases(t, []nestedCase{
		{"add", []string{"company", "relationship", "add", "12", "-F", "companyJoin=40", "-f", "companyJoinType=parent"},
			"PUT", "/company/12/relationship/", map[string]any{"companyJoin": float64(40), "companyJoinType": "parent"}},
		{"update", []string{"company", "relationship", "update", "12", "1", "-f", "notice=JV"},
			"POST", "/company/12/relationship/1/", map[string]any{"notice": "JV"}},
		{"delete", []string{"company", "relationship", "delete", "12", "1", "--yes"}, "DELETE", "/company/12/relationship/1/", nil},
	})
}

func TestPersonRelationship_MutationsAndSetPrimarySendMethodPathAndBody(t *testing.T) {
	runNestedCases(t, []nestedCase{
		{"add", []string{"person", "relationship", "add", "7", "-F", "company=12", "-f", "type=CEO"},
			"PUT", "/person/7/relationship/", map[string]any{"company": float64(12), "type": "CEO"}},
		{"update", []string{"person", "relationship", "update", "7", "1", "-F", "company=12", "-f", "type=CTO", "-F", "companyAddress=3"},
			"POST", "/person/7/relationship/1/", map[string]any{"company": float64(12), "type": "CTO", "companyAddress": float64(3)}},
		{"delete", []string{"person", "relationship", "delete", "7", "1", "--yes"}, "DELETE", "/person/7/relationship/1/", nil},
		{"set-primary", []string{"person", "relationship", "set-primary", "7", "1"}, "POST", "/person/7/relationship/1/setPrimary/", nil},
	})
}

// Each collection offers only the verbs its API supports.
func TestNested_OffersOnlySupportedVerbs(t *testing.T) {
	tests := []struct {
		args []string
		want []string
		lack []string
	}{
		{[]string{"company", "address"}, []string{"list", "add", "update", "delete", "set-primary", "set-contact"}, nil},
		{[]string{"company", "relationship"}, []string{"list", "add", "update", "delete"}, []string{"set-primary", "set-contact"}},
		{[]string{"person", "relationship"}, []string{"add", "update", "delete", "set-primary"}, []string{"list", "set-contact"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			h := newHarness(t)

			res := h.run(append(tt.args, "--help")...)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			got := subcommandNames(res.stdout)
			for _, w := range tt.want {
				if !got[w] {
					t.Errorf("subcommands %v lack %q", got, w)
				}
			}
			for _, l := range tt.lack {
				if got[l] {
					t.Errorf("subcommands %v offer %q", got, l)
				}
			}
			if strings.Contains(res.stdout, "list") != got["list"] {
				t.Errorf("help mentions list without offering it:\n%s", res.stdout)
			}
		})
	}
}

// subcommandNames are the command names under "Available Commands:" in
// cobra help output.
func subcommandNames(help string) map[string]bool {
	names := map[string]bool{}
	_, after, ok := strings.Cut(help, "Available Commands:\n")
	if !ok {
		return names
	}
	for _, line := range strings.Split(after, "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		names[strings.Fields(line)[0]] = true
	}
	return names
}

func TestNestedHelp_ListsFieldReference(t *testing.T) {
	tests := []struct {
		args []string
		want []string
		lack []string
	}{
		{[]string{"company", "address", "add"}, []string{"address", "contactInfo", "territory", "codelist: territory"}, nil},
		{[]string{"company", "relationship", "add"}, []string{"companyJoin", "companyJoinType", "parent, slave, free"}, nil},
		{[]string{"company", "relationship", "update"}, []string{"notice"}, []string{"companyJoin"}},
		{[]string{"person", "relationship", "update"}, []string{"company", "type", "companyAddress", "required applies to update too", "raynet person relationship update 123 5 -F company=1 -f type=CEO"}, nil},
		{[]string{"company", "address", "list"}, []string{"List the addresses of a company record"}, []string{"addresss"}},
		{[]string{"company", "address"}, []string{"List the addresses of a company record"}, []string{"addresss"}},
		{[]string{"company", "address", "set-contact"}, []string{"raynet company address set-contact ext:erp-17 5"}, nil},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			h := newHarness(t)

			res := h.run(append(tt.args, "--help")...)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			for _, w := range tt.want {
				if !strings.Contains(res.stdout, w) {
					t.Errorf("help lacks %q:\n%s", w, res.stdout)
				}
			}
			for _, l := range tt.lack {
				if strings.Contains(res.stdout, l) {
					t.Errorf("help mentions %q:\n%s", l, res.stdout)
				}
			}
		})
	}
}
