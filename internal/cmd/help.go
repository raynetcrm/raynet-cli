/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// filterOperators are the list-filter operators, written as attr[OP]=value;
// the complete set the REST API's OpenAPI spec documents.
var filterOperators = []struct{ op, help string }{
	{"EQ", "equal (the default when [OP] is left out); an empty value matches an empty attribute"},
	{"EQ_OR_NULL", "equal or empty"},
	{"NE", "not equal; an empty value matches a non-empty attribute"},
	{"NE_OR_NULL", "not equal or empty"},
	{"LT", "less than"},
	{"LE", "less than or equal"},
	{"GT", "greater than"},
	{"GE", "greater than or equal"},
	{"LIKE", "pattern match with %, e.g. Acme% (starts with Acme)"},
	{"LIKE_NOCASE", "LIKE, case-insensitive"},
	{"IN", "equal to one of comma-separated values, e.g. 1,2,3"},
	{"NOT_IN", "equal to none of comma-separated values"},
	{"CUSTOM", "special per-attribute filter, e.g. productCategory[CUSTOM]=1,2 on business documents"},
}

// filterDateFormats is how filter (and body) values carry dates.
const filterDateFormats = `Dates are yyyy-MM-dd. Date-times are yyyy-MM-dd HH:mm in the API user's time zone,
or ISO 8601 (2026-01-31T09:00:00.000+01:00). Quote values containing a space.`

// helpTopics are the `raynet help <topic>` pages: commands without a Run,
// which cobra lists under "Additional help topics".
func helpTopics() []*cobra.Command {
	topics := []*cobra.Command{
		{
			Use:   "filtering",
			Short: "Filter, search, sort and page lists: --filter operators and date formats",
			Long:  filteringHelp(),
			Example: "  raynet company list --filter 'name[LIKE_NOCASE]=acme%'\n" +
				"  raynet company list --filter 'rowInfo.createdAt[GE]=2026-01-01 00:00' --filter rating=A\n" +
				"  raynet offer list --filter 'id[IN]=12,15,19' --sort -validTill\n" +
				"  raynet company list --filter 'regNumber[NE]=' --all --json",
		},
		{
			Use:   "fields",
			Short: "Build request bodies: -f, -F, bracket nesting and --input",
			Long:  fieldsHelp,
			Example: "  raynet company create -f name=Acme -f rating=A -f state=A_POTENTIAL -f role=A_SUBSCRIBER -F owner=7\n" +
				"  raynet company create -f name=Acme -f 'addresses[][address][name]=HQ' -f 'addresses[][address][city]=Brno'\n" +
				"  raynet task update 42 -f status=COMPLETED -f 'deadline=2026-01-31 17:00'\n" +
				"  raynet company update 42 -F category=null -F person=false\n" +
				"  jq '.name = \"Acme\"' base.json | raynet company create --input - -F owner=7",
		},
		{
			Use:   "output",
			Short: "Tables, --columns, --json and --jq, and how output differs outside a terminal",
			Long:  outputHelp,
			Example: "  raynet company list --columns id,name,owner.fullName,rowInfo.createdAt\n" +
				"  raynet company view 42 --jq .data.name\n" +
				"  raynet company create -f name=Acme -f rating=A -f state=A_POTENTIAL -f role=A_SUBSCRIBER --jq .data.id\n" +
				"  raynet offer list --all --json > offers.json",
		},
	}
	// cobra prints only Long for a help topic, so the examples go there too.
	for _, t := range topics {
		t.Long += "\n\nExamples:\n" + t.Example
	}
	return topics
}

func filteringHelp() string {
	var b strings.Builder
	b.WriteString(`Every list command takes:

  --filter attr[OP]=value   filter on an attribute; repeatable, filters combine with AND,
                            the same attribute may repeat with different operators
  --fulltext text           search the way the CRM search box does
  --sort column, --sort -column
                            sort ascending, or descending with a leading -
  --limit n, --offset n     lists return 30 records; --limit sets how many (1 to 1000),
                            --offset where to start
  --all                     fetch every page

Operators (attr[OP]=value; attr=value means EQ):
`)
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, o := range filterOperators {
		fmt.Fprintf(w, "  %s\t%s\n", o.op, o.help)
	}
	_ = w.Flush()
	b.WriteString(`
Each attribute allows only some operators; an unsupported one fails with 400 "Unsupported
filter column". "<entity> list --help" names useful attributes. Reference attributes are
spelled per entity (owner, owner-id, primaryAddress-address.city): copy them from that help.
tags=vip,hot matches records with any of the tags. Test for an empty or non-empty value with
an empty EQ or NE value: --filter 'regNumber[NE]='.

`)
	b.WriteString(filterDateFormats)
	return b.String()
}

