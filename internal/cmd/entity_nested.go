/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/raynetcrm/raynet-cli/internal/client"
	"github.com/spf13/cobra"
)

// newNestedCmd builds `<entity> <nested> list|add|update|delete <ref> ...`
// plus the per-child actions for one of a row's nested collections. The
// parent reference accepts ext:<extId> like every record-level command.
func newNestedCmd(a *app, s *entitySpec, n *nestedSpec) *cobra.Command {
	c := &cobra.Command{
		Use:   n.use,
		Short: n.short,
	}
	for _, v := range n.verbs.each() {
		c.AddCommand(nestedVerbCmds[v](a, s, n))
	}
	for _, act := range n.actions {
		c.AddCommand(newNestedActionCmd(a, s, n, act))
	}
	// The collection's example is the first example of each subcommand.
	var lines []string
	for _, sub := range c.Commands() {
		first, _, _ := strings.Cut(sub.Example, "\n")
		lines = append(lines, first)
	}
	c.Example = strings.Join(lines, "\n")
	return c
}

// nestedVerbCmds builds the command of each verb of a nested collection.
var nestedVerbCmds = map[verb]func(*app, *entitySpec, *nestedSpec) *cobra.Command{
	verbList:   newNestedListCmd,
	verbCreate: newNestedAddCmd,
	verbUpdate: newNestedUpdateCmd,
	verbDelete: newNestedDeleteCmd,
}

func newNestedListCmd(a *app, s *entitySpec, n *nestedSpec) *cobra.Command {
	var columns []string
	c := &cobra.Command{
		Use:     "list " + s.refArg(),
		Short:   fmt.Sprintf("List the %s of %s record", n.plural, withArticle(s.use)),
		Example: fmt.Sprintf("  raynet %s %s list 123", s.use, n.use) + extExample(s, n.use+" list", ""),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if columns == nil {
				columns = n.columns
			}
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			records, err := a.nestedRecords(cl, s, n, parseRecordRef(args[0]))
			if err != nil {
				return err
			}
			return a.printRecords(records, columns)
		},
	}
	c.Flags().StringSliceVar(&columns, "columns", nil, "table columns, comma-separated; dotted paths reach nested fields")
	return c
}

func newNestedAddCmd(a *app, s *entitySpec, n *nestedSpec) *cobra.Command {
	var body bodyFlags
	fields := fieldsFor(n.fields, verbCreate)
	ex := []string{"raynet", s.use, n.use, "add", "123"}
	for _, f := range fields {
		if arg := fieldArg(f); f.required && arg != "" {
			ex = append(ex, arg)
		}
	}
	c := &cobra.Command{
		Use:   "add " + s.refArg(),
		Short: fmt.Sprintf("Add a new %s to %s record", n.use, withArticle(s.use)),
		Long: fmt.Sprintf("Add a new %s to %s record from -f/-F fields and --input; prints the API's response.\n\n", n.use, withArticle(s.use)) +
			fieldReference("Fields:", fields),
		Example: "  " + strings.Join(ex, " ") + fmt.Sprintf("\n  raynet %s %s add 123 --input %s.json", s.use, n.use, n.use),
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			b, err := body.body(a.stdin)
			if err != nil {
				return err
			}
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			id, err := a.recordID(cl, s, parseRecordRef(args[0]))
			if err != nil {
				return err
			}
			out, err := a.send(cl, http.MethodPut, s.nestedPath(n, id), b)
			if err != nil {
				return err
			}
			if childID := createdID(out); childID != "" {
				a.confirm("Added %s %s to %s %s", n.use, childID, s.use, id)
			} else {
				a.confirm("Added %s to %s %s", n.use, s.use, id)
			}
			return nil
		},
	}
	body.register(c)
	return c
}

func newNestedUpdateCmd(a *app, s *entitySpec, n *nestedSpec) *cobra.Command {
	var body bodyFlags
	fields := fieldsFor(n.fields, verbUpdate)
	ex := fmt.Sprintf("  raynet %s %s update 123 5", s.use, n.use)
	header := "Fields (required applies to add only):"
	if n.updateRequired {
		header = "Fields (required applies to update too):"
		for _, f := range fields {
			if arg := fieldArg(f); f.required && arg != "" {
				ex += " " + arg
			}
		}
	} else {
		for _, f := range fields {
			if arg := fieldArg(f); !f.required && arg != "" {
				ex += " " + arg
				break
			}
		}
	}
	c := &cobra.Command{
		Use:   fmt.Sprintf("update %s <%sId>", s.refArg(), n.use),
		Short: fmt.Sprintf("Update one %s of %s record", n.use, withArticle(s.use)),
		Long: fmt.Sprintf("Update one %s of %s record: only the fields sent change.\n\n", n.use, withArticle(s.use)) +
			fieldReference(header, fields),
		Example: ex + extExample(s, n.use+" update", " 5 --input changes.json"),
		Args:    cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			b, err := body.body(a.stdin)
			if err != nil {
				return err
			}
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			id, err := a.recordID(cl, s, parseRecordRef(args[0]))
			if err != nil {
				return err
			}
			if _, err := a.send(cl, http.MethodPost, s.childPath(n, id, args[1]), b); err != nil {
				return err
			}
			a.confirm("Updated %s %s of %s %s", n.use, args[1], s.use, id)
			return nil
		},
	}
	body.register(c)
	return c
}

