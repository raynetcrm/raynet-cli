/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a non-2xx response from the REST API. It is parsed leniently
// from the API's error body {type, message, translatedMessage, data, status}:
// an empty or non-JSON body still yields an APIError with just the status.
type APIError struct {
	// Status is the HTTP status code.
	Status int `json:"status"`
	// Type is the API's error class, e.g. BackEnd, BadRequest, Forbidden.
	Type string `json:"type,omitempty"`
	// Message is the API's technical message, e.g.
	// "TranslatedApplicationException: exception.duplicateKey".
	Message string `json:"message,omitempty"`
	// TranslatedMessage is the message in the user's language; empty when
	// the API sent none or an unusable one ("(empty)", garbled "?key?").
	TranslatedMessage string `json:"translatedMessage,omitempty"`
	// FieldErrors are the validation errors from the body's data, per field
	// or, without a field, for the record as a whole.
	FieldErrors []FieldError `json:"fieldErrors,omitempty"`
	// Body is the raw response body, for --verbose.
	Body []byte `json:"-"`
}

// FieldError is one validation error, tied to a body field or, when Field is
// empty, to the record as a whole.
type FieldError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Error returns "<status> <text>", e.g. "400 Invalid record".
func (e *APIError) Error() string {
	return fmt.Sprintf("%d %s", e.Status, e.Text())
}

// Text is the message to show a user: the translated message when usable,
// else the technical message, else the HTTP status text.
func (e *APIError) Text() string {
	switch {
	case e.TranslatedMessage != "":
		return e.TranslatedMessage
	case e.Message != "":
		return e.Message
	default:
		return http.StatusText(e.Status)
	}
}

// newAPIError builds an APIError from a non-2xx response's status and body.
func newAPIError(status int, body []byte) *APIError {
	e := &APIError{Status: status, Body: body}
	var raw struct {
		Type              any             `json:"type"`
		Message           any             `json:"message"`
		TranslatedMessage any             `json:"translatedMessage"`
		Data              json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &raw) != nil {
		return e
	}
	e.Type = str(raw.Type)
	e.Message = strings.TrimSpace(str(raw.Message))
	if t := strings.TrimSpace(str(raw.TranslatedMessage)); usableTranslation(t) {
		e.TranslatedMessage = t
	}
	e.FieldErrors = fieldErrors(raw.Data)
	return e
}

// usableTranslation reports whether a translatedMessage is worth showing: the
// backend sends "(empty)" when it has no translation and "?key?" when a
// translation key is missing.
func usableTranslation(t string) bool {
	if t == "" || t == "(empty)" {
		return false
	}
	garbled := len(t) >= 2 && strings.HasPrefix(t, "?") && strings.HasSuffix(t, "?")
	return !garbled
}

// fieldErrors extracts the validation errors of an error data array: the
// elements that carry a fieldName or a message. data is not always validation
// errors (bulk results are an object, some errors carry a raw string), so
// anything else is skipped.
func fieldErrors(data json.RawMessage) []FieldError {
	var items []json.RawMessage
	if json.Unmarshal(data, &items) != nil {
		return nil
	}
	var out []FieldError
	for _, item := range items {
		var obj map[string]any
		if json.Unmarshal(item, &obj) != nil {
			continue
		}
		fe := FieldError{Field: str(obj["fieldName"]), Message: str(obj["message"])}
		if fe.Field == "" && fe.Message == "" {
			continue
		}
		out = append(out, fe)
	}
	return out
}

// str returns v when it is a JSON string, else "".
func str(v any) string {
	s, _ := v.(string)
	return s
}
