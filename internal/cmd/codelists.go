/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

// codelistSpec is one row of the codelist table: a codelist's REST API path
// segment, the verbs it supports and its body fields. Every codelist shares
// the same endpoints (GET/PUT /<name>/, POST/DELETE /<name>/<id>/) and the
// generic code01/code02 fields; see newCodelistCmd.
type codelistSpec struct {
	name string // API path segment and command name: "companyCategory"
	// verbs: verbList, verbCreate, verbUpdate, verbDelete.
	verbs verb
	// color: code02 is a color, set with --color.
	color   bool
	columns []string
	fields  []field
}

const (
	codelistCRUD         = verbList | verbCreate | verbUpdate | verbDelete
	codelistCreateDelete = verbList | verbCreate | verbDelete
)

// codelistColors are the colors the API documents for category codelists.
var codelistColors = []string{
	"B780FF (purple)", "EB6CD1 (pink)", "F76D8F (red)", "E9802F (orange)", "E2B23A (yellow)",
	"CABC00 (khaki)", "8EA500 (olive)", "60AE00 (lime)", "00B17B (green)", "00A9CA (turquoise)",
	"519CFF (azure)", "5E73E3 (blue)",
}

// plainCodelist is a full-CRUD codelist whose entries carry only a name.
func plainCodelist(name string) codelistSpec {
	return codelistSpec{
		name:    name,
		verbs:   codelistCRUD,
		columns: []string{"id", "code01", "sequenceNumber"},
		fields:  []field{{name: "code01", kind: kindString, required: true, help: "item name (--name)"}},
	}
}

// categoryCodelist is a full-CRUD codelist whose entries carry a name and a
// color.
func categoryCodelist(name string) codelistSpec {
	return codelistSpec{
		name:    name,
		verbs:   codelistCRUD,
		color:   true,
		columns: []string{"id", "code01", "code02", "sequenceNumber"},
		fields: []field{
			{name: "code01", kind: kindString, required: true, help: "category name (--name)"},
			{name: "code02", kind: kindString, help: "color, hex without # (--color)", values: codelistColors},
		},
	}
}

// statusCodelist is a create+delete-only codelist of named statuses.
func statusCodelist(name string) codelistSpec {
	return codelistSpec{
		name:    name,
		verbs:   codelistCreateDelete,
		columns: []string{"id", "code01", "locked", "sequenceNumber"},
		fields:  []field{{name: "code01", kind: kindString, required: true, help: "status name (--name)"}},
	}
}

// codelistTable lists every codelist in the REST API (tag "Picklists"), in
// the OpenAPI spec's order. The API has no catalog endpoint, so this table is
// the catalog `raynet codelist` prints; it is checked by hand against the
// OpenAPI spec.
var codelistTable = []codelistSpec{
	categoryCodelist("leadCategory"),
	categoryCodelist("companyCategory"),
	plainCodelist("contactSource"),
	plainCodelist("employeesNumber"),
	plainCodelist("legalForm"),
	plainCodelist("paymentTerm"),
	plainCodelist("companyTurnover"),
	plainCodelist("economyActivity"),
	plainCodelist("companyClassification1"),
	plainCodelist("companyClassification2"),
	plainCodelist("companyClassification3"),
	plainCodelist("territory"),
	categoryCodelist("personCategory"),
	plainCodelist("personClassification1"),
	plainCodelist("personClassification2"),
	plainCodelist("personClassification3"),
	plainCodelist("language"),
	plainCodelist("maritalStatus"),
	categoryCodelist("productCategory"),
	plainCodelist("productLine"),
	categoryCodelist("businessCaseCategory"),
	categoryCodelist("businessCaseRelationshipCategory"),
	{
		name:    "businessCaseType",
		verbs:   verbList,
		columns: []string{"id", "code01", "sequenceNumber"},
	},
	{
		name:    "businessCasePhase",
		verbs:   codelistCreateDelete,
		color:   true,
		columns: []string{"id", "code01", "code02", "probability", "strValue01", "locked", "sequenceNumber"},
		fields: []field{
			{name: "code01", kind: kindString, required: true, help: "phase name (--name)"},
			{name: "code02", kind: kindString, help: "color, hex without # (--color)"},
			{name: "probability", kind: kindNumber, help: "default win probability in percent"},
			{name: "businessCaseTypeId", kind: kindID, codelist: "businessCaseType",
				help: "business case type; required when the Trade Types add-on is on"},
		},
	},
	plainCodelist("businessCaseClassification1"),
	plainCodelist("businessCaseClassification2"),
	plainCodelist("businessCaseClassification3"),
	{
		name:    "currency",
		verbs:   codelistCRUD,
		columns: []string{"id", "code01", "code02", "strValue01", "sequenceNumber"},
		fields: []field{
			{name: "code01", kind: kindString, required: true, example: "Czech koruna", help: "currency name (--name)"},
			{name: "code02", kind: kindString, required: true, help: "ISO currency code, e.g. CZK"},
			{name: "strValue01", kind: kindString, help: "description"},
		},
	},
	plainCodelist("losingCategory"),
	plainCodelist("telType"),
	{
		name:    "leadPhase",
		verbs:   codelistCreateDelete,
		color:   true,
		columns: []string{"id", "code01", "code02", "locked", "sequenceNumber"},
		fields: []field{
			{name: "code01", kind: kindString, required: true, help: "phase name (--name)"},
			{name: "code02", kind: kindString, help: "color, hex without # (--color)"},
		},
	},
	categoryCodelist("offerCategory"),
	categoryCodelist("salesOrderCategory"),
	categoryCodelist("projectCategory"),
	categoryCodelist("projectRelationshipCategory"),
	categoryCodelist("priceListCategory"),
	statusCodelist("offerStatus"),
	statusCodelist("salesOrderStatus"),
	statusCodelist("projectStatus"),
	{
		name:    "taxRate",
		verbs:   codelistCreateDelete,
		columns: []string{"id", "code01", "numValue01", "strValue01", "sequenceNumber"},
		fields: []field{
			{name: "code01", kind: kindString, required: true, help: "rate as percentage text, e.g. 21% (--name)"},
			{name: "numValue01", kind: kindNumber, help: "numeric rate; computed from code01 when omitted"},
			{name: "strValue01", kind: kindString, help: "rate name"},
		},
	},
	{
		name:    "gdprFormAgreement",
		verbs:   codelistCRUD,
		columns: []string{"id", "code01", "sequenceNumber"},
		fields:  []field{{name: "code01", kind: kindString, required: true, help: "consent form description (--name)"}},
	},
	categoryCodelist("activityCategory"),
	categoryCodelist("documentCategory"),
	categoryCodelist("invoiceCategory"),
}
