/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHelpTopics_ReachableViaHelp(t *testing.T) {
	tests := []struct {
		topic string
		want  []string
	}{
		// Operators and formats as the REST API's OpenAPI spec documents them.
		{"filtering", []string{
			"--filter", "attr[OP]=value", "Examples:\n  raynet company list --filter",
			"EQ ", "EQ_OR_NULL", "NE ", "NE_OR_NULL", "LT ", "LE ", "GT ", "GE ",
			"LIKE ", "LIKE_NOCASE", "IN ", "NOT_IN", "CUSTOM",
			"yyyy-MM-dd", "yyyy-MM-dd HH:mm", "ISO 8601", "--fulltext", "--sort",
		}},
		{"fields", []string{"-f", "-F", "@file", "null", "[]", "addresses[][address][city]=Brno", "--input -", "raynet api"}},
		{"output", []string{"--json", "--jq", "--columns", "owner.fullName", "rowInfo.createdAt", "terminal", "stderr", "tab-separated"}},
	}
	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			h := newHarness(t)

			res := h.runRaw("help", tt.topic)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			for _, w := range tt.want {
				if !strings.Contains(res.stdout, w) {
					t.Errorf("help %s has no %q:\n%s", tt.topic, w, res.stdout)
				}
			}
		})
	}
}

func TestRootHelp_OrientsTheReader(t *testing.T) {
	h := newHarness(t)

	res := h.runRaw("--help")

	if res.err != nil {
		t.Fatalf("err = %v", res.err)
	}
	for _, w := range []string{
		"company", "business-case", "phone-call", "activity", "user-account",
		"company address", "company relationship", "person relationship", "offer item",
		"codelist", "raynet api", "raynet help filtering", "raynet help fields", "raynet help output",
		"Additional help topics", "--api-key string", "--token string",
	} {
		if !strings.Contains(res.stdout, w) {
			t.Errorf("root help has no %q:\n%s", w, res.stdout)
		}
	}
}

// helpProblems walks the command tree and reports every command whose help
// falls short: no short description, no example, or a command taking a
// request body (-F) whose help has no field reference. `raynet api` is the
// one body command without a reference: it takes any endpoint's body.
func helpProblems(root *cobra.Command) []string {
	var problems []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		name := c.CommandPath()
		if strings.TrimSpace(c.Short) == "" {
			problems = append(problems, name+": no short description")
		}
		if strings.TrimSpace(c.Example) == "" {
			problems = append(problems, name+": no example")
		}
		if c.Flags().Lookup("field") != nil && c.Name() != "api" && !fieldReferenceLine.MatchString(c.Long) {
			problems = append(problems, name+": no field reference")
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	return problems
}

// fieldReferenceLine matches a "Fields...:" header followed by a field line.
var fieldReferenceLine = regexp.MustCompile(`(?m)^Fields.*:\n  \S+ +(string|number|bool|date|datetime|id|object|array)\b`)

func TestHelpQuality_EveryCommand(t *testing.T) {
	root := newRootCmd(newApp())

	for _, p := range helpProblems(root) {
		t.Error(p)
	}
}

func TestHelpQuality_CatchesMissingHelp(t *testing.T) {
	root := newRootCmd(newApp())
	find := func(args ...string) *cobra.Command {
		c, _, err := root.Find(args)
		if err != nil {
			t.Fatalf("find %v: %v", args, err)
		}
		return c
	}
	find("company", "lock").Short = ""
	find("offer", "item", "add").Example = ""
	find("lead", "create").Long = "Create a lead record."
	find("codelist", "currency", "update").Long = "Update a currency entry."

	got := helpProblems(root)

	want := []string{
		"raynet company lock: no short description",
		"raynet offer item add: no example",
		"raynet lead create: no field reference",
		"raynet codelist currency update: no field reference",
	}
	if !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
		t.Errorf("problems = %q, want %q", got, want)
	}
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

// Create examples come from the field reference: every required field that
// fits one flag appears in the example, with a realistic value where the
// reference gives one.
func TestCreateExamples_SetRequiredFieldsFromFieldReference(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"company", "create"}, []string{"raynet company create", "-f 'name=Acme Ltd'", "-f state=A_POTENTIAL", "-f rating=A", "-f role=A_SUBSCRIBER"}},
		{[]string{"invoice", "create"}, []string{"-F company=", "-F currency=", "-f issueDate=2026-01-31", "-f taxPayer=YES"}},
		{[]string{"task", "create"}, []string{"-f 'title=Follow-up call'", "-f 'deadline=2026-01-31 09:00'", "-F owner="}},
		{[]string{"codelist", "currency", "create"}, []string{"-f code02=CZK"}},
		{[]string{"codelist", "taxRate", "create"}, []string{"--name 21%"}},
		{[]string{"person", "relationship", "add"}, []string{"-F company=", "-f type=CEO"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			h := newHarness(t)

			res := h.runRaw(append(tt.args, "--help")...)

			if res.err != nil {
				t.Fatalf("err = %v", res.err)
			}
			_, ex, found := strings.Cut(res.stdout, "Examples:")
			if !found {
				t.Fatalf("help has no examples:\n%s", res.stdout)
			}
			for _, w := range tt.want {
				if !strings.Contains(ex, w) {
					t.Errorf("examples have no %q:\n%s", w, ex)
				}
			}
			if strings.Contains(ex, "=text") {
				t.Errorf("examples use a placeholder value:\n%s", ex)
			}
		})
	}
}

// Help says "an offer", "an email", "an invoice": the article fits the noun.
func TestHelp_ArticleFitsTheNoun(t *testing.T) {
	root := newRootCmd(newApp())
	bad := regexp.MustCompile(`\ba (?:[aeioAEIO]|[uU][^sSnN])\w*|\ban [^aeiouAEIOU\s]\w*`)
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, text := range []string{c.Short, c.Long} {
			if m := bad.FindString(text); m != "" {
				t.Errorf("%s: %q in %q", c.CommandPath(), m, text)
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}

// Every required field that fits one flag shows up in the generated create
// and add examples, so a copied example carries a complete body.
func TestCreateExamples_CoverEveryRequiredFlagField(t *testing.T) {
	root := newRootCmd(newApp())
	check := func(path []string, fields []field) {
		c, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		for _, f := range fieldsFor(fields, verbCreate) {
			if !f.required || f.kind == kindObject || f.kind == kindArray {
				continue
			}
			if !regexp.MustCompile(`-[fF] '?` + regexp.QuoteMeta(f.name) + `=`).MatchString(c.Example) {
				t.Errorf("%s example has no required field %s:\n%s", c.CommandPath(), f.name, c.Example)
			}
		}
	}
	for i := range entityTable {
		s := &entityTable[i]
		if s.verbs&verbCreate != 0 {
			check([]string{s.use, "create"}, s.fields)
		}
		for _, n := range s.nested {
			if n.verbs&verbCreate != 0 {
				check([]string{s.use, n.use, "add"}, n.fields)
			}
		}
	}
}
