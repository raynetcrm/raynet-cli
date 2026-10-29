/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"encoding/json"
	"testing"
)

// setAll applies key=value pairs in order to a fresh body and returns it as
// canonical JSON.
func setAll(t *testing.T, pairs ...any) (string, error) {
	t.Helper()
	body := map[string]any{}
	for i := 0; i < len(pairs); i += 2 {
		if err := setBodyField(body, pairs[i].(string), pairs[i+1]); err != nil {
			return "", err
		}
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out), nil
}

func TestSetBodyField_BracketKeys(t *testing.T) {
	tests := []struct {
		name  string
		pairs []any
		want  string
	}{
		{"plain key", []any{"name", "Acme"}, `{"name":"Acme"}`},
		{"nested objects", []any{"owner[id]", 1, "address[city]", "Brno", "address[zip]", "60200"},
			`{"address":{"city":"Brno","zip":"60200"},"owner":{"id":1}}`},
		{"deep nesting", []any{"a[b][c]", true}, `{"a":{"b":{"c":true}}}`},
		{"array of scalars", []any{"tags[]", "vip", "tags[]", "hot"}, `{"tags":["vip","hot"]}`},
		{"array of objects, keys fill the last element", []any{
			"items[][productId]", 5, "items[][count]", 2,
			"items[][productId]", 6, "items[][count]", 1,
		}, `{"items":[{"count":2,"productId":5},{"count":1,"productId":6}]}`},
		{"array of nested objects", []any{
			"addresses[][address][city]", "Brno", "addresses[][address][street]", "Ulice 1",
			"addresses[][address][city]", "Praha",
		}, `{"addresses":[{"address":{"city":"Brno","street":"Ulice 1"}},{"address":{"city":"Praha"}}]}`},
		{"later value overrides", []any{"name", "A", "name", "B"}, `{"name":"B"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setAll(t, tt.pairs...)
			if err != nil {
				t.Fatalf("setBodyField() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("body = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSetBodyField_InvalidKeys(t *testing.T) {
	for _, key := range []string{"", "[a]", "a[", "a]b", "a[b]c", "a[][]", "a[b"} {
		t.Run(key, func(t *testing.T) {
			if _, err := setAll(t, key, "x"); err == nil {
				t.Errorf("setBodyField(%q) expected an error", key)
			}
		})
	}
}

func TestSetBodyField_TypeConflict(t *testing.T) {
	if _, err := setAll(t, "a", "x", "a[b]", "y"); err == nil {
		t.Error("expected an error nesting under a string value")
	}
	if _, err := setAll(t, "a[b]", "x", "a[]", "y"); err == nil {
		t.Error("expected an error appending to an object")
	}
}
