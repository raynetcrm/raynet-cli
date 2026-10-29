/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package client

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

// decodeError runs a fake error response through DecodeJSON and returns the
// typed API error it produced.
func decodeError(t *testing.T, status int, body string) *APIError {
	t.Helper()
	resp := &http.Response{StatusCode: status, Body: jsonBody(body)}
	err := DecodeJSON(resp, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DecodeJSON() error = %v (%T), want *APIError", err, err)
	}
	return apiErr
}

func TestAPIError_FromErrorBody(t *testing.T) {
	body := `{"type":"BackEnd","message":"FieldValidationException: name","translatedMessage":"Invalid record",` +
		`"data":[{"message":"Name is required","_validationId":"v1","fieldName":"name"},` +
		`{"message":"Bad e-mail","_validationId":"v2","fieldName":"contactInfo.email"}],"status":400}`

	got := decodeError(t, 400, body)

	if got.Status != 400 || got.Type != "BackEnd" {
		t.Errorf("Status, Type = %d, %q, want 400, BackEnd", got.Status, got.Type)
	}
	if got.Message != "FieldValidationException: name" || got.TranslatedMessage != "Invalid record" {
		t.Errorf("Message, TranslatedMessage = %q, %q", got.Message, got.TranslatedMessage)
	}
	wantFields := []FieldError{{Field: "name", Message: "Name is required"}, {Field: "contactInfo.email", Message: "Bad e-mail"}}
	if !reflect.DeepEqual(got.FieldErrors, wantFields) {
		t.Errorf("FieldErrors = %+v, want %+v", got.FieldErrors, wantFields)
	}
	if string(got.Body) != body {
		t.Errorf("Body = %q, want the raw response body", got.Body)
	}
	if got.Error() != "400 Invalid record" {
		t.Errorf("Error() = %q, want %q", got.Error(), "400 Invalid record")
	}
}

func TestAPIError_Text(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"translated message wins", `{"message":"TranslatedApplicationException: exception.duplicateKey","translatedMessage":"Duplicate key"}`, "Duplicate key"},
		{"no translated message (BadRequest)", `{"type":"BadRequest","message":"Cannot convert body"}`, "Cannot convert body"},
		{"translated message is (empty)", `{"message":"EntityLockedException","translatedMessage":"(empty)"}`, "EntityLockedException"},
		{"translated message is garbled", `{"message":"EntityLockedException","translatedMessage":"?exception.locked?"}`, "EntityLockedException"},
		{"translated message is blank", `{"message":"Unauthorized","translatedMessage":"  "}`, "Unauthorized"},
		{"empty body", ``, "Internal Server Error"},
		{"non-JSON body", `<html>oops</html>`, "Internal Server Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeError(t, 500, tt.body)
			if got.Text() != tt.want {
				t.Errorf("Text() = %q, want %q", got.Text(), tt.want)
			}
		})
	}
}

func TestAPIError_UnusableTranslatedMessageIsDropped(t *testing.T) {
	got := decodeError(t, 500, `{"message":"EntityLockedException","translatedMessage":"(empty)"}`)
	if got.TranslatedMessage != "" {
		t.Errorf("TranslatedMessage = %q, want empty", got.TranslatedMessage)
	}
}

func TestAPIError_DataThatIsNotFieldErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want []FieldError
	}{
		{"validation messages without fieldName", `[{"message":"Something is wrong","_validationId":"v1"}]`,
			[]FieldError{{Message: "Something is wrong"}}},
		{"bulk operation object", `{"changedRecords":0,"insertedRecords":0,"errors":{"1":{"message":"x","name":"y"}}}`, nil},
		{"raw string", `"some backend data"`, nil},
		{"array of scalars", `[1,"two",null]`, nil},
		{"array of mixed objects", `[{"fieldName":42},{"_validationId":"v"},{"fieldName":"name","message":"Required"}]`,
			[]FieldError{{Field: "name", Message: "Required"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeError(t, 400, `{"type":"BackEnd","message":"ValidationException","data":`+tt.data+`}`)
			if got.Message != "ValidationException" {
				t.Errorf("Message = %q, want ValidationException", got.Message)
			}
			if !reflect.DeepEqual(got.FieldErrors, tt.want) {
				t.Errorf("FieldErrors = %+v, want %+v", got.FieldErrors, tt.want)
			}
		})
	}
}

func TestDecodeListResponse_ReturnsAPIError(t *testing.T) {
	resp := &http.Response{StatusCode: 403, Body: jsonBody(`{"type":"Forbidden","message":"Forbidden"}`)}
	_, err := DecodeListResponse(resp)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("DecodeListResponse() error = %v (%T), want *APIError", err, err)
	}
	if apiErr.Status != 403 || apiErr.Text() != "Forbidden" {
		t.Errorf("APIError = %d %q, want 403 Forbidden", apiErr.Status, apiErr.Text())
	}
}