func newNestedDeleteCmd(a *app, s *entitySpec, n *nestedSpec) *cobra.Command {
	use := fmt.Sprintf("delete %s <%sId>", s.refArg(), n.use)
	args := cobra.ExactArgs(2)
	short := fmt.Sprintf("Delete one %s of %s record", n.use, withArticle(s.use))
	long := short + "."
	example := fmt.Sprintf("  raynet %s %s delete 123 5 --yes", s.use, n.use)
	if n.bulkDelete {
		use += "..."
		args = cobra.MatchAll(cobra.MinimumNArgs(2), func(_ *cobra.Command, args []string) error {
			_, err := n.bulkIDs(args[1:])
			return err
		})
		short = fmt.Sprintf("Delete %s of %s record", n.plural, withArticle(s.use))
		long = short + "; several ids are deleted in one request."
		example += fmt.Sprintf("\n  raynet %s %s delete 123 5 6 7", s.use, n.use)
	}
	c := &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    args,
	}
	return withDelete(a, c, func(cl *client.Client, args []string) (deletion, error) {
		id, err := a.recordID(cl, s, parseRecordRef(args[0]))
		if err != nil {
			return deletion{}, err
		}
		childIDs := args[1:]
		if len(childIDs) == 1 {
			return deletion{
				what: fmt.Sprintf("%s %s from %s %s", n.use, childIDs[0], s.use, id),
				path: s.childPath(n, id, childIDs[0]),
			}, nil
		}
		ids, err := n.bulkIDs(childIDs)
		if err != nil {
			return deletion{}, err
		}
		return deletion{
			what: fmt.Sprintf("%s %s from %s %s", n.plural, strings.Join(childIDs, ", "), s.use, id),
			path: s.nestedPath(n, id),
			body: map[string]any{"ids": ids},
		}, nil
	})
}

// bulkIDs parses the child ids of a bulk delete; a single id is not a bulk
// delete and is passed through unparsed (nil).
func (n *nestedSpec) bulkIDs(childIDs []string) ([]int64, error) {
	if len(childIDs) < 2 {
		return nil, nil
	}
	ids := make([]int64, len(childIDs))
	for i, raw := range childIDs {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s id %q, expected a number", n.use, raw)
		}
		ids[i] = id
	}
	return ids, nil
}

// newNestedActionCmd builds `<entity> <nested> <action> <ref> <childId>`:
// act.method on /<path>/<id>/<segment>/<childId>/<suffix>.
func newNestedActionCmd(a *app, s *entitySpec, n *nestedSpec, act action) *cobra.Command {
	return &cobra.Command{
		Use:     fmt.Sprintf("%s %s <%sId>", act.use, s.refArg(), n.use),
		Short:   fmt.Sprintf(act.short, withArticle(s.use)),
		Example: fmt.Sprintf("  raynet %s %s %s 123 5", s.use, n.use, act.use) + extExample(s, n.use+" "+act.use, " 5"),
		Args:    cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			id, err := a.recordID(cl, s, parseRecordRef(args[0]))
			if err != nil {
				return err
			}
			if _, err := a.send(cl, act.method, s.childPath(n, id, args[1])+act.suffix, nil); err != nil {
				return err
			}
			a.confirm("%s %s %s of %s %s", act.done, n.use, args[1], s.use, id)
			return nil
		},
	}
}

// nestedRecords fetches the children of the record ref names: the
// viewField array of the parent's view response, or the collection's own
// list endpoint.
func (a *app) nestedRecords(cl *client.Client, s *entitySpec, n *nestedSpec, ref recordRef) ([]map[string]any, error) {
	var raw []json.RawMessage
	if n.viewField != "" {
		path, err := s.viewPath(ref)
		if err != nil {
			return nil, err
		}
		data, err := getData(cl, path)
		if err != nil {
			return nil, err
		}
		var parent map[string]json.RawMessage
		if err := json.Unmarshal(data, &parent); err != nil {
			return nil, fmt.Errorf("decoding %s %s: %w", s.use, ref, err)
		}
		if items := parent[n.viewField]; len(items) > 0 && string(items) != "null" {
			if err := json.Unmarshal(items, &raw); err != nil {
				return nil, fmt.Errorf("decoding %s %s %s: %w", s.use, ref, n.viewField, err)
			}
		}
	} else {
		id, err := a.recordID(cl, s, ref)
		if err != nil {
			return nil, err
		}
		resp, err := cl.Do(http.MethodGet, s.nestedPath(n, id), nil, nil)
		if err != nil {
			return nil, err
		}
		if raw, err = client.DecodeListResponse(resp); err != nil {
			return nil, err
		}
	}
	return decodeRecords(raw, n.use)
}

// nestedPath is the collection path /<path>/<id>/<segment>/.
func (s *entitySpec) nestedPath(n *nestedSpec, id string) string {
	return s.recordPath(id) + n.segment + "/"
}

// childPath is /<path>/<id>/<segment>/<childID>/.
func (s *entitySpec) childPath(n *nestedSpec, id, childID string) string {
	return s.nestedPath(n, id) + url.PathEscape(childID) + "/"
}

// extExample is an extra example line using an ext: reference, on entities
// with external id lookup; "" otherwise.
func extExample(s *entitySpec, sub, tail string) string {
	if !s.ext {
		return ""
	}
	return fmt.Sprintf("\n  raynet %s %s ext:erp-17%s", s.use, sub, tail)
}
