/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/raynetcrm/raynet-cli/internal/client"
	"github.com/spf13/cobra"
)

// newCodelistCmd builds `raynet codelist`: on its own it prints the codelist
// catalog (codelistTable), with one subcommand per codelist.
func newCodelistCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "codelist",
		Short: "List codelists and manage their entries",
		Long: "Without arguments, print the name of every codelist. `codelist <name>` lists that " +
			"codelist's entries; entity fields point to them by id.",
		Example: "  raynet codelist\n  raynet codelist companyCategory",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			for _, s := range codelistTable {
				fmt.Fprintln(a.stdout, s.name)
			}
			return nil
		},
	}
	for i := range codelistTable {
		c.AddCommand(newCodelistEntryCmd(a, &codelistTable[i]))
	}
	return c
}

// newCodelistEntryCmd builds `codelist <name>` and the subcommands for the
// verbs the codelist supports; on its own it lists the entries, as `list`
// does.
func newCodelistEntryCmd(a *app, s *codelistSpec) *cobra.Command {
	c := newCodelistListCmd(a, s)
	c.Use = s.name
	c.Short = fmt.Sprintf("List %s entries (also: %s)", s.name, s.verbNames())
	c.Example = fmt.Sprintf("  raynet codelist %s\n  raynet codelist %s --json", s.name, s.name)
	for _, v := range s.verbs.each() {
		c.AddCommand(codelistVerbCmds[v](a, s))
	}
	return c
}

// codelistVerbCmds builds the command of each verb of a codelist.
var codelistVerbCmds = map[verb]func(*app, *codelistSpec) *cobra.Command{
	verbList:   newCodelistListCmd,
	verbCreate: newCodelistCreateCmd,
	verbUpdate: newCodelistUpdateCmd,
	verbDelete: newCodelistDeleteCmd,
}

func newCodelistListCmd(a *app, s *codelistSpec) *cobra.Command {
	var columns []string
	c := &cobra.Command{
		Use:     "list",
		Short:   fmt.Sprintf("List %s entries", s.name),
		Example: fmt.Sprintf("  raynet codelist %s list --columns id,code01", s.name),
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.listCodelist(s, columns)
		},
	}
	c.Flags().StringSliceVar(&columns, "columns", nil, "table columns, comma-separated")
	return c
}

// verbNames lists the codelist's verbs for help, e.g. "list, create, delete".
func (s *codelistSpec) verbNames() string {
	var names []string
	for _, v := range s.verbs.each() {
		names = append(names, v.name())
	}
	return strings.Join(names, ", ")
}

// listCodelist prints every entry of a codelist: GET /<name>/ returns them
// all at once, without paging.
func (a *app) listCodelist(s *codelistSpec, columns []string) error {
	if columns == nil {
		columns = s.columns
	}
	cl, err := a.resolveClient()
	if err != nil {
		return err
	}
	resp, err := cl.Do(http.MethodGet, "/"+s.name+"/", nil, nil)
	if err != nil {
		return err
	}
	page, err := client.DecodeListPage(resp)
	if err != nil {
		return err
	}
	records, err := decodeRecords(page.Records, "entry")
	if err != nil {
		return err
	}
	return a.printRecords(records, columns)
}

// codelistBody is the request body of codelist create/update: the shared
// body flags plus --name (code01) and, on color codelists, --color (code02),
// which win over the same keys set with -f/-F/--input.
type codelistBody struct {
	body        bodyFlags
	name, color string
}

func (b *codelistBody) register(c *cobra.Command, s *codelistSpec) {
	c.Flags().StringVar(&b.name, "name", "", "entry name (sets code01)")
	if s.color {
		c.Flags().StringVar(&b.color, "color", "", "entry color, hex without # (sets code02)")
	}
	b.body.register(c)
}

// build returns the JSON request body.
func (b *codelistBody) build(stdin io.Reader) (any, error) {
	base, err := b.body.body(stdin)
	if err != nil || (b.name == "" && b.color == "") {
		return base, err
	}
	obj, ok := base.(map[string]any)
	if !ok {
		if base != nil {
			return nil, fmt.Errorf("--input must be a JSON object when combined with --name/--color")
		}
		obj = map[string]any{}
	}
	if b.name != "" {
		obj["code01"] = b.name
	}
	if b.color != "" {
		obj["code02"] = b.color
	}
	return obj, nil
}

func newCodelistCreateCmd(a *app, s *codelistSpec) *cobra.Command {
	var body codelistBody
	name := "\"New entry\""
	for _, f := range s.fields {
		if v := fieldExampleValue(f); f.name == "code01" && v != "" {
			name = shellQuote(v)
		}
	}
	example := fmt.Sprintf("  raynet codelist %s create --name %s", s.name, name)
	flags := "--name"
	if s.color {
		example += " --color 519CFF"
		flags += "/--color"
	}
	for _, f := range s.fields {
		if f.required && f.name != "code01" {
			example += " " + fieldArg(f)
		}
	}
	c := &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create %s entry", withArticle(s.name)),
		Long: fmt.Sprintf("Create %s entry from %s and -f/-F/--input (%s wins); prints the API's "+
			"response, whose data is the new entry's id.\n\n", withArticle(s.name), flags, flags) +
			fieldReference("Fields:", s.fields),
		Example: example,
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			b, err := body.build(a.stdin)
			if err != nil {
				return err
			}
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			out, err := a.send(cl, http.MethodPut, "/"+s.name+"/", b)
			if err != nil {
				return err
			}
			if id := createdID(out); id != "" {
				a.confirm("Created %s entry %s", s.name, id)
			} else {
				a.confirm("Created %s entry", s.name)
			}
			return nil
		},
	}
	body.register(c, s)
	return c
}

// entryPath is /<name>/<id>/.
func (s *codelistSpec) entryPath(id string) string {
	return fmt.Sprintf("/%s/%s/", s.name, url.PathEscape(id))
}

func newCodelistUpdateCmd(a *app, s *codelistSpec) *cobra.Command {
	var body codelistBody
	example := fmt.Sprintf("  raynet codelist %s update 3 --name \"Renamed\"", s.name)
	c := &cobra.Command{
		Use:   "update <id>",
		Short: fmt.Sprintf("Update %s entry", withArticle(s.name)),
		Long: fmt.Sprintf("Update %s entry: only the fields sent change.\n\n", withArticle(s.name)) +
			fieldReference("Fields (required applies to create only):", s.fields),
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			b, err := body.build(a.stdin)
			if err != nil {
				return err
			}
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			if _, err := a.send(cl, http.MethodPost, s.entryPath(args[0]), b); err != nil {
				return err
			}
			a.confirm("Updated %s entry %s", s.name, args[0])
			return nil
		},
	}
	body.register(c, s)
	return c
}

func newCodelistDeleteCmd(a *app, s *codelistSpec) *cobra.Command {
	c := &cobra.Command{
		Use:     "delete <id>",
		Short:   fmt.Sprintf("Delete %s entry", withArticle(s.name)),
		Long:    fmt.Sprintf("Delete %s entry.", withArticle(s.name)),
		Example: fmt.Sprintf("  raynet codelist %s delete 3 --yes", s.name),
		Args:    cobra.ExactArgs(1),
	}
	return withDelete(a, c, func(_ *client.Client, args []string) (deletion, error) {
		return deletion{what: fmt.Sprintf("%s entry %s", s.name, args[0]), path: s.entryPath(args[0])}, nil
	})
}
