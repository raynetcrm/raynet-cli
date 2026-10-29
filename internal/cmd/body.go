/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// bodyFlags are the request-body flags shared by `raynet api` and every
// command that sends a body (create, update, actions):
//
//	-f key=value   string field
//	-F key=value   typed field: number, true/false, null, @file (file contents)
//	--input file   JSON base body from a file, or "-" for stdin
//
// Keys may use bracket nesting (see setBodyField). Fields are applied in the
// order given, on top of the --input body.
type bodyFlags struct {
	fields []bodyField
	input  string
}

// bodyField is one -f or -F flag value.
type bodyField struct {
	key, value string
	typed      bool
}

// register adds -f/--raw-field, -F/--field and --input to c.
func (b *bodyFlags) register(c *cobra.Command) {
	c.Flags().VarP(&bodyFieldFlag{b: b}, "raw-field", "f", "add a string field `key=value` (repeatable; key[sub] and key[] nest)")
	c.Flags().VarP(&bodyFieldFlag{b: b, typed: true}, "field", "F", "add a typed field `key=value`: number, true/false, null or @file (repeatable; key[sub] and key[] nest)")
	c.Flags().StringVar(&b.input, "input", "", "read the JSON request body from `file` (\"-\" for stdin); -f/-F apply on top")
}

// body builds the JSON request body: the --input document with every field
// applied on top. It returns nil when there is neither input nor a field.
func (b *bodyFlags) body(stdin io.Reader) (any, error) {
	base, err := b.readInput(stdin)
	if err != nil {
		return nil, err
	}
	if len(b.fields) == 0 {
		return base, nil
	}
	obj, ok := base.(map[string]any)
	if !ok {
		if base != nil {
			return nil, fmt.Errorf("--input must be a JSON object when combined with -f/-F")
		}
		obj = map[string]any{}
	}
	for _, f := range b.fields {
		v, err := f.parse()
		if err != nil {
			return nil, err
		}
		if err := setBodyField(obj, f.key, v); err != nil {
			return nil, err
		}
	}
	return obj, nil
}

// query returns the fields as query parameters, keys taken literally and
// values as text (@file still reads the file).
func (b *bodyFlags) query() (url.Values, error) {
	q := url.Values{}
	for _, f := range b.fields {
		v := f.value
		if f.typed && strings.HasPrefix(v, "@") {
			data, err := os.ReadFile(v[1:])
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", f.key, err)
			}
			v = string(data)
		}
		q.Add(f.key, v)
	}
	return q, nil
}

// readInput returns the --input document, nil without --input. Callers that
// send fields as query parameters use it as the whole body.
func (b *bodyFlags) readInput(stdin io.Reader) (any, error) {
	if b.input == "" {
		return nil, nil
	}
	var (
		data []byte
		err  error
	)
	if b.input == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(b.input)
	}
	if err != nil {
		return nil, fmt.Errorf("--input: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("--input is not valid JSON: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("--input is not valid JSON: trailing data")
	}
	return v, nil
}

// parse converts the field's value to its JSON value.
func (f bodyField) parse() (any, error) {
	if !f.typed {
		return f.value, nil
	}
	switch v := f.value; {
	case v == "true":
		return true, nil
	case v == "false":
		return false, nil
	case v == "null":
		return nil, nil
	case strings.HasPrefix(v, "@"):
		data, err := os.ReadFile(v[1:])
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", f.key, err)
		}
		return string(data), nil
	case isJSONNumber(v):
		return json.Number(v), nil
	default:
		return v, nil
	}
}

// isJSONNumber reports whether s is a JSON number literal, e.g. 3, -1.5, 2e3.
func isJSONNumber(s string) bool {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		return false
	}
	n, ok := v.(json.Number)
	return ok && n.String() == s
}

// bodyFieldFlag is the pflag value behind -f and -F; both append to one list
// so fields apply in command-line order.
type bodyFieldFlag struct {
	b     *bodyFlags
	typed bool
}

func (v *bodyFieldFlag) Set(s string) error {
	key, value, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("expected key=value, got %q", s)
	}
	v.b.fields = append(v.b.fields, bodyField{key: key, value: value, typed: v.typed})
	return nil
}

func (v *bodyFieldFlag) String() string { return "" }

func (v *bodyFieldFlag) Type() string { return "stringArray" }

// setBodyField sets value in body at a glab-style bracket key:
//
//	name                  {"name": v}
//	owner[id]             {"owner": {"id": v}}
//	tags[]                appends v to the "tags" array
//	items[][count]        sets "count" on the last object of "items", or
//	                      appends a new object when the last one already has it
func setBodyField(body map[string]any, key string, value any) error {
	path, err := parseBodyKey(key)
	if err != nil {
		return err
	}
	return setPath(body, path, value, key)
}

// parseBodyKey splits "a[b][][c]" into ["a", "b", "", "c"]; "" marks an array.
func parseBodyKey(key string) ([]string, error) {
	invalid := fmt.Errorf("invalid field key %q, expected name or name[key][]…", key)
	head, rest, _ := strings.Cut(key, "[")
	if head == "" || strings.Contains(head, "]") {
		return nil, invalid
	}
	path := []string{head}
	if rest == "" {
		if strings.HasSuffix(key, "[") {
			return nil, invalid
		}
		return path, nil
	}
	rest = "[" + rest
	for rest != "" {
		if rest[0] != '[' {
			return nil, invalid
		}
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			return nil, invalid
		}
		seg := rest[1:end]
		if strings.Contains(seg, "[") {
			return nil, invalid
		}
		// An array must hold scalars or objects: "a[][]" is not supported.
		if seg == "" && len(path) > 0 && path[len(path)-1] == "" {
			return nil, invalid
		}
		path = append(path, seg)
		rest = rest[end+1:]
	}
	return path, nil
}

func setPath(m map[string]any, path []string, value any, key string) error {
	name := path[0]
	rest := path[1:]
	if len(rest) == 0 {
		m[name] = value
		return nil
	}
	if rest[0] == "" {
		arr, ok := m[name].([]any)
		if !ok && m[name] != nil {
			return fmt.Errorf("field %q: %q is not an array", key, name)
		}
		elemPath := rest[1:]
		if len(elemPath) == 0 {
			m[name] = append(arr, value)
			return nil
		}
		var elem map[string]any
		if n := len(arr); n > 0 {
			if last, ok := arr[n-1].(map[string]any); ok && !hasPath(last, elemPath) {
				elem = last
			}
		}
		if elem == nil {
			elem = map[string]any{}
			arr = append(arr, elem)
		}
		m[name] = arr
		return setPath(elem, elemPath, value, key)
	}
	sub, ok := m[name].(map[string]any)
	if !ok {
		if m[name] != nil {
			return fmt.Errorf("field %q: %q is not an object", key, name)
		}
		sub = map[string]any{}
		m[name] = sub
	}
	return setPath(sub, rest, value, key)
}

// hasPath reports whether m already holds a value at the object path; a path
// through an array ("") never counts as set, since it appends.
func hasPath(m map[string]any, path []string) bool {
	for i, seg := range path {
		if seg == "" {
			return false
		}
		v, ok := m[seg]
		if !ok {
			return false
		}
		if i == len(path)-1 {
			return true
		}
		if m, ok = v.(map[string]any); !ok {
			return true
		}
	}
	return false
}
