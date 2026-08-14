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
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Version is the version of the observability package, embedded in OTel resources.
var Version = "0.1.0"

// InitOTel initializes OpenTelemetry with OTLP gRPC exporters.
// Returns a shutdown function. If collectorEndpoint is empty, returns a no-op.
func InitOTel(ctx context.Context, serviceName, collectorEndpoint string) (func(context.Context) error, error) {
	// If no endpoint is configured, return a no-op shutdown so the operator
	// can run without an OTel collector.
	if collectorEndpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(Version),
		),
	)
	if err != nil {
		return nil, err
	}

	// --- Traces ---
	conn, err := grpc.NewClient(collectorEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	// --- Metrics ---
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(15*time.Second))),
	)
	otel.SetMeterProvider(mp)

	// Combined shutdown function.
	shutdown := func(ctx context.Context) error {
		return errors.Join(
			tp.Shutdown(ctx),
			mp.Shutdown(ctx),
			conn.Close(),
		)
	}
	return shutdown, nil
}
