/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/raynetcrm/raynet-cli/internal/client"
	"github.com/spf13/cobra"
)

func newCreateCmd(a *app, s *entitySpec) *cobra.Command {
	var body bodyFlags
	c := &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("Create %s record", withArticle(s.use)),
		Long: fmt.Sprintf("Create %s record from -f/-F fields and --input; prints the API's response, "+
			"whose data.id is the new record's id. See \"raynet help fields\".\n\n", withArticle(s.use)) +
			fieldReference("Fields:", fieldsFor(s.fields, verbCreate)),
		Example: createExample(s),
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			b, err := body.body(a.stdin)
			if err != nil {
				return err
			}
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			out, err := a.send(cl, http.MethodPut, s.createPath(b), b)
			if err != nil {
				return err
			}
			if id := createdID(out); id != "" {
				a.confirm("Created %s %s", s.use, id)
			} else {
				a.confirm("Created %s", s.use)
			}
			return nil
		},
	}
	body.register(c)
	return c
}

func newUpdateCmd(a *app, s *entitySpec) *cobra.Command {
	var body bodyFlags
	c := &cobra.Command{
		Use:   "update " + s.refArg(),
		Short: fmt.Sprintf("Update %s record", withArticle(s.use)),
		Long: fmt.Sprintf("Update %s record: only the fields sent change.\n\n", withArticle(s.use)) +
			fieldReference("Fields (required applies to create only):", fieldsFor(s.fields, verbUpdate)),
		Example: updateExample(s),
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
			if _, err := a.send(cl, http.MethodPost, s.recordPath(id), b); err != nil {
				return err
			}
			a.confirm("Updated %s %s", s.use, id)
			return nil
		},
	}
	body.register(c)
	return c
}

func newDeleteCmd(a *app, s *entitySpec) *cobra.Command {
	c := &cobra.Command{
		Use:     "delete " + s.refArg(),
		Short:   fmt.Sprintf("Delete %s record", withArticle(s.use)),
		Long:    fmt.Sprintf("Delete %s record.", withArticle(s.use)),
		Example: fmt.Sprintf("  raynet %s delete 123 --yes", s.use) + extExample(s, "delete", ""),
		Args:    cobra.ExactArgs(1),
	}
	return withDelete(a, c, func(cl *client.Client, args []string) (deletion, error) {
		id, err := a.recordID(cl, s, parseRecordRef(args[0]))
		if err != nil {
			return deletion{}, err
		}
		return deletion{what: fmt.Sprintf("%s %s", s.use, id), path: s.recordPath(id)}, nil
	})
}

// deletion is what a delete command removes: DELETE path with body,
// described as what ("company 123") in the question and confirmation.
type deletion struct {
	what string
	path string
	body any
}

// withDelete completes c (Use, Short, Long, Example and Args set) as a
// delete command: it adds --yes and the confirmation sentence to Long, and
// runs requireYes, then target to resolve the arguments, then asks in a
// terminal before sending the DELETE.
func withDelete(a *app, c *cobra.Command, target func(cl *client.Client, args []string) (deletion, error)) *cobra.Command {
	var yes bool
	c.Long += " In a terminal it asks for confirmation first; without a terminal it refuses unless --yes is given."
	c.RunE = func(_ *cobra.Command, args []string) error {
		if err := a.requireYes(yes); err != nil {
			return err
		}
		cl, err := a.resolveClient()
		if err != nil {
			return err
		}
		d, err := target(cl, args)
		if err != nil {
			return err
		}
		if !yes && !a.ask(fmt.Sprintf("Delete %s?", d.what)) {
			fmt.Fprintln(a.stderr, "Cancelled.")
			return nil
		}
		if _, err := a.send(cl, http.MethodDelete, d.path, d.body); err != nil {
			return err
		}
		a.confirm("Deleted %s", d.what)
		return nil
	}
	c.Flags().BoolVarP(&yes, "yes", "y", false, "delete without asking for confirmation")
	return c
}

// requireYes guards a destructive command before it touches the API:
// without a terminal to ask in, the command must be confirmed with --yes.
func (a *app) requireYes(yes bool) error {
	if !yes && !a.isTerminal() {
		return fmt.Errorf("refusing to delete without confirmation: pass --yes when not running in a terminal")
	}
	return nil
}

// ask prints a y/N question on stderr and reads the answer from stdin; only
// "y" or "yes" (any case) agree.
func (a *app) ask(question string) bool {
	fmt.Fprintf(a.stderr, "%s [y/N] ", question)
	answer, _ := bufio.NewReader(a.stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	}
	return false
}

// refArg is the record argument in a command's usage line.
func (s *entitySpec) refArg() string {
	if s.ext {
		return "<id | ext:extId>"
	}
	return "<id>"
}

// recordPath is /<path>/<id>/.
func (s *entitySpec) recordPath(id string) string {
	return fmt.Sprintf("/%s/%s/", s.path, url.PathEscape(id))
}

// recordID turns a record reference into a numeric id: a plain id is
// returned as is, an external id is resolved via GET /<path>/ext/<extId>/.
// Every record-level command except view (which GETs the ext path directly)
// resolves its reference through here.
func (a *app) recordID(c *client.Client, s *entitySpec, ref recordRef) (string, error) {
	if !ref.ext {
		return ref.value, nil
	}
	path, err := s.viewPath(ref)
	if err != nil {
		return "", err
	}
	data, err := getData(c, path)
	if err != nil {
		return "", err
	}
	var rec struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(data, &rec); err != nil || rec.ID == "" {
		return "", fmt.Errorf("%s %s: lookup returned no record id", s.use, ref)
	}
	return rec.ID.String(), nil
}

// createPath is the create endpoint for body: the create-with-items endpoint
// when the row allows it and the body carries "items", else PUT /<path>/.
func (s *entitySpec) createPath(body any) string {
	if obj, ok := body.(map[string]any); ok && s.withItems {
		if _, hasItems := obj["items"]; hasItems {
			return "/" + s.path + "/createWithItems"
		}
	}
	return "/" + s.path + "/"
}

// send makes a mutating request (create, update, delete, actions), prints
// the API's JSON response on stdout and returns it.
func (a *app) send(c *client.Client, method, path string, body any) (json.RawMessage, error) {
	resp, err := c.Do(method, path, nil, body)
	if err != nil {
		return nil, err
	}
	var out json.RawMessage
	if err := client.DecodeJSON(resp, &out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, a.printJSON(out)
}

// confirm prints a short confirmation of a mutation on stderr, only in a
// terminal: scripts read the response JSON on stdout instead.
func (a *app) confirm(format string, args ...any) {
	if a.isTerminal() {
		fmt.Fprintf(a.stderr, format+"\n", args...)
	}
}

// createdID extracts the new record's id from a create response: data.id, or
// data itself when it is the bare id (codelist create); "" when absent.
func createdID(resp json.RawMessage) string {
	var env client.Envelope
	if json.Unmarshal(resp, &env) != nil {
		return ""
	}
	var id json.Number
	if json.Unmarshal(env.Data, &id) == nil {
		return id.String()
	}
	var rec struct {
		ID json.Number `json:"id"`
	}
	if json.Unmarshal(env.Data, &rec) != nil {
		return ""
	}
	return rec.ID.String()
}
