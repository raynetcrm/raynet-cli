/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import "github.com/spf13/cobra"

// verb is a bitset of the standard commands an entity supports; see
// entitySpec.verbs.
type verb uint

const (
	verbList verb = 1 << iota
	verbView
	verbCreate
	verbUpdate
	verbDelete
)

const verbsCRUD = verbList | verbView | verbCreate | verbUpdate | verbDelete

// verbTable is every verb in command order, with its command name (a nested
// collection names verbCreate "add" instead).
var verbTable = []struct {
	v    verb
	name string
}{{verbList, "list"}, {verbView, "view"}, {verbCreate, "create"}, {verbUpdate, "update"}, {verbDelete, "delete"}}

// each returns the verbs in the set v, in verbTable order.
func (v verb) each() []verb {
	var out []verb
	for _, t := range verbTable {
		if v&t.v != 0 {
			out = append(out, t.v)
		}
	}
	return out
}

// name is the command name of the single verb v.
func (v verb) name() string {
	for _, t := range verbTable {
		if t.v == v {
			return t.name
		}
	}
	return ""
}

// entitySpec is one row of the entity table (entities.go): everything
// newEntityCmd needs to build an entity's command, its subcommands and help.
type entitySpec struct {
	use     string   // command name, kebab-case: "business-case"
	aliases []string // the API's camelCase name plus a short alias
	short   string
	path    string // REST API path segment, no slashes: "businessCase"
	webName string // the web app's entity name for --web links: "BusinessCase"
	columns []string
	verbs   verb
	// ext reports whether `view ext:<extId>` works (GET /<path>/ext/<extId>/).
	ext bool
	// withItems: create switches to PUT /<path>/createWithItems when the
	// body carries "items".
	withItems bool
	// listFilters is a one-line hint of useful --filter attributes for
	// `list --help`.
	listFilters string
	// typeColumn: list rows mix entities (the activity list), so table output
	// gains a "type" column, the command of the row whose webName equals the
	// record's _entityName.
	typeColumn bool
	fields     []field
	actions    []action
	nested     []nestedSpec
	// commands builds hand-written subcommands that don't fit the table,
	// such as `user-account me`.
	commands func(a *app) []*cobra.Command
}

// fieldKind is the JSON type of a body field, as shown in help and as -F
// would need to send it.
type fieldKind string

const (
	kindString   fieldKind = "string"
	kindNumber   fieldKind = "number"
	kindBool     fieldKind = "bool"
	kindDate     fieldKind = "date"     // yyyy-MM-dd
	kindDateTime fieldKind = "datetime" // yyyy-MM-dd HH:mm
	kindID       fieldKind = "id"       // numeric id of another record or codelist entry
	kindObject   fieldKind = "object"
	kindArray    fieldKind = "array"
)

// field documents one request-body field for create/update help. It is
// reference only: the CLI never validates bodies against it, the API does.
type field struct {
	name     string
	kind     fieldKind
	required bool     // required on create
	values   []string // allowed enum values
	codelist string   // codelist name the id points into, e.g. "companyCategory"
	ref      string   // entity command the id points to, e.g. "company"
	help     string
	// example is a realistic value for generated examples, where neither
	// values nor an "e.g." in help gives one.
	example string
	// only limits the field to the create or the update body (verbCreate or
	// verbUpdate); zero means both.
	only verb
}

// fieldsFor is the part of fields that the body of v (verbCreate or
// verbUpdate) takes.
func fieldsFor(fields []field, v verb) []field {
	var out []field
	for _, f := range fields {
		if f.only == 0 || f.only == v {
			out = append(out, f)
		}
	}
	return out
}

// action is a record-level command beyond the standard verbs: POST/PUT/DELETE
// on /<path>/<id>/<suffix>. Rows list them in their actions field; the
// common ones are predefined in entity_action.go (actLock, actTag, ...).
type action struct {
	use    string // "lock"
	short  string // help line; "%s" is replaced with the entity command name and its article: "an offer"
	done   string // past tense for the terminal confirmation: "Locked"
	method string
	suffix string // appended after "/<path>/<id>/", e.g. "lock" or "convert/"
	// tags: the remaining positional args are tag names, sent as
	// {"tag": "a,b"}.
	tags bool
	// body: the action accepts -f/-F/--input.
	body bool
	// fields is the body's field reference for the action's help.
	fields []field
}

// nestedSpec is a child collection of a record (line items, addresses,
// relationships) exposed as `<entity> <use> list|add|update|delete <id> ...`.
type nestedSpec struct {
	use     string // "item"
	plural  string // "items", for help and confirmations
	short   string
	segment string // path segment under the record: "item"
	// viewField, when set, makes `list` read this array from the parent's
	// view response instead of GET /<path>/<id>/<segment>/.
	viewField string
	columns   []string
	verbs     verb // verbList, verbCreate (add: PUT), verbUpdate (POST), verbDelete
	// bulkDelete: `delete` with more than one child id sends
	// DELETE /<path>/<id>/<segment>/ with {"ids": [...]}.
	bulkDelete bool
	// updateRequired: update must resend the required fields too, as the
	// API marks them required on edit as well.
	updateRequired bool
	fields         []field  // add/update body reference; field.only applies (verbCreate = add)
	actions        []action // per-child actions: POST .../<segment>/<childId>/<suffix>
}
