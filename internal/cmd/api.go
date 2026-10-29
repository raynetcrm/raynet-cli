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

// apiOptions are the flags of `raynet api`.
type apiOptions struct {
	method   string
	body     bodyFlags
	paginate bool
}

func newAPICmd(a *app) *cobra.Command {
	var o apiOptions
	c := &cobra.Command{
		Use:   "api <path>",
		Short: "Make an authenticated call to any RAYNET CRM REST API endpoint",
		Long: "Make an authenticated call to any RAYNET CRM REST API endpoint: the escape hatch for\n" +
			"everything the entity commands don't cover. <path> is relative to /api/v2.\n\n" +
			"For GET and DELETE, -f/-F fields are sent as query parameters and --input as the body;\n" +
			"otherwise they build the JSON body (see \"raynet help fields\"). The API creates with PUT\n" +
			"and updates with POST.",
		Example: "  raynet api /company/\n" +
			"  raynet api /company/ -f fulltext=acme --paginate\n" +
			"  raynet api /company/ -X PUT -f name=\"RAYNET s.r.o.\" -f rating=A -f 'addresses[][address][city]=Brno'\n" +
			"  raynet api /company/42/ -X POST --input body.json -F owner=7",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.runAPI(&o, args[0])
		},
	}
	c.Flags().StringVarP(&o.method, "method", "X", http.MethodGet, "HTTP method")
	o.body.register(c)
	c.Flags().BoolVar(&o.paginate, "paginate", false, "auto-walk offset/limit paging and return all pages (GET only)")
	return c
}

func (a *app) runAPI(o *apiOptions, path string) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	method := strings.ToUpper(o.method)

	var (
		query url.Values
		body  any
		err   error
	)
	if method == http.MethodGet || method == http.MethodDelete {
		if query, err = o.body.query(); err == nil {
			body, err = o.body.readInput(a.stdin)
		}
	} else {
		body, err = o.body.body(a.stdin)
	}
	if err != nil {
		return err
	}

	c, err := a.resolveClient()
	if err != nil {
		return err
	}

	if o.paginate {
		if method != http.MethodGet {
			return fmt.Errorf("--paginate is only supported with GET")
		}
		pages, err := c.GetAllPages(path, query)
		if err != nil {
			return err
		}
		return a.printJSON(pages)
	}

	resp, err := c.Do(method, path, query, body)
	if err != nil {
		return err
	}
	var out json.RawMessage
	if decodeErr := client.DecodeJSON(resp, &out); decodeErr != nil {
		return decodeErr
	}
	if len(out) == 0 {
		return nil
	}
	return a.printJSON(out)
}
