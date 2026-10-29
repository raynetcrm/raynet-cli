/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

// Package version holds the CLI's build-time version string.
package version

// Version is "dev" for a plain `go build`. Release builds set it via
// -ldflags "-X github.com/raynetcrm/raynet-cli/internal/version.Version=...",
// with the value coming from the Makefile (.version or git describe) or the
// release pipeline.
var Version = "dev"
