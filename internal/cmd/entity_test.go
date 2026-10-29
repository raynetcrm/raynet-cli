/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import "testing"

func TestColumnValue(t *testing.T) {
	rec := map[string]any{
		"id":                float64(1),
		"owner":             map[string]any{"id": float64(5), "fullName": "Jan Novák"},
		"rowInfo.createdAt": "2026-01-02 10:00",
		"rowInfo":           map[string]any{"createdAt": "nested loses"},
		"primaryAddress":    map[string]any{"address": map[string]any{"city": "Brno"}},
		"tags":              []any{"vip", "hot"},
		"empty":             nil,
	}
	tests := []struct {
		path string
		want string
	}{
		{"id", "1"},
		{"owner.fullName", "Jan Novák"},
		{"primaryAddress.address.city", "Brno"},
		{"rowInfo.createdAt", "2026-01-02 10:00"},
		{"owner", `{"fullName":"Jan Novák","id":5}`},
		{"tags", `["vip","hot"]`},
		{"empty", ""},
		{"missing", ""},
		{"owner.missing", ""},
		{"id.nested", ""},
		{"primaryAddress.address.city.more", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := columnValue(rec, tt.path); got != tt.want {
				t.Errorf("columnValue(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestColumnValue_FlatDottedKeyInsideNestedObject(t *testing.T) {
	rec := map[string]any{"data": map[string]any{"rowInfo.updatedBy": "eva"}}
	if got := columnValue(rec, "data.rowInfo.updatedBy"); got != "eva" {
		t.Errorf("columnValue() = %q, want eva", got)
	}
}
