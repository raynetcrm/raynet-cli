/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import "github.com/spf13/cobra"

// newUserMeCmd builds `user-account me`: who and which instance the active
// credentials resolve to (GET /security/info).
func newUserMeCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "me",
		Short:   "Show who and which instance the active credentials resolve to",
		Long:    "Show the username, `userAccountId`, instance name and instance id the active credentials resolve to (GET /security/info).",
		Example: "  raynet user-account me\n  raynet user-account me --jq .userAccountId",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.viewEntity("/security/info")
		},
	}
}
