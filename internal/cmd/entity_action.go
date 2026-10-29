/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

// The record actions shared by many entities; a row lists the ones its API
// supports, e.g. actions: []action{actLock, actUnlock, actTag, actUntag}.
var (
	actLock       = action{use: "lock", short: "Lock %s record against edits", done: "Locked", method: http.MethodPost, suffix: "lock"}
	actUnlock     = action{use: "unlock", short: "Unlock %s record", done: "Unlocked", method: http.MethodPost, suffix: "unlock"}
	actInvalidate = action{use: "invalidate", short: "Mark %s record as no longer valid", done: "Invalidated", method: http.MethodPost, suffix: "invalid"}
	actRevalidate = action{use: "revalidate", short: "Mark %s record as valid again after invalidate", done: "Revalidated", method: http.MethodPost, suffix: "valid"}
	actTag        = action{use: "tag", short: "Add tags to %s record", done: "Tagged", method: http.MethodPut, suffix: "tag/", tags: true}
	actUntag      = action{use: "untag", short: "Remove tags from %s record", done: "Untagged", method: http.MethodDelete, suffix: "tag/", tags: true}
	// actSync and actUnsync exist on offers and sales orders only.
	actSync   = action{use: "sync", short: "Sync %s with its business case, overwriting the business case's line items", done: "Synced", method: http.MethodPost, suffix: "sync"}
	actUnsync = action{use: "unsync", short: "Stop syncing %s with its business case", done: "Unsynced", method: http.MethodDelete, suffix: "sync"}
)

// newActionCmd builds `<entity> <action> <ref> [tag...]` for one of a row's
// actions.
func newActionCmd(a *app, s *entitySpec, act action) *cobra.Command {
	var body bodyFlags
	use := act.use + " " + s.refArg()
	args := cobra.ExactArgs(1)
	tail := ""
	if act.tags {
		use += " <tag>..."
		args = cobra.MinimumNArgs(2)
		tail = " vip hot"
	}
	example := fmt.Sprintf("  raynet %s %s 123%s", s.use, act.use, tail)
	if s.ext {
		example += fmt.Sprintf("\n  raynet %s %s ext:erp-17%s", s.use, act.use, tail)
	}
	if act.body {
		example += fmt.Sprintf("\n  raynet %s %s 123 --input %s.json", s.use, act.use, act.use)
	}
	c := &cobra.Command{
		Use:     use,
		Short:   fmt.Sprintf(act.short, withArticle(s.use)),
		Example: example,
		Args:    args,
		RunE: func(_ *cobra.Command, args []string) error {
			var b any
			if act.tags {
				b = map[string]any{"tag": strings.Join(args[1:], ",")}
			} else if act.body {
				var err error
				if b, err = body.body(a.stdin); err != nil {
					return err
				}
			}
			cl, err := a.resolveClient()
			if err != nil {
				return err
			}
			id, err := a.recordID(cl, s, parseRecordRef(args[0]))
			if err != nil {
				return err
			}
			if _, err := a.send(cl, act.method, s.recordPath(id)+act.suffix, b); err != nil {
				return err
			}
			a.confirm("%s %s %s", act.done, s.use, id)
			return nil
		},
	}
	if act.body {
		body.register(c)
	}
	if len(act.fields) > 0 {
		c.Long = c.Short + ".\n\n" + fieldReference("Fields:", act.fields)
	}
	return c
}
