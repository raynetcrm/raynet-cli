/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

// Package client is a thin HTTP client for the RAYNET CRM REST API v2:
// it resolves the base URL for the active host, injects the configured
// auth headers, and offers an offset/limit pagination helper.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/raynetcrm/raynet-cli/internal/config"
)

// MaxPageSize is the REST API's documented maximum page size.
const MaxPageSize = 1000

// Client is an authenticated HTTP client for one RAYNET CRM profile.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	Profile config.Profile
	// Verbose, when true, prints each request line (method + URL) and the
	// resulting status code to Log — inspired by stripe-cli's -v flag.
	Verbose bool
	// Log receives the Verbose output; nil means os.Stderr.
	Log io.Writer
}

// New builds a Client for the given profile (its Host field determines the
// base URL; see config.Profile).
func New(p config.Profile) *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
			// The REST API never redirects; a CRM web app answers unknown
			// requests with a redirect to its login page. Do reports it.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		BaseURL: fmt.Sprintf("%s://%s/api/v2", p.GetScheme(), p.Host),
		Profile: p,
	}
}

// Do issues an authenticated request against path (e.g. "/company/") with the
// given query parameters and optional JSON body, returning the raw response.
func (c *Client) Do(method, path string, query url.Values, body any) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", path, err)
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, u.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if err := c.applyAuth(req); err != nil {
		return nil, err
	}

	if c.Verbose {
		fmt.Fprintf(c.log(), "> %s %s\n", req.Method, req.URL)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if c.Verbose {
		fmt.Fprintf(c.log(), "< %s\n", resp.Status)
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%s %s: the server redirected to %q, so %s is not the RAYNET CRM REST API (a CRM web app redirects to its login page)",
			method, path, resp.Header.Get("Location"), u.Host)
	}
	return resp, nil
}

func (c *Client) log() io.Writer {
	if c.Log != nil {
		return c.Log
	}
	return os.Stderr
}

func (c *Client) applyAuth(req *http.Request) error {
	switch c.Profile.AuthType {
	case config.AuthBearer:
		if c.Profile.Token == "" {
			return fmt.Errorf("host %s is configured for bearer auth but has no token stored", req.URL.Host)
		}
		req.Header.Set("Authorization", "Bearer "+c.Profile.Token)
	case config.AuthAPIKey:
		if c.Profile.Username == "" || c.Profile.APIKey == "" {
			return fmt.Errorf("host %s is configured for apikey auth but is missing username/apiKey", req.URL.Host)
		}
		req.SetBasicAuth(c.Profile.Username, c.Profile.APIKey)
		switch {
		case c.Profile.InstanceName != "":
			req.Header.Set("X-Instance-Name", c.Profile.InstanceName)
		case c.Profile.InstanceID != "":
			req.Header.Set("X-Instance-Id", c.Profile.InstanceID)
		default:
			return fmt.Errorf("host %s is missing instanceName/instanceId", req.URL.Host)
		}
	default:
		return fmt.Errorf("unknown auth type %q for host %s", c.Profile.AuthType, req.URL.Host)
	}
	return nil
}

// DecodeJSON runs resp through json.Decoder into v, closing the body and
// surfacing non-2xx statuses as an *APIError.
func DecodeJSON(resp *http.Response, v any) error {
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode >= 300 {
		return newAPIError(resp.StatusCode, data)
	}
	if v == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// ListPage is one page of a list endpoint.
type ListPage struct {
	Records []json.RawMessage
	// TotalCount is the number of records matching the query across all
	// pages, as the API reports it; -1 when the response doesn't say.
	TotalCount int
}

// GetPage fetches one page of a list endpoint: query plus offset and limit.
func (c *Client) GetPage(path string, query url.Values, offset, limit int) (ListPage, error) {
	q := cloneValues(query)
	if q == nil {
		q = url.Values{}
	}
	q.Set("offset", strconv.Itoa(offset))
	q.Set("limit", strconv.Itoa(limit))
	resp, err := c.Do(http.MethodGet, path, q, nil)
	if err != nil {
		return ListPage{}, err
	}
	return DecodeListPage(resp)
}

// GetAllPages walks a list endpoint with offset/limit paging (page size
// MaxPageSize) until a page returns fewer items than requested, accumulating
// every item as raw JSON. Handles both array and wrapped object responses.
func (c *Client) GetAllPages(path string, query url.Values) ([]json.RawMessage, error) {
	var all []json.RawMessage
	for offset := 0; ; offset += MaxPageSize {
		page, err := c.GetPage(path, query, offset, MaxPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Records...)
		if len(page.Records) < MaxPageSize {
			return all, nil
		}
	}
}

// DecodeListResponse handles both array responses and wrapped object responses
// (where the data is in a field like "data" or the object itself is the list).
func DecodeListResponse(resp *http.Response) ([]json.RawMessage, error) {
	page, err := DecodeListPage(resp)
	return page.Records, err
}

// DecodeListPage decodes a list response, bare array or wrapped object, into
// its records and the totalCount the wrapper carries, if any.
func DecodeListPage(resp *http.Response) (ListPage, error) {
	page := ListPage{TotalCount: -1}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return page, fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode >= 300 {
		return page, newAPIError(resp.StatusCode, data)
	}

	// Try unmarshaling as a direct array first
	if err := json.Unmarshal(data, &page.Records); err == nil {
		return page, nil
	}

	// If that fails, try as an object with a "data" field (or fallback field names)
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return page, fmt.Errorf("response is neither an array nor a JSON object")
	}
	page.TotalCount = decodeCount(obj["totalCount"])

	// Try common field names for wrapped list responses
	for _, key := range []string{"data", "items", "records", "results"} {
		if val, ok := obj[key]; ok {
			var arr []json.RawMessage
			if err := json.Unmarshal(val, &arr); err == nil {
				page.Records = arr
				return page, nil
			}
		}
	}

	// If no common field found, return empty (might be a different response format)
	page.Records = []json.RawMessage{}
	return page, nil
}

// decodeCount reads a count sent as a JSON number or a numeric string; -1
// when absent or unreadable.
func decodeCount(raw json.RawMessage) int {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return -1
}

func cloneValues(v url.Values) url.Values {
	return maps.Clone(v)
}
