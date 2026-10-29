/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// printJSON pretty-prints v. When --jq is given it always pipes through that
// expression; otherwise, in an interactive terminal with `jq` on PATH, it
// defaults to `jq .` purely for syntax highlighting. Non-terminal output (or
// no jq installed) falls back to plain indented JSON so piped/scripted use is
// unaffected.
func (a *app) printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding output: %w", err)
	}

	expr := a.flags.jq
	if expr == "" && a.isTerminal() {
		if _, err := exec.LookPath("jq"); err == nil {
			expr = "."
		}
	}
	if expr == "" {
		_, err := fmt.Fprintln(a.stdout, string(data))
		return err
	}
	return a.runJQ(data, expr)
}

func (a *app) runJQ(input []byte, expr string) error {
	jqPath, err := exec.LookPath("jq")
	if err != nil {
		return fmt.Errorf("--jq requires the `jq` binary on PATH: %w", err)
	}
	c := exec.Command(jqPath, expr)
	c.Stdin = bytes.NewReader(input)
	c.Stdout = a.stdout
	c.Stderr = a.stderr
	return c.Run()
}

// printRecords renders a list of JSON records either as raw JSON (--json /
// --jq) or as a table of the given columns.
func (a *app) printRecords(records []map[string]any, columns []string) error {
	if a.flags.json || a.flags.jq != "" {
		return a.printJSON(records)
	}
	if len(records) == 0 {
		fmt.Fprintln(a.stdout, "No records found.")
		return nil
	}
	a.printTable(records, columns)
	return nil
}

// printTable renders records as a table. On an interactive terminal (and
// unless NO_COLOR is set) it uses a bordered, colored lipgloss table (glab
// style); when piped or redirected it falls back to plain tab-separated
// columns so plain-text tools like grep/awk keep working.
func (a *app) printTable(records []map[string]any, columns []string) {
	if a.isTerminal() && os.Getenv("NO_COLOR") == "" {
		fmt.Fprintln(a.stdout, renderStyledTable(records, columns))
		return
	}
	printPlainTable(a.stdout, records, columns)
}

// printPlainTable renders records as plain tab-separated columns: no
// borders, no colors, so piped/scripted use stays machine-readable.
func printPlainTable(w io.Writer, records []map[string]any, columns []string) {
	header := make([]string, len(columns))
	copy(header, columns)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, rec := range records {
		row := make([]string, len(columns))
		for i, col := range columns {
			row[i] = columnValue(rec, col)
		}
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
}

// columnValue renders the value at a column path in rec. Dots in path reach
// into nested objects ("owner.fullName"), but a literal key containing a dot
// wins over nesting ("rowInfo.createdAt" is a flat key in list rows). Missing
// values and null render empty; objects and arrays render as compact JSON.
func columnValue(rec map[string]any, path string) string {
	v, ok := lookupPath(rec, path)
	if !ok || v == nil {
		return ""
	}
	switch v := v.(type) {
	case string:
		return v
	case map[string]any, []any:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	default:
		return fmt.Sprint(v)
	}
}

// lookupPath finds the value at a dotted path in rec, preferring the longest
// literal key at each level.
func lookupPath(rec map[string]any, path string) (any, bool) {
	if v, ok := rec[path]; ok {
		return v, true
	}
	for i := strings.LastIndex(path, "."); i > 0; i = strings.LastIndex(path[:i], ".") {
		nested, ok := rec[path[:i]].(map[string]any)
		if !ok {
			continue
		}
		if v, ok := lookupPath(nested, path[i+1:]); ok {
			return v, true
		}
	}
	return nil, false
}

// renderStyledTable renders records as a bordered lipgloss table with a bold
// header row.
func renderStyledTable(records []map[string]any, columns []string) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("21"))
	cellStyle := lipgloss.NewStyle()

	t := table.New().
		Headers(columns...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	for _, rec := range records {
		row := make([]string, len(columns))
		for i, col := range columns {
			row[i] = columnValue(rec, col)
		}
		t.Row(row...)
	}
	return t.Render()
}
