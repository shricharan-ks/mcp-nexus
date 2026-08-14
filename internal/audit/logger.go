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

// Package audit provides structured audit logging for MCP Gateway requests.
// It writes JSON-formatted log entries via slog and forwards them to
// pluggable sinks (e.g. webhook, database) for long-term retention.
package audit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// --------------------------------------------------------------------------
// Types
// --------------------------------------------------------------------------

// Entry represents a single audit log record for an MCP request.
type Entry struct {
	Timestamp   time.Time `json:"timestamp"`
	AgentID     string    `json:"agentId"`
	Server      string    `json:"server"`
	Tool        string    `json:"tool"`
	Method      string    `json:"method"`
	Status      string    `json:"status"`      // success, denied, error, rate_limited
	StatusCode  int       `json:"statusCode"`
	DurationMs  int64     `json:"durationMs"`
	RequestHash string    `json:"requestHash"`
	ClientIP    string    `json:"clientIp,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// Sink is an interface for audit log destinations.
type Sink interface {
	Write(ctx context.Context, entry Entry) error
}

// Logger writes structured audit entries and dispatches them to sinks.
type Logger struct {
	slogger *slog.Logger
	sinks   []Sink
}

// --------------------------------------------------------------------------
// Constructor
// --------------------------------------------------------------------------

// NewLogger creates a new audit Logger. Entries are always written as
// structured JSON to stdout via slog. Additional sinks receive a copy of
// each entry asynchronously.
func NewLogger(sinks ...Sink) *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{
		slogger: slog.New(handler),
		sinks:   sinks,
	}
}

// --------------------------------------------------------------------------
// Logging
// --------------------------------------------------------------------------

// Log writes an audit entry to the structured logger and dispatches it to
// all registered sinks in separate goroutines.
func (l *Logger) Log(ctx context.Context, entry Entry) {
	attrs := []slog.Attr{
		slog.Time("timestamp", entry.Timestamp),
		slog.String("agentId", entry.AgentID),
		slog.String("server", entry.Server),
		slog.String("tool", entry.Tool),
		slog.String("method", entry.Method),
		slog.String("status", entry.Status),
		slog.Int("statusCode", entry.StatusCode),
		slog.Int64("durationMs", entry.DurationMs),
		slog.String("requestHash", entry.RequestHash),
	}
	if entry.ClientIP != "" {
		attrs = append(attrs, slog.String("clientIp", entry.ClientIP))
	}
	if entry.Error != "" {
		attrs = append(attrs, slog.String("error", entry.Error))
	}

	// Convert []slog.Attr to []any for LogAttrs.
	l.slogger.LogAttrs(ctx, slog.LevelInfo, "audit", attrs...)

	for _, s := range l.sinks {
		go func(sink Sink) {
			_ = sink.Write(ctx, entry)
		}(s)
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// HashRequestBody computes a SHA-256 hash of the given reader's contents
// and returns the first 12 hex characters. This provides a short,
// deterministic fingerprint of the request body for correlation purposes.
func HashRequestBody(body io.Reader) string {
	h := sha256.New()
	_, _ = io.Copy(h, body)
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}
