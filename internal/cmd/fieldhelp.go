/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"
)

// fieldReference renders fields as the "Fields" section of create/update
// help, one line per field: name, kind, required marker, then the help text
// with the enum values and the codelist or entity an id points to.
func fieldReference(header string, fields []field) string {
	var b strings.Builder
	b.WriteString(header + "\n")
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, f := range fields {
		required := ""
		if f.required {
			required = "required"
		}
		help := f.help
		if len(f.values) > 0 {
			help += "; values: " + strings.Join(f.values, ", ")
		}
		if f.codelist != "" {
			help += "; codelist: " + f.codelist
		}
		if f.ref != "" {
			help += "; points to: " + f.ref
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", f.name, f.kind, required, help)
	}
	_ = w.Flush()
	// tabwriter pads the empty required column; drop the trailing spaces.
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// fieldArg is an example -f/-F flag setting f to a realistic value: its
// example, the first enum value, the "e.g." in its help, else a value of its
// kind. It is "" for objects, arrays and strings with no realistic value,
// which don't fit one flag.
func fieldArg(f field) string {
	value := fieldExampleValue(f)
	if value == "" {
		return ""
	}
	flag := "-f"
	if f.kind == kindNumber || f.kind == kindID || f.kind == kindBool {
		flag = "-F"
	}
	return flag + " " + shellQuote(f.name+"="+value)
}

// fieldExampleValue is the realistic value fieldArg sets f to, "" when none.
func fieldExampleValue(f field) string {
	value := ""
	switch {
	case f.example != "":
		value = f.example
	case len(f.values) > 0:
		value = strings.Fields(f.values[0])[0]
	case f.kind == kindString:
		value = helpExample(f.help)
	case f.kind == kindNumber, f.kind == kindID:
		value = "1"
	case f.kind == kindBool:
		value = "true"
	case f.kind == kindDate:
		value = "2026-01-31"
	case f.kind == kindDateTime:
		value = "2026-01-31 09:00"
	}
	return value
}

// helpExampleRe finds the value after "e.g." in field help: "ISO currency
// code, e.g. CZK" gives CZK.
var helpExampleRe = regexp.MustCompile(`e\.g\. ([^\s,;()\[\]{}"]+)`)

func helpExample(help string) string {
	if m := helpExampleRe.FindStringSubmatch(help); m != nil {
		return m[1]
	}
	return ""
}

// shellQuote single-quotes s when the shell would split or expand it.
func shellQuote(s string) string {
	if strings.ContainsAny(s, " \t'\"$`\\*?[]{}()<>|&;#~!") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

// withArticle prefixes noun with "a" or "an": "a company", "an offer".
func withArticle(noun string) string {
	lower := strings.ToLower(noun)
	switch {
	case strings.HasPrefix(lower, "us"), strings.HasPrefix(lower, "uni"):
		return "a " + noun
	case lower != "" && strings.ContainsRune("aeiou", rune(lower[0])):
		return "an " + noun
	}
	return "a " + noun
}

// createExample is a create invocation setting every required field that
// fits a flag.
func createExample(s *entitySpec) string {
	args := []string{"raynet", s.use, "create"}
	for _, f := range fieldsFor(s.fields, verbCreate) {
		if a := fieldArg(f); f.required && a != "" {
			args = append(args, a)
		}
	}
	return "  " + strings.Join(args, " ") + fmt.Sprintf("\n  raynet %s create --input %s.json", s.use, s.use)
}

// updateExample is an update invocation setting the first optional field
// that fits a flag.
func updateExample(s *entitySpec) string {
	ex := fmt.Sprintf("  raynet %s update 123", s.use)
	for _, f := range fieldsFor(s.fields, verbUpdate) {
		if a := fieldArg(f); !f.required && a != "" {
			ex += " " + a
			break
		}
	}
	ref := "123"
	if s.ext {
		ref = "ext:erp-17"
	}
	return ex + fmt.Sprintf("\n  raynet %s update %s --input changes.json", s.use, ref)
}
