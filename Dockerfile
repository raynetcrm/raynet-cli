# Copyright 2026-present RAYNET s.r.o.
# SPDX-License-Identifier: MIT

# Cross-compiles natively via buildx's TARGETOS/TARGETARCH build args, so a
# single `docker buildx build --platform linux/amd64,linux/arm64` produces
# both architectures without needing pre-built binaries as input.
# Go itself is installed by mise from .mise.toml, the same pin local dev and
# CI use, so there's no separate golang base-image version to keep in sync.
FROM debian:13-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad AS builder
COPY --from=ghcr.io/jdx/mise:2026.10.18@sha256:fc31de7b93d195613deb600d42c65a164b0205716ade38132aadeabac3838248 /usr/local/bin/mise /usr/local/bin/mise
RUN set -eux; \
    apt-get update; \
    apt-get -y --no-install-recommends install ca-certificates git; \
    rm -rf /var/lib/apt/lists/*
ENV MISE_DATA_DIR="/mise"
ENV MISE_CACHE_DIR="/mise/cache"
ENV PATH="/mise/shims:$PATH"
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY .mise.toml ./
RUN set -eux; mise trust; mise install go
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags "-X github.com/raynetcrm/raynet-cli/internal/version.Version=$VERSION" \
    -o /out/raynet ./cmd/raynet

FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc
# ca-certificates for HTTPS to the RAYNET CRM API; jq so --jq/the default
# terminal syntax-highlighting path works out of the box in the container too.
RUN apk add --no-cache ca-certificates jq
COPY --from=builder /out/raynet /usr/local/bin/raynet
ENTRYPOINT ["/usr/local/bin/raynet"]
