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

package observability

import (
	"context"
	"testing"
)

func TestInitOTel_EmptyEndpoint(t *testing.T) {
	shutdown, err := InitOTel(context.Background(), "test-service", "")
	if err != nil {
		t.Fatalf("expected no error with empty endpoint, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	// The no-op shutdown should succeed without error.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown returned error: %v", err)
	}
}

func TestInitOTel_WithEndpoint(t *testing.T) {
	// Use a localhost endpoint that likely has nothing listening.
	// InitOTel should still succeed because gRPC connections are lazy.
	shutdown, err := InitOTel(context.Background(), "test-service", "localhost:4317")
	if err != nil {
		t.Fatalf("expected no error on init, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}
	// Shutdown may log warnings about failed connections, but should not error
	// fatally during tests. We call it to clean up global providers.
	_ = shutdown(context.Background())
}

func TestInitMetrics(t *testing.T) {
	if err := InitMetrics(); err != nil {
		t.Fatalf("InitMetrics failed: %v", err)
	}

	if ServersTotal == nil {
		t.Error("ServersTotal is nil after InitMetrics")
	}
	if AgentsTotal == nil {
		t.Error("AgentsTotal is nil after InitMetrics")
	}
	if ReconcileDuration == nil {
		t.Error("ReconcileDuration is nil after InitMetrics")
	}
	if ReconcileErrors == nil {
		t.Error("ReconcileErrors is nil after InitMetrics")
	}
	if ToolCallsTotal == nil {
		t.Error("ToolCallsTotal is nil after InitMetrics")
	}
}
