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
	"strings"

	"github.com/raynetcrm/raynet-cli/internal/client"
	"github.com/spf13/cobra"
)

// defaultListLimit is how many records `list` fetches without --limit or --all.
const defaultListLimit = 30

// extPrefix marks a record reference as an external id: "ext:<extId>".
const extPrefix = "ext:"

// recordRef is a record reference as a command argument names it: a numeric
// id, or "ext:<extId>" for an external id.
type recordRef struct {
	// value is the id, or the external id when ext is set.
	value string
	ext   bool
}

// parseRecordRef parses a record reference argument.
func parseRecordRef(arg string) recordRef {
	extID, isExt := strings.CutPrefix(arg, extPrefix)
	if isExt {
		return recordRef{value: extID, ext: true}
	}
	return recordRef{value: arg}
}

// String is the reference as it was given.
func (r recordRef) String() string {
	if r.ext {
		return extPrefix + r.value
	}
	return r.value
}

// newEntityCmds builds one command per entity spec table row.
func newEntityCmds(a *app) []*cobra.Command {
	cmds := make([]*cobra.Command, len(entityTable))
	for i := range entityTable {
		cmds[i] = newEntityCmd(a, &entityTable[i])
	}
	return cmds
}

// newEntityCmd is the builder: it turns an entity spec table row into the
// entity's command and the subcommands for the verbs it supports.
func newEntityCmd(a *app, s *entitySpec) *cobra.Command {
	c := &cobra.Command{
		Use:     s.use,
		Aliases: s.aliases,
		Short:   s.short,
		Example: entityExample(s),
	}
	for _, v := range s.verbs.each() {
		c.AddCommand(entityVerbCmds[v](a, s))
	}
	for _, act := range s.actions {
		c.AddCommand(newActionCmd(a, s, act))
	}
	for i := range s.nested {
		c.AddCommand(newNestedCmd(a, s, &s.nested[i]))
	}
	if s.commands != nil {
		c.AddCommand(s.commands(a)...)
	}
	return c
}

// entityVerbCmds builds the command of each verb of an entity.
var entityVerbCmds = map[verb]func(*app, *entitySpec) *cobra.Command{
	verbList:   newListCmd,
	verbView:   newViewCmd,
	verbCreate: newCreateCmd,
	verbUpdate: newUpdateCmd,
	verbDelete: newDeleteCmd,
}

// entityExample is the entity command's own example: a taste of its
// commands, pointing at create --help for the field reference.
func entityExample(s *entitySpec) string {
	tails := map[verb]string{verbList: "", verbView: " 123", verbCreate: " --help"}
	var lines []string
	for _, v := range s.verbs.each() {
		if tail, ok := tails[v]; ok {
			lines = append(lines, fmt.Sprintf("raynet %s %s%s", s.use, v.name(), tail))
		}
	}
	return "  " + strings.Join(lines, "\n  ")
}

// listOptions are the flags of `<entity> list`; they map onto the REST API's
// paging, filtering and sorting query parameters.
type listOptions struct {
	limit    int
	offset   int
	all      bool
	fulltext string
	filters  []string
	sort     string
	columns  []string
	web      bool
	// typeColumn mirrors entitySpec.typeColumn.
	typeColumn bool
}

func newListCmd(a *app, s *entitySpec) *cobra.Command {
	o := listOptions{typeColumn: s.typeColumn}
	long := fmt.Sprintf("List %s records, %d at a time (--limit up to %d); page with --offset or fetch every page with --all.", s.use, defaultListLimit, client.MaxPageSize)
	if s.listFilters != "" {
		long += "\n\nUseful filters: " + s.listFilters
	}
	long += "\n\nSee \"raynet help filtering\" for --filter operators and date formats."
	c := &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("List %s records", s.use),
		Long:  long,
		Example: fmt.Sprintf("  raynet %s list --fulltext acme --sort -id\n  raynet %s list --columns %s",
			s.use, s.use, strings.Join(s.columns, ",")),
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if o.web {
				return a.openListWeb(s)
			}
			if o.columns == nil {
				o.columns = s.columns
			}
			return a.listRecords("/"+s.path+"/", &o)
		},
	}
	f := c.Flags()
	f.IntVar(&o.limit, "limit", defaultListLimit, fmt.Sprintf("how many records to fetch, 1 to %d", client.MaxPageSize))
	f.IntVar(&o.offset, "offset", 0, "index of the first record to fetch")
	f.BoolVar(&o.all, "all", false, "fetch every page")
	f.StringVar(&o.fulltext, "fulltext", "", "fulltext search, as the CRM search box does")
	f.StringArrayVar(&o.filters, "filter", nil, `attribute[OP]=value, repeatable, e.g. --filter "rowInfo.createdAt[GT]=2026-01-01 00:00" (OP defaults to EQ)`)
	f.StringVar(&o.sort, "sort", "", "column to sort by; prefix with - for descending, e.g. -name")
	f.StringSliceVar(&o.columns, "columns", nil, "table columns, comma-separated; dotted paths reach nested fields, e.g. owner.fullName")
	c.MarkFlagsMutuallyExclusive("all", "offset")
	c.MarkFlagsMutuallyExclusive("all", "limit")
	if s.webName != "" {
		f.BoolVar(&o.web, "web", false, webFlagUsage)
	}
	return c
}