const fieldsHelp = `create, update, record actions with a body, nested add/update and "raynet api" build the
JSON request body the same way:

  -f key=value    string field
  -F key=value    typed field: a number, true/false, null, or @file (the file's contents)
  --input file    a JSON body from a file, or "--input -" for stdin; -f/-F apply on top of it

Send ids, numbers and booleans with -F so they reach the API as JSON numbers and booleans:
it stores false for the string "true". Text, enum values, dates and date-times go with -f.

Keys nest with brackets: key[sub] sets an object field, key[] appends to an array, and
key[][sub] sets a field of the array's last element, or starts a new element once that
field is already set:

  -f 'addresses[][address][city]=Brno'   {"addresses":[{"address":{"city":"Brno"}}]}
  -F 'items[][product]=5' -F 'items[][count]=2'

"<entity> create --help" and "update --help" list the entity's key fields: type, required
marker, allowed values, and the codelist or entity an id points to ("raynet codelist <name>"
lists codelist entry ids). The API, not the CLI, validates the body.

In "raynet api", GET and DELETE send -f/-F as query parameters instead.`

const outputHelp = `Lists print a table of the entity's default columns. --columns picks others, comma-separated;
a dotted path reaches into nested objects (owner.fullName, company.name), and flat API keys
that contain a dot (rowInfo.createdAt, rowInfo.updatedBy) work as written.

view prints the record as JSON. create, update, delete and actions print the API's JSON
response, so --jq .data.id gives a new record's id.

  --json        print the raw JSON records instead of a table
  --jq expr     filter the JSON through a jq expression (needs jq on PATH); implies JSON

In a terminal, tables are styled, JSON is colored, and stderr also gets short notes: "30 of
319" after a list, "Created company 123" after a mutation. Outside a terminal (a pipe, a
script, an agent) tables are plain tab-separated columns, JSON is plain, those notes are left
out, and delete refuses to run without --yes. Output is never switched to JSON
automatically: pass --json for machine-readable output.

Errors go to stderr as "Error: <status> <message>", one indented "field: message" line per
field error, and the exit code is non-zero. With --json the error is printed as JSON on
stderr instead. --verbose adds each request line, the status and the raw error body.

Date-times are yyyy-MM-dd HH:mm in the API user's time zone; on a list, --filter
dateFormat=ISO8601 returns ISO 8601 instead.`

// rootLong orients a reader (often an AI agent) in `raynet --help`: what the
// commands are, generated from the entity spec table, and where to read on.
func rootLong() string {
	var entities, nested []string
	for i := range entityTable {
		s := &entityTable[i]
		entities = append(entities, s.use)
		for _, n := range s.nested {
			nested = append(nested, s.use+" "+n.use)
		}
	}
	return `Command-line client for the RAYNET CRM REST API.

Entities: raynet <entity> list|view|create|update|delete <id>, plus actions such as lock or tag:
  ` + wrapList(entities, 90, "  ") + `
  activity lists every activity type together; user-account also has "user-account me".
  Record arguments take a numeric id, or ext:<extId> on entities with external ids.

Nested collections: raynet <entity> <collection> list|add|update|delete <id> ...:
  ` + wrapList(nested, 90, "  ") + `

Codelists: "raynet codelist" names them; "raynet codelist <name>" lists entry ids.

Anything else: "raynet api <path>" calls any REST API endpoint with the same body flags.

Help topics, worth reading before building a request:
  raynet help filtering   --filter operators, --fulltext, --sort, paging, date formats
  raynet help fields      -f/-F body fields, bracket nesting, --input
  raynet help output      tables, --columns, --json/--jq, terminal vs. pipe output

Start with "raynet auth login", then "raynet <entity> <command> --help": create and update
help lists the entity's fields.`
}

// wrapList joins items with ", ", wrapping lines at width and indenting
// continuation lines.
func wrapList(items []string, width int, indent string) string {
	var b strings.Builder
	line := 0
	for i, it := range items {
		sep := ""
		if i > 0 {
			sep = ", "
		}
		if line > 0 && line+len(sep)+len(it) > width {
			b.WriteString(",\n" + indent)
			line, sep = 0, ""
		}
		b.WriteString(sep + it)
		line += len(sep) + len(it)
	}
	return b.String()
}
