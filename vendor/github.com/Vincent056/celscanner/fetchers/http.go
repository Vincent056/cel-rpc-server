/*
Copyright © 2024 Red Hat Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fetchers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Vincent056/celscanner"
)

// HTTPFetcher implements InputFetcher for HTTP API endpoints
type HTTPFetcher struct {
	// HTTP client with timeout
	client *http.Client
	// Default timeout for requests
	defaultTimeout time.Duration
	// Whether to follow redirects
	followRedirects bool
	// Maximum number of retries
	maxRetries int
}

// HTTPResult represents the result of an HTTP request
type HTTPResult struct {
	// Response status code
	StatusCode int `json:"statusCode"`
	// Response headers
	Headers map[string][]string `json:"headers"`
	// Response body (parsed if JSON, raw if not)
	Body interface{} `json:"body"`
	// Raw response body as string
	RawBody string `json:"rawBody"`
	// Response time in milliseconds
	ResponseTime int64 `json:"responseTime"`
	// Whether the request was successful (2xx status code)
	Success bool `json:"success"`
	// Error message if any
	Error string `json:"error,omitempty"`
	// Request metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewHTTPFetcher creates a new HTTP input fetcher
func NewHTTPFetcher(timeout time.Duration, followRedirects bool, maxRetries int) *HTTPFetcher {
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}

	if maxRetries < 0 {
		maxRetries = 3 // Default retries
	}

	client := &http.Client{
		Timeout: timeout,
	}

	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &HTTPFetcher{
		client:          client,
		defaultTimeout:  timeout,
		followRedirects: followRedirects,
		maxRetries:      maxRetries,
	}
}

// FetchInputs retrieves HTTP resources for the specified inputs
func (h *HTTPFetcher) FetchInputs(inputs []celscanner.Input, variables []celscanner.CelVariable) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, input := range inputs {
		if input.Type() != celscanner.InputTypeHTTP {
			continue
		}

		httpSpec, ok := input.Spec().(celscanner.HTTPInputSpec)
		if !ok {
			return nil, fmt.Errorf("invalid HTTP input spec for input %s", input.Name())
		}

		data, err := h.fetchHTTPResource(httpSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch HTTP resource for input %s: %w", input.Name(), err)
		}

		result[input.Name()] = data
	}

	return result, nil
}

// SupportsInputType returns true for HTTP input types
func (h *HTTPFetcher) SupportsInputType(inputType celscanner.InputType) bool {
	return inputType == celscanner.InputTypeHTTP
}

// fetchHTTPResource retrieves a specific HTTP resource
func (h *HTTPFetcher) fetchHTTPResource(spec celscanner.HTTPInputSpec) (interface{}, error) {
	var result *HTTPResult
	var err error

	// Retry logic
	for attempt := 0; attempt <= h.maxRetries; attempt++ {
		result, err = h.makeHTTPRequest(spec)
		if err == nil || attempt == h.maxRetries {
			break
		}

		// Wait before retry
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	if err != nil {
		return nil, err
	}

	// Convert HTTPResult to map[string]interface{} for CEL compatibility
	return h.httpResultToMap(result), nil
}

// makeHTTPRequest performs the actual HTTP request
func (h *HTTPFetcher) makeHTTPRequest(spec celscanner.HTTPInputSpec) (*HTTPResult, error) {
	start := time.Now()

	// Prepare request
	method := spec.Method()
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if len(spec.Body()) > 0 {
		bodyReader = bytes.NewReader(spec.Body())
	}

	req, err := http.NewRequest(method, spec.URL(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range spec.Headers() {
		req.Header.Set(key, value)
	}

	// Set default Content-Type for POST/PUT requests with body
	if (method == "POST" || method == "PUT") && len(spec.Body()) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), h.defaultTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := h.client.Do(req)
	if err != nil {
		return &HTTPResult{
			Success:      false,
			Error:        err.Error(),
			ResponseTime: time.Since(start).Milliseconds(),
			Metadata: map[string]interface{}{
				"url":    spec.URL(),
				"method": method,
			},
		}, nil
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &HTTPResult{
			StatusCode:   resp.StatusCode,
			Success:      false,
			Error:        fmt.Sprintf("failed to read response body: %v", err),
			ResponseTime: time.Since(start).Milliseconds(),
			Metadata: map[string]interface{}{
				"url":    spec.URL(),
				"method": method,
			},
		}, nil
	}

	// Parse response
	result := &HTTPResult{
		StatusCode:   resp.StatusCode,
		Headers:      resp.Header,
		RawBody:      string(body),
		ResponseTime: time.Since(start).Milliseconds(),
		Success:      resp.StatusCode >= 200 && resp.StatusCode < 300,
		Metadata: map[string]interface{}{
			"url":     spec.URL(),
			"method":  method,
			"headers": spec.Headers(),
		},
	}

	// Try to parse JSON response
	if len(body) > 0 {
		var jsonBody interface{}
		if err := json.Unmarshal(body, &jsonBody); err == nil {
			result.Body = jsonBody
		} else {
			// If not JSON, store as string
			result.Body = string(body)
		}
	}

	return result, nil
}

// httpResultToMap converts HTTPResult to map[string]interface{} for CEL compatibility
func (h *HTTPFetcher) httpResultToMap(result *HTTPResult) map[string]interface{} {
	return map[string]interface{}{
		"statusCode":   result.StatusCode,
		"headers":      result.Headers,
		"body":         result.Body,
		"rawBody":      result.RawBody,
		"responseTime": result.ResponseTime,
		"success":      result.Success,
		"error":        result.Error,
		"metadata":     result.Metadata,
	}
}

// Helper functions for HTTP operations

// Get performs a GET request
func Get(url string, headers map[string]string) (*HTTPResult, error) {
	fetcher := NewHTTPFetcher(30*time.Second, true, 3)
	spec := &celscanner.HTTPInput{
		Endpoint:    url,
		HTTPMethod:  "GET",
		HTTPHeaders: headers,
	}
	data, err := fetcher.fetchHTTPResource(spec)
	if err != nil {
		return nil, err
	}

	// Convert back to HTTPResult for direct usage
	resultMap := data.(map[string]interface{})
	return &HTTPResult{
		StatusCode:   resultMap["statusCode"].(int),
		Headers:      resultMap["headers"].(map[string][]string),
		Body:         resultMap["body"],
		RawBody:      resultMap["rawBody"].(string),
		ResponseTime: resultMap["responseTime"].(int64),
		Success:      resultMap["success"].(bool),
		Error:        resultMap["error"].(string),
		Metadata:     resultMap["metadata"].(map[string]interface{}),
	}, nil
}

// Post performs a POST request
func Post(url string, body []byte, headers map[string]string) (*HTTPResult, error) {
	fetcher := NewHTTPFetcher(30*time.Second, true, 3)
	spec := &celscanner.HTTPInput{
		Endpoint:    url,
		HTTPMethod:  "POST",
		HTTPHeaders: headers,
		HTTPBody:    body,
	}
	data, err := fetcher.fetchHTTPResource(spec)
	if err != nil {
		return nil, err
	}

	// Convert back to HTTPResult for direct usage
	resultMap := data.(map[string]interface{})
	return &HTTPResult{
		StatusCode:   resultMap["statusCode"].(int),
		Headers:      resultMap["headers"].(map[string][]string),
		Body:         resultMap["body"],
		RawBody:      resultMap["rawBody"].(string),
		ResponseTime: resultMap["responseTime"].(int64),
		Success:      resultMap["success"].(bool),
		Error:        resultMap["error"].(string),
		Metadata:     resultMap["metadata"].(map[string]interface{}),
	}, nil
}

// ValidateHTTPInputSpec validates an HTTP input specification
func ValidateHTTPInputSpec(spec celscanner.HTTPInputSpec) error {
	if spec.URL() == "" {
		return fmt.Errorf("URL is required for HTTP input")
	}

	method := strings.ToUpper(spec.Method())
	if method != "" && method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" && method != "HEAD" && method != "OPTIONS" {
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	return nil
}

// Example usage:
//
// // Create HTTP fetcher
// fetcher := NewHTTPFetcher(30*time.Second, true, 3)
//
// // Create HTTP input for API endpoint
// input := celscanner.NewHTTPInput("api", "https://api.example.com/users", "GET", map[string]string{"Authorization": "Bearer token"}, nil)
// data, err := fetcher.FetchInputs([]celscanner.Input{input}, nil)
//
// // Use helper functions
// result, err := Get("https://api.example.com/health", map[string]string{"Accept": "application/json"})
// fmt.Printf("Status: %d, Body: %v\n", result.StatusCode, result.Body)
