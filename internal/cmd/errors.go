/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/raynetcrm/raynet-cli/internal/client"
)

// errorOutput is the --json form of a failed command, printed on stderr.
// error is the text a user would see; the rest is set for API errors only.
type errorOutput struct {
	Error string `json:"error"`
	*client.APIError
}

// renderError prints a failed command's error on stderr, the single place
// every command's errors are rendered:
//
//	Error: <status> <message>
//	  <field>: <message>
//	  <message>              (a validation error without a field)
//
// With --json the error is printed as JSON instead; with --verbose the raw
// API error body comes first (after the request and status lines the client
// already logged).
func (a *app) renderError(err error) {
	var apiErr *client.APIError
	isAPI := errors.As(err, &apiErr)

	if isAPI && a.flags.verbose && len(apiErr.Body) > 0 {
		fmt.Fprintf(a.stderr, "< %s\n", apiErr.Body)
	}

	if a.flags.json {
		out := errorOutput{Error: err.Error()}
		if isAPI {
			out = errorOutput{Error: apiErr.Text(), APIError: apiErr}
		}
		data, mErr := json.MarshalIndent(out, "", "  ")
		if mErr == nil {
			fmt.Fprintln(a.stderr, string(data))
			return
		}
	}

	fmt.Fprintln(a.stderr, "Error:", err)
	if isAPI {
		for _, fe := range apiErr.FieldErrors {
			if fe.Field == "" {
				fmt.Fprintf(a.stderr, "  %s\n", fe.Message)
			} else {
				fmt.Fprintf(a.stderr, "  %s: %s\n", fe.Field, fe.Message)
			}
		}
	}
}
