# Copyright 2026-present RAYNET s.r.o.
# SPDX-License-Identifier: MIT

.DEFAULT_GOAL := build

BINARY := raynet
BIN_DIR := bin
CMD := ./cmd/raynet
VERSION_PKG := github.com/raynetcrm/raynet-cli/internal/version

# Prefer a version already resolved into .version (CI writes one). Local dev
# builds fall back to `git describe`, or plain "dev" in a repo with no tags yet.
VERSION := $(shell cat .version 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

.PHONY: run
run:
	go run $(CMD)

.PHONY: fmt
fmt:
	gofmt -l -w .

.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:"; gofmt -l .; exit 1)

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: test
test:
	go test ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# Fails if a published file contains a denylisted string, then runs the tests
# of the internal scripts. The scripts and .public-denylist aren't part
# of the published source, so this does nothing without the denylist; with it,
# a missing script fails loudly. CI's `check` job runs the same two commands
# without make; keep them in step.
.PHONY: public-check
public-check:
	@if [ -f .public-denylist ]; then \
		bash scripts/check-public-denylist.sh && \
		for t in scripts/*_test.sh; do bash "$$t" || exit 1; done; \
	fi

# Everything CI runs, for a quick local pre-push check.
.PHONY: check
check: fmt-check vet lint test public-check build
