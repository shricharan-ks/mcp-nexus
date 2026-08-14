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

package audit

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --------------------------------------------------------------------------
// Mock sink
// --------------------------------------------------------------------------

type mockSink struct {
	mu      sync.Mutex
	entries []Entry
}

func (m *mockSink) Write(_ context.Context, entry Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockSink) getEntries() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	dst := make([]Entry, len(m.entries))
	copy(dst, m.entries)
	return dst
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestLog_WritesToSlog(t *testing.T) {
	logger := NewLogger()

	entry := Entry{
		Timestamp:   time.Now(),
		AgentID:     "agent-1",
		Server:      "my-server",
		Tool:        "read_file",
		Method:      "tools/call",
		Status:      "success",
		StatusCode:  200,
		DurationMs:  42,
		RequestHash: "abc123def456",
	}

	// Should not panic.
	assert.NotPanics(t, func() {
		logger.Log(context.Background(), entry)
	})
}

func TestLog_CallsSinks(t *testing.T) {
	sink := &mockSink{}
	logger := NewLogger(sink)

	entry := Entry{
		Timestamp:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		AgentID:     "agent-2",
		Server:      "prod-server",
		Tool:        "write_file",
		Method:      "tools/call",
		Status:      "denied",
		StatusCode:  403,
		DurationMs:  5,
		RequestHash: "deadbeef1234",
		ClientIP:    "10.0.0.1",
		Error:       "permission denied",
	}

	logger.Log(context.Background(), entry)

	// Sinks are called in goroutines; give them a moment to complete.
	assert.Eventually(t, func() bool {
		return len(sink.getEntries()) == 1
	}, time.Second, 10*time.Millisecond)

	got := sink.getEntries()[0]
	assert.Equal(t, "agent-2", got.AgentID)
	assert.Equal(t, "prod-server", got.Server)
	assert.Equal(t, "write_file", got.Tool)
	assert.Equal(t, "tools/call", got.Method)
	assert.Equal(t, "denied", got.Status)
	assert.Equal(t, 403, got.StatusCode)
	assert.Equal(t, int64(5), got.DurationMs)
	assert.Equal(t, "deadbeef1234", got.RequestHash)
	assert.Equal(t, "10.0.0.1", got.ClientIP)
	assert.Equal(t, "permission denied", got.Error)
}

func TestHashRequestBody(t *testing.T) {
	body := strings.NewReader(`{"method":"tools/call","params":{"name":"read_file"}}`)
	hash := HashRequestBody(body)

	assert.Len(t, hash, 12, "hash should be exactly 12 hex characters")

	// Verify determinism: hashing the same content again should produce the
	// same result.
	body2 := strings.NewReader(`{"method":"tools/call","params":{"name":"read_file"}}`)
	hash2 := HashRequestBody(body2)

	assert.Equal(t, hash, hash2, "hash should be deterministic")
}

func TestHashRequestBody_Empty(t *testing.T) {
	body := strings.NewReader("")
	hash := HashRequestBody(body)

	assert.Len(t, hash, 12, "hash of empty body should still be 12 hex characters")

	// Verify determinism for empty input.
	body2 := strings.NewReader("")
	hash2 := HashRequestBody(body2)

	assert.Equal(t, hash, hash2, "hash of empty body should be deterministic")
}
