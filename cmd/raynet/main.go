/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

// Command raynet is a CLI client for the RAYNET CRM REST API.
package main

import (
	"os"

	"github.com/raynetcrm/raynet-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
