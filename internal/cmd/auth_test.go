/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"strings"
	"testing"
)

func TestParseHostInput(t *testing.T) {
	tests := []struct {
		in         string
		wantHost   string
		wantScheme string
		wantErr    bool
	}{
		{"app.raynet.cz", "app.raynet.cz", "", false},
		{"localhost:8080", "localhost:8080", "", false},
		{"http://localhost:8082", "localhost:8082", "http", false},
		{"https://app.raynet.cz", "app.raynet.cz", "https", false},
		{"HTTPS://app.raynet.cz", "app.raynet.cz", "https", false},
		{"http://localhost:8082/", "localhost:8082", "http", false},
		{"  localhost:8080  ", "localhost:8080", "", false},
		{"ftp://localhost:8082", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			host, scheme, err := parseHostInput(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseHostInput(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if host != tt.wantHost || scheme != tt.wantScheme {
				t.Errorf("parseHostInput(%q) = (%q, %q), want (%q, %q)", tt.in, host, scheme, tt.wantHost, tt.wantScheme)
			}
		})
	}
}

func TestValidateHostScheme(t *testing.T) {
	tests := []struct {
		host    string
		scheme  string
		wantErr bool
	}{
		{"app.raynet.cz", "", false},
		{"app.raynet.cz", "https", false},
		{"localhost:8082", "http", false},
		{"127.0.0.1:8082", "http", false},
		{"[::1]:8082", "http", false},
		{"app.raynet.cz", "http", true},
		{"staging.example.com:8080", "http", true},
	}
	for _, tt := range tests {
		t.Run(tt.host+"/"+tt.scheme, func(t *testing.T) {
			err := validateHostScheme(tt.host, tt.scheme)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateHostScheme(%q, %q) error = %v, wantErr %v", tt.host, tt.scheme, err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "https") {
				t.Errorf("validateHostScheme(%q, %q) error = %q, want mention of https", tt.host, tt.scheme, err)
			}
		})
	}
}
