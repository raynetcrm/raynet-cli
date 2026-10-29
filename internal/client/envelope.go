/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package client

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Envelope is the REST API's response wrapper: {success, totalCount, data}
// for lists, {success, data} for view and create, {success} for update,
// delete and actions. Decode into it with DecodeJSON.
type Envelope struct {
	Success Flag `json:"success"`
	// TotalCount is the number of records matching a list query (0 when absent).
	TotalCount int `json:"totalCount"`
	// Data is the payload: an object, an array, or a bare number (codelist
	// create returns the new id as data).
	Data json.RawMessage `json:"data"`
}

// Flag is a boolean the API sends either as a JSON bool or as a string
// ("true"/"false").
type Flag bool

// UnmarshalJSON accepts true, false, "true" and "false" (any strconv.ParseBool form).
func (f *Flag) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*f = Flag(b)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("success: want a bool or a string, got %s", data)
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("success: %q is not a bool", s)
	}
	*f = Flag(b)
	return nil
}
