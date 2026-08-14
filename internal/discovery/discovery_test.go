/*
Copyright 2026 The MCP Gateway Authors.

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

package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newJSONRPCResponse builds a raw JSON-RPC 2.0 success response body for the
// given result payload.
func newJSONRPCResponse(t *testing.T, id int64, result interface{}) []byte {
	t.Helper()
	resultBytes, err := json.Marshal(result)
	require.NoError(t, err)

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultBytes,
	}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

// newJSONRPCErrorResponse builds a raw JSON-RPC 2.0 error response body.
func newJSONRPCErrorResponse(t *testing.T, id int64, code int, message string) []byte {
	t.Helper()
	resp := struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      int64         `json:"id"`
		Error   *JSONRPCError `json:"error"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

// mockMCPServer returns an httptest.Server that responds to tools/list,
// resources/list, and prompts/list with the supplied payloads. callCount
// tracks the auto-incrementing JSON-RPC request ID so responses can use
// matching IDs.
func mockMCPServer(t *testing.T, tools *ToolsListResult, resources *resourcesListResult, prompts *promptsListResult) *httptest.Server {
	t.Helper()

	var callCount int64

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		callCount++

		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case "tools/list":
			_, _ = w.Write(newJSONRPCResponse(t, req.ID, tools))
		case "resources/list":
			_, _ = w.Write(newJSONRPCResponse(t, req.ID, resources))
		case "prompts/list":
			_, _ = w.Write(newJSONRPCResponse(t, req.ID, prompts))
		default:
			_, _ = w.Write(newJSONRPCErrorResponse(t, req.ID, -32601, "method not found"))
		}
	}))
}

func TestDiscover_WithTools(t *testing.T) {
	tools := &ToolsListResult{
		Tools: []ToolInfo{
			{Name: "read_file", Description: "Read a file from disk"},
			{Name: "write_file", Description: "Write a file to disk"},
			{Name: "list_dir", Description: "List directory contents"},
		},
	}
	resources := &resourcesListResult{}
	prompts := &promptsListResult{}

	srv := mockMCPServer(t, tools, resources, prompts)
	defer srv.Close()

	client := NewClient(5 * time.Second)
	caps, err := client.Discover(context.Background(), srv.URL, "/mcp")
	require.NoError(t, err)

	assert.Len(t, caps.Tools, 3)
	assert.Contains(t, caps.Tools, "read_file")
	assert.Contains(t, caps.Tools, "write_file")
	assert.Contains(t, caps.Tools, "list_dir")
	assert.Empty(t, caps.Resources)
	assert.Empty(t, caps.Prompts)
}

func TestDiscover_WithResources(t *testing.T) {
	tools := &ToolsListResult{}
	resources := &resourcesListResult{
		Resources: []resourceInfo{
			{Name: "config://app-settings"},
			{Name: "file://schema.json"},
		},
	}
	prompts := &promptsListResult{}

	srv := mockMCPServer(t, tools, resources, prompts)
	defer srv.Close()

	client := NewClient(5 * time.Second)
	caps, err := client.Discover(context.Background(), srv.URL, "/mcp")
	require.NoError(t, err)

	assert.Empty(t, caps.Tools)
	assert.Len(t, caps.Resources, 2)
	assert.Contains(t, caps.Resources, "config://app-settings")
	assert.Contains(t, caps.Resources, "file://schema.json")
	assert.Empty(t, caps.Prompts)
}

func TestDiscover_EmptyServer(t *testing.T) {
	tools := &ToolsListResult{}
	resources := &resourcesListResult{}
	prompts := &promptsListResult{}

	srv := mockMCPServer(t, tools, resources, prompts)
	defer srv.Close()

	client := NewClient(5 * time.Second)
	caps, err := client.Discover(context.Background(), srv.URL, "/mcp")
	require.NoError(t, err)

	assert.Empty(t, caps.Tools)
	assert.Empty(t, caps.Resources)
	assert.Empty(t, caps.Prompts)
}

func TestDiscover_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	caps, err := client.Discover(context.Background(), srv.URL, "/mcp")

	require.Error(t, err)
	assert.Nil(t, caps)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestDiscover_JSONRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(newJSONRPCErrorResponse(t, req.ID, -32603, "internal error"))
	}))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	caps, err := client.Discover(context.Background(), srv.URL, "/mcp")

	require.Error(t, err)
	assert.Nil(t, caps)
	assert.Contains(t, err.Error(), "internal error")
}
