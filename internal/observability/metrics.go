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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	// Meter is the package-level OTel meter for mcp-gateway.
	Meter = otel.Meter("mcp-gateway")

	// ServersTotal tracks the current number of MCPServer resources.
	ServersTotal metric.Int64UpDownCounter

	// AgentsTotal tracks the current number of MCPAgent resources.
	AgentsTotal metric.Int64UpDownCounter

	// ReconcileDuration records the duration of reconcile loops in seconds.
	ReconcileDuration metric.Float64Histogram

	// ReconcileErrors counts the total number of reconcile errors.
	ReconcileErrors metric.Int64Counter

	// ToolCallsTotal counts the total number of MCP tool invocations.
	ToolCallsTotal metric.Int64Counter
)

// InitMetrics creates all counters and histograms used by the operator.
func InitMetrics() error {
	var err error

	ServersTotal, err = Meter.Int64UpDownCounter("mcpgateway.servers.total",
		metric.WithDescription("Current number of MCPServer resources"),
		metric.WithUnit("{server}"),
	)
	if err != nil {
		return err
	}

	AgentsTotal, err = Meter.Int64UpDownCounter("mcpgateway.agents.total",
		metric.WithDescription("Current number of MCPAgent resources"),
		metric.WithUnit("{agent}"),
	)
	if err != nil {
		return err
	}

	ReconcileDuration, err = Meter.Float64Histogram("mcpgateway.reconcile.duration",
		metric.WithDescription("Duration of reconcile loops"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return err
	}

	ReconcileErrors, err = Meter.Int64Counter("mcpgateway.reconcile.errors",
		metric.WithDescription("Total number of reconcile errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	ToolCallsTotal, err = Meter.Int64Counter("mcpgateway.toolcalls.total",
		metric.WithDescription("Total number of MCP tool invocations"),
		metric.WithUnit("{call}"),
	)
	if err != nil {
		return err
	}

	return nil
}