// query checks --limit and maps the options, except paging, onto list query
// parameters.
func (o *listOptions) query() (url.Values, error) {
	if o.limit < 1 || o.limit > client.MaxPageSize {
		return nil, fmt.Errorf("--limit must be between 1 and %d, got %d", client.MaxPageSize, o.limit)
	}
	q := url.Values{}
	if o.fulltext != "" {
		q.Set("fulltext", o.fulltext)
	}
	if o.sort != "" {
		column, desc := strings.CutPrefix(o.sort, "-")
		q.Set("sortColumn", column)
		if desc {
			q.Set("sortDirection", "DESC")
		} else {
			q.Set("sortDirection", "ASC")
		}
	}
	for _, raw := range o.filters {
		key, value, found := strings.Cut(raw, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("invalid --filter %q, expected attribute[OP]=value", raw)
		}
		q.Add(key, value)
	}
	return q, nil
}

// listRecords fetches one page (or every page with --all) of a list endpoint
// and prints it; in a terminal it also reports "<shown> of <total>" on
// stderr.
func (a *app) listRecords(path string, o *listOptions) error {
	q, err := o.query()
	if err != nil {
		return err
	}
	c, err := a.resolveClient()
	if err != nil {
		return err
	}

	var page client.ListPage
	if o.all {
		page.Records, err = c.GetAllPages(path, q)
		page.TotalCount = len(page.Records)
	} else {
		page, err = c.GetPage(path, q, o.offset, o.limit)
	}
	if err != nil {
		return err
	}

	records, err := decodeRecords(page.Records, "record")
	if err != nil {
		return err
	}
	if o.typeColumn && !a.flags.json && a.flags.jq == "" {
		for _, r := range records {
			r["type"] = entityType(r)
		}
	}
	if err := a.printRecords(records, o.columns); err != nil {
		return err
	}
	if a.isTerminal() && page.TotalCount >= 0 {
		fmt.Fprintf(a.stderr, "%d of %d\n", len(records), page.TotalCount)
	}
	return nil
}

// decodeRecords decodes the raw records of a list response; noun names a
// record in the error, e.g. "decoding record 3".
func decodeRecords(raw []json.RawMessage, noun string) ([]map[string]any, error) {
	records := make([]map[string]any, len(raw))
	for i, r := range raw {
		if err := json.Unmarshal(r, &records[i]); err != nil {
			return nil, fmt.Errorf("decoding %s %d: %w", noun, i, err)
		}
	}
	return records, nil
}

// entityType is the entity command of a record in a mixed list, found by its
// _entityName ("PhoneCall" is phone-call); unknown names show as they are.
func entityType(rec map[string]any) string {
	name, _ := rec["_entityName"].(string)
	for i := range entityTable {
		if s := &entityTable[i]; s.webName != "" && s.webName == name {
			return s.use
		}
	}
	return name
}

func newViewCmd(a *app, s *entitySpec) *cobra.Command {
	example := fmt.Sprintf("  raynet %s view 123", s.use)
	if s.ext {
		example += fmt.Sprintf("\n  raynet %s view ext:erp-17", s.use)
	}
	var web bool
	c := &cobra.Command{
		Use:     "view " + s.refArg(),
		Short:   fmt.Sprintf("Show %s record", withArticle(s.use)),
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ref := parseRecordRef(args[0])
			if web {
				return a.openRecordWeb(s, ref)
			}
			path, err := s.viewPath(ref)
			if err != nil {
				return err
			}
			return a.viewEntity(path)
		},
	}
	if s.webName != "" {
		c.Flags().BoolVar(&web, "web", false, webFlagUsage)
		c.Example += fmt.Sprintf("\n  raynet %s view 123 --web", s.use)
	}
	return c
}

// viewPath is the GET path of the record ref names: by id, or by external
// id on entities with external id lookup.
func (s *entitySpec) viewPath(ref recordRef) (string, error) {
	switch {
	case !ref.ext:
		return s.recordPath(ref.value), nil
	case !s.ext:
		return "", fmt.Errorf("%s records can't be looked up by external id", s.use)
	case ref.value == "":
		return "", fmt.Errorf("empty external id in %q", ref)
	default:
		return fmt.Sprintf("/%s/ext/%s/", s.path, url.PathEscape(ref.value)), nil
	}
}

// viewEntity fetches entityPath (already including the id) and prints it as JSON.
func (a *app) viewEntity(entityPath string) error {
	c, err := a.resolveClient()
	if err != nil {
		return err
	}
	resp, err := c.Do(http.MethodGet, entityPath, nil, nil)
	if err != nil {
		return err
	}
	var out json.RawMessage
	if err := client.DecodeJSON(resp, &out); err != nil {
		return err
	}
	return a.printJSON(out)
}

// getData GETs path and returns the data of the API's response envelope.
func getData(c *client.Client, path string) (json.RawMessage, error) {
	resp, err := c.Do(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var env client.Envelope
	if err := client.DecodeJSON(resp, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}
