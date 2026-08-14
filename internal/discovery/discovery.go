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

// Package discovery implements an MCP server capability discovery client.
// It queries remote MCP servers for their advertised tools, resources, and
// prompts using the 2026-07-28 stateless HTTP specification (no initialize
// handshake required).
package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// --------------------------------------------------------------------------
// JSON-RPC types
// --------------------------------------------------------------------------

// JSONRPCRequest represents a JSON-RPC 2.0 request object.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response object.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// --------------------------------------------------------------------------
// Discovery result types
// --------------------------------------------------------------------------

// Capabilities holds the discovered capabilities of an MCP server.
type Capabilities struct {
	Tools     []string
	Resources []string
	Prompts   []string
}

// ToolInfo describes a single tool advertised by an MCP server.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolsListResult is the result payload for the tools/list JSON-RPC method.
type ToolsListResult struct {
	Tools []ToolInfo `json:"tools"`
}

// resourceInfo describes a single resource advertised by an MCP server.
type resourceInfo struct {
	Name string `json:"name"`
}

type resourcesListResult struct {
	Resources []resourceInfo `json:"resources"`
}

// promptInfo describes a single prompt advertised by an MCP server.
type promptInfo struct {
	Name string `json:"name"`
}

type promptsListResult struct {
	Prompts []promptInfo `json:"prompts"`
}

// --------------------------------------------------------------------------
// Client
// --------------------------------------------------------------------------

// Client is an MCP server capability discovery client.
type Client struct {
	httpClient *http.Client
	idCounter  atomic.Int64
}

// NewClient creates a new discovery Client with the given HTTP timeout.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Discover queries an MCP server at baseURL+endpoint and returns its
// advertised capabilities. It calls tools/list, resources/list, and
// prompts/list in sequence.
func (c *Client) Discover(ctx context.Context, baseURL, endpoint string) (*Capabilities, error) {
	url := baseURL + endpoint

	tools, err := c.listTools(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}

	resources, err := c.listResources(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("listing resources: %w", err)
	}

	prompts, err := c.listPrompts(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("listing prompts: %w", err)
	}

	return &Capabilities{
		Tools:     tools,
		Resources: resources,
		Prompts:   prompts,
	}, nil
}

// --------------------------------------------------------------------------
// Private helpers
// --------------------------------------------------------------------------

func (c *Client) listTools(ctx context.Context, url string) ([]string, error) {
	raw, err := c.call(ctx, url, "tools/list")
	if err != nil {
		return nil, err
	}

	var result ToolsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding tools/list result: %w", err)
	}

	names := make([]string, 0, len(result.Tools))
	for _, t := range result.Tools {
		names = append(names, t.Name)
	}
	return names, nil
}

func (c *Client) listResources(ctx context.Context, url string) ([]string, error) {
	raw, err := c.call(ctx, url, "resources/list")
	if err != nil {
		return nil, err
	}

	var result resourcesListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding resources/list result: %w", err)
	}

	names := make([]string, 0, len(result.Resources))
	for _, r := range result.Resources {
		names = append(names, r.Name)
	}
	return names, nil
}

func (c *Client) listPrompts(ctx context.Context, url string) ([]string, error) {
	raw, err := c.call(ctx, url, "prompts/list")
	if err != nil {
		return nil, err
	}

	var result promptsListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decoding prompts/list result: %w", err)
	}

	names := make([]string, 0, len(result.Prompts))
	for _, p := range result.Prompts {
		names = append(names, p.Name)
	}
	return names, nil
}

// call sends a JSON-RPC 2.0 POST request and returns the raw result payload.
func (c *Client) call(ctx context.Context, url, method string) (json.RawMessage, error) {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.idCounter.Add(1),
		Method:  method,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s %s", resp.StatusCode, method, url)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("decoding JSON-RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}

	return rpcResp.Result, nil
}
