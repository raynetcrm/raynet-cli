/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const validationErrorBody = `{"type":"BackEnd","message":"FieldValidationException: name","translatedMessage":"Invalid record",` +
	`"data":[{"message":"Name is required","_validationId":"v1","fieldName":"name"},` +
	`{"message":"Bad e-mail","_validationId":"v2","fieldName":"contactInfo.email"}],"status":400}`

const messageOnlyErrorBody = `{"type":"BackEnd","message":"ValidationException","translatedMessage":"Not allowed",` +
	`"data":[{"message":"Owner is inactive","_validationId":"v"}],"status":400}`

func TestAPIErrorRendering(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			"translated message with field errors", 400, validationErrorBody,
			"Error: 400 Invalid record\n  name: Name is required\n  contactInfo.email: Bad e-mail\n",
		},
		{
			"BadRequest without translated message", 400, `{"type":"BadRequest","message":"Failed to convert request body","status":400}`,
			"Error: 400 Failed to convert request body\n",
		},
		{
			"translated message is (empty)", 500, `{"type":"BackEnd","message":"EntityLockedException","translatedMessage":"(empty)","status":500}`,
			"Error: 500 EntityLockedException\n",
		},
		{
			"garbled translated message", 500, `{"type":"BackEnd","message":"EntityLockedException","translatedMessage":"?exception.entityLocked?","status":500}`,
			"Error: 500 EntityLockedException\n",
		},
		{
			"validation messages without a field", 400, `{"type":"BackEnd","message":"ValidationException","translatedMessage":"Not allowed",` +
				`"data":[{"message":"Owner is inactive","_validationId":"v"},{"message":"Name is required","fieldName":"name"}],"status":400}`,
			"Error: 400 Not allowed\n  Owner is inactive\n  name: Name is required\n",
		},
		{
			"data that is not field errors", 400, `{"type":"BackEnd","message":"ValidationException","translatedMessage":"Not allowed","data":[1,"x",{"_validationId":"v"}],"status":400}`,
			"Error: 400 Not allowed\n",
		},
		{
			"empty body", 404, ``,
			"Error: 404 Not Found\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.api.reply("POST", "/company/5/", tt.status, tt.body)

			res := h.run("api", "/company/5/", "-X", "POST", "-f", "name=")

			if res.err == nil {
				t.Fatal("err = nil, want a failure (non-zero exit)")
			}
			if res.stderr != tt.want {
				t.Errorf("stderr =\n%q\nwant\n%q", res.stderr, tt.want)
			}
			if res.stdout != "" {
				t.Errorf("stdout = %q, want nothing", res.stdout)
			}
		})
	}
}

func TestAPIErrorRendering_JSON(t *testing.T) {
	h := newHarness(t)
	h.api.reply("POST", "/company/5/", 400, validationErrorBody)

	res := h.run("api", "/company/5/", "-X", "POST", "-f", "name=", "--json")

	if res.err == nil {
		t.Fatal("err = nil, want a failure (non-zero exit)")
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want nothing", res.stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(res.stderr), &got); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, res.stderr)
	}
	want := map[string]any{
		"error":             "Invalid record",
		"status":            float64(400),
		"type":              "BackEnd",
		"message":           "FieldValidationException: name",
		"translatedMessage": "Invalid record",
		"fieldErrors": []any{
			map[string]any{"field": "name", "message": "Name is required"},
			map[string]any{"field": "contactInfo.email", "message": "Bad e-mail"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stderr JSON =\n%v\nwant\n%v", got, want)
	}
}

func TestAPIErrorRendering_JSONIncludesMessagesWithoutField(t *testing.T) {
	h := newHarness(t)
	h.api.reply("POST", "/company/5/", 400, messageOnlyErrorBody)

	res := h.run("api", "/company/5/", "-X", "POST", "-f", "name=", "--json")

	var got map[string]any
	if err := json.Unmarshal([]byte(res.stderr), &got); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, res.stderr)
	}
	want := []any{map[string]any{"message": "Owner is inactive"}}
	if !reflect.DeepEqual(got["fieldErrors"], want) {
		t.Errorf("fieldErrors = %v, want %v", got["fieldErrors"], want)
	}
}

func TestNonAPIErrorRendering(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		h := newHarness(t)
		res := h.run("api", "/company/", "-X", "POST", "--paginate")
		if res.err == nil {
			t.Fatal("err = nil, want a failure")
		}
		if want := "Error: --paginate is only supported with GET\n"; res.stderr != want {
			t.Errorf("stderr = %q, want %q", res.stderr, want)
		}
	})
	t.Run("json", func(t *testing.T) {
		h := newHarness(t)
		res := h.run("api", "/company/", "-X", "POST", "--paginate", "--json")
		var got map[string]any
		if err := json.Unmarshal([]byte(res.stderr), &got); err != nil {
			t.Fatalf("stderr is not JSON: %v\n%s", err, res.stderr)
		}
		if got["error"] != "--paginate is only supported with GET" {
			t.Errorf("stderr JSON = %v, want error = the message", got)
		}
	})
}

func TestAPIErrorRendering_Verbose(t *testing.T) {
	h := newHarness(t)
	h.api.reply("POST", "/company/5/", 400, validationErrorBody)

	res := h.run("api", "/company/5/", "-X", "POST", "-f", "name=", "--verbose")

	if res.err == nil {
		t.Fatal("err = nil, want a failure")
	}
	lines := strings.Split(strings.TrimRight(res.stderr, "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("stderr =\n%s\nwant request line, status, raw body and the error", res.stderr)
	}
	if !strings.HasPrefix(lines[0], "> POST ") || !strings.HasSuffix(lines[0], "/api/v2/company/5/") {
		t.Errorf("request line = %q", lines[0])
	}
	if lines[1] != "< 400 Bad Request" {
		t.Errorf("status line = %q, want %q", lines[1], "< 400 Bad Request")
	}
	if lines[2] != "< "+validationErrorBody {
		t.Errorf("raw body line = %q, want the raw error body", lines[2])
	}
	if !strings.Contains(res.stderr, "Error: 400 Invalid record\n  name: Name is required\n") {
		t.Errorf("stderr =\n%s\nwant the rendered error after the raw body", res.stderr)
	}
}
