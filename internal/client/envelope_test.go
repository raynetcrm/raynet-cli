/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package client

import (
	"net/http"
	"testing"
)

func TestDecodeEnvelope(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantSuccess bool
		wantTotal   int
		wantData    string
	}{
		{"success as a bool", `{"success":true,"data":{"id":5}}`, true, 0, `{"id":5}`},
		{"success as a string", `{"success":"true","data":{"id":5}}`, true, 0, `{"id":5}`},
		{"success false as a string", `{"success":"false"}`, false, 0, ``},
		{"list with totalCount", `{"success":"true","totalCount":319,"data":[{"id":1}]}`, true, 319, `[{"id":1}]`},
		{"codelist create returns a bare number", `{"success":"true","data":3}`, true, 0, `3`},
		{"no success field", `{"data":[]}`, false, 0, `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: 200, Body: jsonBody(tt.body)}
			var env Envelope
			if err := DecodeJSON(resp, &env); err != nil {
				t.Fatalf("DecodeJSON() error = %v", err)
			}
			if bool(env.Success) != tt.wantSuccess || env.TotalCount != tt.wantTotal || string(env.Data) != tt.wantData {
				t.Errorf("Envelope = {%v %d %s}, want {%v %d %s}", env.Success, env.TotalCount, env.Data, tt.wantSuccess, tt.wantTotal, tt.wantData)
			}
		})
	}
}

func TestDecodeEnvelope_RejectsOddSuccess(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Body: jsonBody(`{"success":{"x":1}}`)}
	var env Envelope
	if err := DecodeJSON(resp, &env); err == nil {
		t.Error("DecodeJSON() expected an error for a non-bool, non-string success")
	}
}
