# Phase 4: Observability (Weeks 14-16)

Build full-stack observability: distributed tracing through Envoy and MCP servers, custom metrics for operator and proxy health, MLflow integration for AI-workflow tracking, and pre-built Grafana dashboards.

---

## Step 1: OTel Collector DaemonSet

Deploy the OpenTelemetry Collector as a DaemonSet so every node forwards traces, metrics, and logs through a unified pipeline.

### Files

```
deploy/helm/templates/otel-collector-daemonset.yaml
deploy/helm/templates/otel-collector-configmap.yaml
deploy/helm/templates/otel-collector-serviceaccount.yaml
deploy/helm/templates/otel-collector-service.yaml
deploy/helm/values.yaml  (add observability section)
```

### Key Code

**deploy/helm/templates/otel-collector-configmap.yaml**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-otel-collector
  namespace: {{ .Release.Namespace }}
data:
  config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318

    processors:
      batch:
        send_batch_size: 512
        timeout: 5s
        send_batch_max_size: 1024

      memory_limiter:
        check_interval: 1s
        limit_mib: 512
        spike_limit_mib: 128

      attributes/mcp:
        actions:
          - key: mcp.server.name
            action: upsert
            from_attribute: k8s.deployment.name
          - key: mcp.server.namespace
            action: upsert
            from_attribute: k8s.namespace.name
          - key: mcp.gateway.version
            action: upsert
            value: "{{ .Chart.AppVersion }}"

    exporters:
      prometheus:
        endpoint: 0.0.0.0:8889
        metric_expiration: 5m
        resource_to_telemetry_conversion:
          enabled: true

      otlphttp/mlflow:
        endpoint: http://{{ .Release.Name }}-mlflow:5000/api/2.0/mlflow
        headers:
          Content-Type: application/x-protobuf
        retry_on_failure:
          enabled: true
          initial_interval: 5s
          max_interval: 30s

      debug:
        verbosity: basic

    extensions:
      health_check:
        endpoint: 0.0.0.0:13133

    service:
      extensions: [health_check]
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch, attributes/mcp]
          exporters: [otlphttp/mlflow, debug]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, batch, attributes/mcp]
          exporters: [prometheus]
```

**deploy/helm/templates/otel-collector-daemonset.yaml**

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ .Release.Name }}-otel-collector
  namespace: {{ .Release.Namespace }}
  labels:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/part-of: mcp-gateway
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: otel-collector
  template:
    metadata:
      labels:
        app.kubernetes.io/name: otel-collector
      annotations:
        checksum/config: {{ include (print .Template.BasePath "/otel-collector-configmap.yaml") . | sha256sum }}
    spec:
      serviceAccountName: {{ .Release.Name }}-otel-collector
      containers:
        - name: otel-collector
          image: "{{ .Values.observability.collector.image }}:{{ .Values.observability.collector.tag }}"
          args: ["--config=/etc/otel/config.yaml"]
          ports:
            - name: otlp-grpc
              containerPort: 4317
              protocol: TCP
            - name: otlp-http
              containerPort: 4318
              protocol: TCP
            - name: prometheus
              containerPort: 8889
              protocol: TCP
            - name: health
              containerPort: 13133
              protocol: TCP
          volumeMounts:
            - name: config
              mountPath: /etc/otel
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
          livenessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 10
          readinessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 5
      volumes:
        - name: config
          configMap:
            name: {{ .Release.Name }}-otel-collector
```

### Quality Gate

- Collector pods running on every node (`kubectl get ds` shows DESIRED == READY).
- Health endpoint returns 200 on port 13133.
- Prometheus scrape endpoint on 8889 returns `otelcol_receiver_accepted_spans` metric.

### Testing Command

```bash
# Verify DaemonSet rollout
kubectl rollout status daemonset/mcp-gateway-otel-collector -n mcp-system --timeout=120s

# Send a test span
kubectl run otel-test --rm -it --restart=Never \
  --image=ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:latest \
  -- traces --otlp-endpoint=mcp-gateway-otel-collector:4317 --otlp-insecure --traces 10

# Check prometheus metrics
kubectl port-forward ds/mcp-gateway-otel-collector 8889:8889 -n mcp-system &
curl -s http://localhost:8889/metrics | grep otelcol_receiver_accepted_spans
```

### Pitfalls

- **Memory pressure on small nodes:** The `memory_limiter` processor is critical; without it the collector will OOM when backpressure builds from a slow MLflow exporter. Always set `spike_limit_mib` to at least 25% of `limit_mib`.
- **ConfigMap update lag:** Changes to the ConfigMap do not auto-restart DaemonSet pods. The `checksum/config` annotation handles this, but only on `helm upgrade`, not on raw `kubectl apply`.
- **Port conflicts:** If another OTel Collector is already running (e.g., from a cluster-level install), the 4317/4318 ports will collide. Use `hostPort` only if strictly needed; prefer ClusterIP service.

### Progress Marker

- [ ] ConfigMap with complete pipeline config deployed
- [ ] DaemonSet running on all nodes
- [ ] Health check passing
- [ ] Prometheus metrics endpoint returning data
- [ ] Test span visible in debug exporter logs

---

## Step 2: Operator OTel Instrumentation

Instrument the operator controller with OpenTelemetry SDK: structured spans around every reconcile loop, custom counters for reconciliation outcomes, and histograms for reconcile duration.

### Files

```
internal/observability/otel.go
internal/observability/metrics.go
internal/controller/mcpserver_controller.go  (wrap Reconcile)
internal/controller/mcppolicy_controller.go  (wrap Reconcile)
cmd/operator/main.go                         (init OTel)
```

### Key Code

**internal/observability/otel.go**

```go
package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config holds OTel SDK configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string        // e.g. "otel-collector:4317"
	SampleRatio    float64       // 0.0 to 1.0
	BatchTimeout   time.Duration // flush interval
}

// InitTracing initializes the global TracerProvider and returns a shutdown func.
func InitTracing(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(cfg.BatchTimeout)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRatio),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
```

**internal/observability/metrics.go**

```go
package observability

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter("mcp-gateway-operator")

	// ReconcileTotal counts reconciliation attempts by controller and result.
	ReconcileTotal metric.Int64Counter

	// ReconcileDuration tracks reconcile latency in seconds.
	ReconcileDuration metric.Float64Histogram

	// MCPServersActive tracks the number of active MCP server instances.
	MCPServersActive metric.Int64UpDownCounter

	// EnvoyConfigPushTotal counts Envoy xDS config push events.
	EnvoyConfigPushTotal metric.Int64Counter

	// EnvoyConfigPushErrors counts failed Envoy config pushes.
	EnvoyConfigPushErrors metric.Int64Counter
)

func InitMetrics() error {
	var err error

	ReconcileTotal, err = meter.Int64Counter("mcpgateway.reconcile.total",
		metric.WithDescription("Total reconciliation attempts"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return err
	}

	ReconcileDuration, err = meter.Float64Histogram("mcpgateway.reconcile.duration",
		metric.WithDescription("Duration of reconciliation in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return err
	}

	MCPServersActive, err = meter.Int64UpDownCounter("mcpgateway.servers.active",
		metric.WithDescription("Number of active MCP server instances"),
		metric.WithUnit("{server}"),
	)
	if err != nil {
		return err
	}

	EnvoyConfigPushTotal, err = meter.Int64Counter("mcpgateway.envoy.config_push.total",
		metric.WithDescription("Total Envoy config push events"),
		metric.WithUnit("{push}"),
	)
	if err != nil {
		return err
	}

	EnvoyConfigPushErrors, err = meter.Int64Counter("mcpgateway.envoy.config_push.errors",
		metric.WithDescription("Failed Envoy config push events"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	return nil
}
```

**Reconciler span wrapping pattern (internal/controller/mcpserver_controller.go)**

```go
func (r *MCPServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tracer := otel.Tracer("mcpserver-controller")
	ctx, span := tracer.Start(ctx, "MCPServer.Reconcile",
		trace.WithAttributes(
			attribute.String("mcpserver.name", req.Name),
			attribute.String("mcpserver.namespace", req.Namespace),
		),
	)
	defer span.End()

	start := time.Now()
	result, err := r.reconcileInner(ctx, req)
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	observability.ReconcileDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("controller", "mcpserver"),
			attribute.String("status", status),
		),
	)
	observability.ReconcileTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("controller", "mcpserver"),
			attribute.String("status", status),
		),
	)

	return result, err
}
```

### Quality Gate

- `go vet ./internal/observability/...` passes with no errors.
- Reconcile spans appear in OTel Collector debug output when an MCPServer CR is created.
- `mcpgateway.reconcile.duration` histogram is exposed on the Prometheus metrics endpoint.

### Testing Command

```bash
# Unit test observability package
go test ./internal/observability/... -v -count=1

# Integration test: create MCPServer, verify span
make test-integration TEST_ARGS="-run TestReconcileTracing"

# Check metrics endpoint
kubectl port-forward deploy/mcp-gateway-operator 8080:8080 -n mcp-system &
curl -s http://localhost:8080/metrics | grep mcpgateway_reconcile
```

### Pitfalls

- **Span explosion during rapid reconciliation:** Controller-runtime requeues can trigger hundreds of reconciles per second. The `ParentBased(TraceIDRatioBased(...))` sampler is essential; start with 0.1 (10%) in production.
- **Metric cardinality:** Never add high-cardinality attributes (resource UID, IP address) to metrics. Use them only in trace spans.
- **Shutdown ordering:** The OTel TracerProvider must be shut down after the controller manager stops, or in-flight spans will be lost. Wire `shutdown` into the manager's context cancellation.

### Progress Marker

- [ ] `otel.go` and `metrics.go` compile and pass unit tests
- [ ] Operator main.go calls `InitTracing` and `InitMetrics`
- [ ] Reconcile spans visible in collector logs
- [ ] Custom metrics scraped by Prometheus
- [ ] No metric cardinality warnings in collector logs

---

## Step 3: Envoy MCP Tracing

Configure Envoy proxies for distributed tracing with MCP-specific access log fields and W3C trace context propagation.

### Files

```
internal/xds/tracing.go               (tracing config builder)
internal/xds/access_log.go            (access log format builder)
deploy/helm/templates/envoy-tracing-bootstrap.yaml
```

### Key Code

**Access log format with MCP fields**

```go
// internal/xds/access_log.go
package xds

const MCPAccessLogFormat = `{
  "timestamp": "%START_TIME%",
  "request_id": "%REQ(X-REQUEST-ID)%",
  "trace_id": "%REQ(TRACEPARENT)%",
  "method": "%REQ(:METHOD)%",
  "path": "%REQ(:PATH)%",
  "mcp_method": "%DYNAMIC_METADATA(envoy.filters.http.lua:mcp_method)%",
  "mcp_name": "%DYNAMIC_METADATA(envoy.filters.http.lua:mcp_name)%",
  "response_code": "%RESPONSE_CODE%",
  "response_flags": "%RESPONSE_FLAGS%",
  "duration_ms": "%DURATION%",
  "upstream_host": "%UPSTREAM_HOST%",
  "upstream_cluster": "%UPSTREAM_CLUSTER%",
  "bytes_sent": "%BYTES_SENT%",
  "bytes_received": "%BYTES_RECEIVED%"
}`
```

**W3C traceparent propagation in xDS tracing config**

```go
// internal/xds/tracing.go
package xds

import (
	tracingv3 "github.com/envoyproxy/go-control-plane/envoy/config/trace/v3"
	httpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// BuildTracingConfig returns an HCM tracing config that sends spans
// to the OTel Collector via OTLP/gRPC.
func BuildTracingConfig(collectorCluster string, serverName string) *httpv3.HttpConnectionManager_Tracing {
	return &httpv3.HttpConnectionManager_Tracing{
		Provider: &tracingv3.Tracing_Http{
			Name: "envoy.tracers.opentelemetry",
			TypedConfig: mustMarshalAny(&tracingv3.OpenTelemetryConfig{
				GrpcService: &corev3.GrpcService{
					TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
						EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
							ClusterName: collectorCluster,
						},
					},
				},
				ServiceName: "mcp-envoy-" + serverName,
			}),
		},
		ClientSampling:  &typev3.Percent{Value: 100},
		RandomSampling:  &typev3.Percent{Value: 10},
		OverallSampling: &typev3.Percent{Value: 100},
		SpawnUpstreamSpan: wrapperspb.Bool(true),
	}
}
```

**Per-server stat prefix**

```go
// When building listener filter chains, set a unique stat prefix per MCPServer
func statPrefix(server *v1alpha1.MCPServer) string {
	return fmt.Sprintf("mcp.%s.%s", server.Namespace, server.Name)
}
```

### Quality Gate

- Envoy access logs include `mcp_method` and `mcp_name` fields.
- `traceparent` header is propagated from client through Envoy to upstream MCP server.
- Per-server stats appear under `mcp.<namespace>.<name>.*` in Envoy admin `/stats`.

### Testing Command

```bash
# Integration test: verify trace propagation
make test-integration TEST_ARGS="-run TestEnvoyTracePropagation"

# Manual verification
kubectl port-forward svc/mcp-gateway-envoy 10000:10000 -n mcp-system &
curl -H "traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" \
     http://localhost:10000/mcp/test-server/sse

# Check Envoy stats
kubectl exec deploy/mcp-gateway-envoy -n mcp-system -- \
  curl -s http://localhost:9901/stats | grep "mcp\."
```

### Pitfalls

- **Lua filter ordering:** The Lua filter that extracts `mcp_method` and `mcp_name` from JSON-RPC bodies must run before the access log formatter reads dynamic metadata. Place it first in the HTTP filter chain.
- **Sampling rate mismatch:** If Envoy samples at 10% but the operator samples at 100%, you get orphaned parent spans. Align sampling rates or use parent-based sampling everywhere.
- **Large JSON-RPC bodies:** The Lua filter parsing the request body to extract `mcp_method` must use `body():getBytes(0, 4096)` to cap reads and avoid memory issues with large tool call payloads.

### Progress Marker

- [ ] Tracing config builder unit tested
- [ ] Access log format includes mcp_method and mcp_name
- [ ] traceparent header propagated end-to-end
- [ ] Per-server stat prefixes visible in Envoy admin
- [ ] Lua filter extracts MCP fields correctly

---

## Step 4: MLflow Tracking Server Deployment

Deploy MLflow as a tracking server for AI workflow observability. The OTel-to-MLflow converter translates spans into MLflow experiment runs.

### Files

```
deploy/helm/templates/mlflow-deployment.yaml
deploy/helm/templates/mlflow-service.yaml
deploy/helm/templates/mlflow-pvc.yaml
deploy/helm/templates/mlflow-postgresql.yaml
deploy/helm/templates/mlflow-minio.yaml
cmd/mlflow-converter/main.go
cmd/mlflow-converter/converter.go
cmd/mlflow-converter/Dockerfile
```

### Key Code

**deploy/helm/templates/mlflow-deployment.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-mlflow
  namespace: {{ .Release.Namespace }}
  labels:
    app.kubernetes.io/name: mlflow
    app.kubernetes.io/part-of: mcp-gateway
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: mlflow
  template:
    metadata:
      labels:
        app.kubernetes.io/name: mlflow
    spec:
      containers:
        - name: mlflow
          image: "{{ .Values.observability.mlflow.image }}:{{ .Values.observability.mlflow.tag }}"
          args:
            - server
            - --host=0.0.0.0
            - --port=5000
            - --backend-store-uri=postgresql://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@{{ .Release.Name }}-mlflow-postgresql:5432/mlflow
            - --default-artifact-root=s3://mlflow-artifacts/
          env:
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: {{ .Release.Name }}-mlflow-postgresql
                  key: username
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ .Release.Name }}-mlflow-postgresql
                  key: password
            - name: MLFLOW_S3_ENDPOINT_URL
              value: "http://{{ .Release.Name }}-minio:9000"
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: {{ .Release.Name }}-minio
                  key: access-key
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: {{ .Release.Name }}-minio
                  key: secret-key
          ports:
            - name: http
              containerPort: 5000
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 10
          resources:
            requests:
              cpu: 200m
              memory: 512Mi
            limits:
              cpu: "1"
              memory: 1Gi
```

**cmd/mlflow-converter/main.go**

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/grpc"
)

func main() {
	cfg := LoadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mlflowClient, err := NewMLflowClient(cfg.MLflowEndpoint)
	if err != nil {
		logger.Error("failed to create MLflow client", "error", err)
		os.Exit(1)
	}

	converter := &SpanConverter{
		mlflow: mlflowClient,
		logger: logger,
		experimentMapping: make(map[string]string), // traceID -> experimentID
	}

	// Start OTLP gRPC receiver
	server := grpc.NewServer()
	RegisterTraceReceiver(server, converter)

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Error("failed to listen", "addr", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()

	logger.Info("starting mlflow-converter", "addr", cfg.ListenAddr)
	if err := server.Serve(listener); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

**cmd/mlflow-converter/converter.go** (core mapping logic)

```go
package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type SpanConverter struct {
	mlflow            *MLflowClient
	logger            *slog.Logger
	experimentMapping map[string]string
}

// ConvertSpans maps OTel spans to MLflow runs.
// Mapping rules:
//   - Root span with mcp.server.name attribute -> MLflow experiment
//   - Each trace ID -> one MLflow run
//   - Span attributes -> MLflow run parameters
//   - Span duration -> MLflow metric "duration_ms"
//   - mcp_method -> MLflow run tag
func (c *SpanConverter) ConvertSpans(ctx context.Context, traces ptrace.Traces) error {
	resourceSpans := traces.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		serverName, ok := rs.Resource().Attributes().Get("mcp.server.name")
		if !ok {
			continue
		}

		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				traceID := span.TraceID().String()

				experimentID, err := c.ensureExperiment(ctx, serverName.AsString())
				if err != nil {
					c.logger.Error("failed to ensure experiment", "error", err)
					continue
				}

				runID, err := c.ensureRun(ctx, experimentID, traceID)
				if err != nil {
					c.logger.Error("failed to ensure run", "error", err)
					continue
				}

				durationMs := float64(span.EndTimestamp()-span.StartTimestamp()) / 1e6
				if err := c.mlflow.LogMetric(ctx, runID, "duration_ms", durationMs); err != nil {
					c.logger.Error("failed to log metric", "error", err)
				}

				mcpMethod, _ := span.Attributes().Get("mcp_method")
				if mcpMethod.Str() != "" {
					c.mlflow.SetTag(ctx, runID, "mcp.method", mcpMethod.Str())
				}
			}
		}
	}
	return nil
}
```

### Quality Gate

- MLflow UI accessible on port 5000; experiments list loads.
- PostgreSQL backend stores runs (not file-based).
- MinIO bucket `mlflow-artifacts` exists and is writable.
- Converter receives spans and creates corresponding MLflow runs.

### Testing Command

```bash
# Deploy MLflow stack
helm upgrade --install mcp-gateway deploy/helm/ \
  --set observability.mlflow.enabled=true \
  -n mcp-system

# Verify MLflow health
kubectl port-forward svc/mcp-gateway-mlflow 5000:5000 -n mcp-system &
curl -s http://localhost:5000/health

# Run converter unit tests
go test ./cmd/mlflow-converter/... -v -count=1

# Integration: verify span -> MLflow run
make test-integration TEST_ARGS="-run TestMLflowConverter"
```

### Pitfalls

- **PostgreSQL connection pool exhaustion:** MLflow uses SQLAlchemy under the hood. Set `--workers=2` to limit parallel connections. The default unbounded pool can exhaust the 100-connection PostgreSQL limit under load.
- **MinIO bucket creation race:** The MinIO init container must create the `mlflow-artifacts` bucket before MLflow starts. Use an `initContainer` with `mc mb` to guarantee ordering.
- **Converter restart loses mapping:** The `experimentMapping` is in-memory. On restart, the converter calls `mlflow.getExperimentByName()` to rebuild. Add a startup reconciliation loop.

### Progress Marker

- [ ] MLflow Deployment running with PostgreSQL backend
- [ ] MinIO deployed and bucket created
- [ ] MLflow UI accessible and functional
- [ ] Converter receives OTLP spans
- [ ] Spans correctly mapped to MLflow experiments and runs

---

## Step 5: Prometheus + Grafana

Deploy ServiceMonitors for all components, PrometheusRule alerts for critical conditions, and three Grafana dashboards for platform-wide visibility.

### Files

```
deploy/helm/templates/servicemonitor-operator.yaml
deploy/helm/templates/servicemonitor-envoy.yaml
deploy/helm/templates/servicemonitor-otel-collector.yaml
deploy/helm/templates/prometheusrule.yaml
deploy/helm/templates/grafana-dashboard-configmaps.yaml
deploy/grafana/dashboards/platform-overview.json
deploy/grafana/dashboards/per-server.json
deploy/grafana/dashboards/per-agent.json
```

### Key Code

**deploy/helm/templates/servicemonitor-operator.yaml**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ .Release.Name }}-operator
  namespace: {{ .Release.Namespace }}
  labels:
    app.kubernetes.io/part-of: mcp-gateway
    prometheus: mcp-gateway
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: operator
  endpoints:
    - port: metrics
      interval: 15s
      path: /metrics
      metricRelabelings:
        - sourceLabels: [__name__]
          regex: "mcpgateway_.*"
          action: keep
```

**deploy/helm/templates/prometheusrule.yaml**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: {{ .Release.Name }}-alerts
  namespace: {{ .Release.Namespace }}
  labels:
    prometheus: mcp-gateway
spec:
  groups:
    - name: mcp-gateway.rules
      rules:
        - alert: MCPServerDown
          expr: |
            mcpgateway_servers_active == 0
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "No active MCP servers"
            description: "The MCP Gateway has no active server instances for {{ "{{ $labels.namespace }}" }}."
            runbook_url: "https://docs.mcp-gateway.io/runbooks/server-down"

        - alert: MCPHighErrorRate
          expr: |
            (
              sum(rate(mcpgateway_reconcile_total{status="error"}[5m])) by (controller)
              /
              sum(rate(mcpgateway_reconcile_total[5m])) by (controller)
            ) > 0.1
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "High reconciliation error rate"
            description: "Controller {{ "{{ $labels.controller }}" }} has >10% error rate over 10 minutes."

        - alert: MCPQuotaExhausted
          expr: |
            mcpgateway_agent_quota_remaining_ratio < 0.05
          for: 1m
          labels:
            severity: warning
          annotations:
            summary: "Agent quota nearly exhausted"
            description: "Agent {{ "{{ $labels.agent }}" }} has <5% quota remaining for server {{ "{{ $labels.server }}" }}."

        - alert: EnvoyConfigPushFailure
          expr: |
            increase(mcpgateway_envoy_config_push_errors_total[5m]) > 0
          for: 2m
          labels:
            severity: critical
          annotations:
            summary: "Envoy config push failing"
            description: "xDS config pushes to Envoy are failing, new server configs will not take effect."
```

**deploy/grafana/dashboards/platform-overview.json** (condensed structure)

```json
{
  "dashboard": {
    "title": "MCP Gateway - Platform Overview",
    "uid": "mcp-platform-overview",
    "tags": ["mcp-gateway"],
    "timezone": "browser",
    "refresh": "30s",
    "panels": [
      {
        "title": "Active MCP Servers",
        "type": "stat",
        "gridPos": { "h": 4, "w": 6, "x": 0, "y": 0 },
        "targets": [{ "expr": "mcpgateway_servers_active", "legendFormat": "{{ namespace }}" }]
      },
      {
        "title": "Reconciliation Rate",
        "type": "timeseries",
        "gridPos": { "h": 8, "w": 12, "x": 0, "y": 4 },
        "targets": [
          { "expr": "sum(rate(mcpgateway_reconcile_total[5m])) by (controller, status)", "legendFormat": "{{ controller }}/{{ status }}" }
        ]
      },
      {
        "title": "Reconcile Duration (p99)",
        "type": "timeseries",
        "gridPos": { "h": 8, "w": 12, "x": 12, "y": 4 },
        "targets": [
          { "expr": "histogram_quantile(0.99, sum(rate(mcpgateway_reconcile_duration_bucket[5m])) by (le, controller))", "legendFormat": "{{ controller }}" }
        ]
      },
      {
        "title": "Envoy Request Rate",
        "type": "timeseries",
        "gridPos": { "h": 8, "w": 12, "x": 0, "y": 12 },
        "targets": [
          { "expr": "sum(rate(envoy_http_downstream_rq_total[5m])) by (envoy_http_conn_manager_prefix)", "legendFormat": "{{ envoy_http_conn_manager_prefix }}" }
        ]
      },
      {
        "title": "Config Push Errors",
        "type": "stat",
        "gridPos": { "h": 4, "w": 6, "x": 6, "y": 0 },
        "targets": [{ "expr": "increase(mcpgateway_envoy_config_push_errors_total[1h])", "legendFormat": "errors" }],
        "fieldConfig": { "defaults": { "thresholds": { "steps": [{ "value": 0, "color": "green" }, { "value": 1, "color": "red" }] } } }
      }
    ]
  }
}
```

**deploy/grafana/dashboards/per-server.json** (panels)

```json
{
  "dashboard": {
    "title": "MCP Gateway - Per Server",
    "uid": "mcp-per-server",
    "tags": ["mcp-gateway"],
    "templating": {
      "list": [
        {
          "name": "server",
          "type": "query",
          "query": "label_values(mcpgateway_reconcile_total{controller='mcpserver'}, server_name)",
          "refresh": 2
        }
      ]
    },
    "panels": [
      {
        "title": "Request Rate",
        "type": "timeseries",
        "targets": [{ "expr": "sum(rate(envoy_http_downstream_rq_total{envoy_http_conn_manager_prefix=~\"mcp.*$server.*\"}[5m]))" }]
      },
      {
        "title": "Error Rate",
        "type": "timeseries",
        "targets": [{ "expr": "sum(rate(envoy_http_downstream_rq_xx{envoy_http_conn_manager_prefix=~\"mcp.*$server.*\",envoy_response_code_class=\"5\"}[5m]))" }]
      },
      {
        "title": "Latency Distribution",
        "type": "heatmap",
        "targets": [{ "expr": "sum(rate(envoy_http_downstream_rq_time_bucket{envoy_http_conn_manager_prefix=~\"mcp.*$server.*\"}[5m])) by (le)" }]
      },
      {
        "title": "Active Connections",
        "type": "stat",
        "targets": [{ "expr": "envoy_http_downstream_cx_active{envoy_http_conn_manager_prefix=~\"mcp.*$server.*\"}" }]
      }
    ]
  }
}
```

**deploy/grafana/dashboards/per-agent.json** (panels)

```json
{
  "dashboard": {
    "title": "MCP Gateway - Per Agent",
    "uid": "mcp-per-agent",
    "tags": ["mcp-gateway"],
    "templating": {
      "list": [
        {
          "name": "agent",
          "type": "query",
          "query": "label_values(mcpgateway_agent_quota_remaining_ratio, agent)",
          "refresh": 2
        }
      ]
    },
    "panels": [
      {
        "title": "Quota Remaining",
        "type": "gauge",
        "targets": [{ "expr": "mcpgateway_agent_quota_remaining_ratio{agent=\"$agent\"}" }],
        "fieldConfig": { "defaults": { "min": 0, "max": 1, "thresholds": { "steps": [{ "value": 0, "color": "red" }, { "value": 0.2, "color": "yellow" }, { "value": 0.5, "color": "green" }] } } }
      },
      {
        "title": "Request Rate by Tool",
        "type": "timeseries",
        "targets": [{ "expr": "sum(rate(mcpgateway_agent_tool_calls_total{agent=\"$agent\"}[5m])) by (tool, server)" }]
      },
      {
        "title": "Rate Limit Rejections",
        "type": "timeseries",
        "targets": [{ "expr": "sum(rate(mcpgateway_agent_rate_limited_total{agent=\"$agent\"}[5m])) by (server)" }]
      },
      {
        "title": "Allowed Servers",
        "type": "table",
        "targets": [{ "expr": "mcpgateway_agent_server_access{agent=\"$agent\"}", "format": "table", "instant": true }]
      }
    ]
  }
}
```

### Quality Gate

- All three ServiceMonitors discovered by Prometheus (check Prometheus targets page).
- All four alerts in "inactive" state in Prometheus alerts page.
- All three Grafana dashboards load without "No Data" panels when at least one MCPServer is running.

### Testing Command

```bash
# Verify ServiceMonitors
kubectl get servicemonitors -n mcp-system -l app.kubernetes.io/part-of=mcp-gateway

# Verify PrometheusRules
kubectl get prometheusrules -n mcp-system

# Check Prometheus targets (assumes prometheus-operator installed)
kubectl port-forward svc/prometheus-operated 9090:9090 -n monitoring &
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job | contains("mcp"))'

# Validate dashboard JSON
for f in deploy/grafana/dashboards/*.json; do
  python3 -c "import json; json.load(open('$f')); print(f'OK: $f')"
done
```

### Pitfalls

- **ServiceMonitor label selector mismatch:** The Prometheus Operator selects ServiceMonitors by label. The `prometheus: mcp-gateway` label must match the Prometheus CR's `serviceMonitorSelector`. Check the existing Prometheus CR in the cluster first.
- **Dashboard provisioning:** Grafana sidecar looks for ConfigMaps with label `grafana_dashboard: "1"`. The Helm template must add this label and mount the JSON into the right path.
- **Alert fatigue:** The `MCPServerDown` alert fires when `mcpgateway_servers_active == 0`, which is true on fresh installs. Add a recording rule that excludes namespaces with no MCPServer CRDs.

### Progress Marker

- [ ] ServiceMonitors created and discovered by Prometheus
- [ ] PrometheusRule deployed with all four alerts
- [ ] Platform Overview dashboard loads with live data
- [ ] Per-Server dashboard variable populates server list
- [ ] Per-Agent dashboard shows quota gauges

---

## Step 6: E2E Observability Test

Validate the full observability pipeline end-to-end: create an MCPServer, send traffic through Envoy, verify traces in OTel Collector, metrics in Prometheus, and runs in MLflow.

### Files

```
test/e2e/observability_test.go
test/e2e/testdata/observability-mcpserver.yaml
```

### Key Code

**test/e2e/observability_test.go**

```go
//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObservabilityPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Step 1: Deploy test MCPServer
	kubectl(t, "apply", "-f", "testdata/observability-mcpserver.yaml")
	t.Cleanup(func() {
		kubectl(t, "delete", "-f", "testdata/observability-mcpserver.yaml", "--ignore-not-found")
	})
	waitForMCPServer(t, ctx, "obs-test-server", "mcp-system")

	// Step 2: Send traffic through Envoy
	envoyAddr := portForward(t, "svc/mcp-gateway-envoy", "mcp-system", 10000)
	for i := 0; i < 10; i++ {
		resp, err := http.Post(
			fmt.Sprintf("http://%s/mcp/obs-test-server/message", envoyAddr),
			"application/json",
			strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`),
		)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// Step 3: Verify Prometheus metrics
	t.Run("prometheus_metrics", func(t *testing.T) {
		promAddr := portForward(t, "svc/prometheus-operated", "monitoring", 9090)
		assertPromMetricExists(t, ctx, promAddr, `mcpgateway_reconcile_total{controller="mcpserver"}`)
		assertPromMetricExists(t, ctx, promAddr, `mcpgateway_servers_active`)
		assertPromMetricExists(t, ctx, promAddr, `envoy_http_downstream_rq_total`)
	})

	// Step 4: Verify OTel Collector processed spans
	t.Run("otel_collector_spans", func(t *testing.T) {
		collectorMetrics := portForward(t, "svc/mcp-gateway-otel-collector", "mcp-system", 8889)
		assertPromMetricExists(t, ctx, collectorMetrics, `otelcol_receiver_accepted_spans`)
	})

	// Step 5: Verify MLflow experiment
	t.Run("mlflow_experiment", func(t *testing.T) {
		mlflowAddr := portForward(t, "svc/mcp-gateway-mlflow", "mcp-system", 5000)
		assertEventually(t, ctx, 30*time.Second, func() bool {
			resp, err := http.Get(fmt.Sprintf("http://%s/api/2.0/mlflow/experiments/search?filter=name%%20LIKE%%20%%27%%25obs-test%%25%%27", mlflowAddr))
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			var result struct {
				Experiments []struct{ Name string } `json:"experiments"`
			}
			json.NewDecoder(resp.Body).Decode(&result)
			return len(result.Experiments) > 0
		}, "MLflow experiment for obs-test-server should exist")
	})
}

// Helper: assert a PromQL query returns results
func assertPromMetricExists(t *testing.T, ctx context.Context, promAddr, query string) {
	t.Helper()
	assertEventually(t, ctx, 60*time.Second, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/query?query=%s", promAddr, url.QueryEscape(query)))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		var result struct {
			Data struct {
				Result []json.RawMessage `json:"result"`
			} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		return len(result.Data.Result) > 0
	}, fmt.Sprintf("metric %s should exist in Prometheus", query))
}
```

### Quality Gate

- Test passes in CI with a Kind cluster that has Prometheus Operator and Grafana pre-installed.
- All five verification steps pass: MCPServer ready, traffic succeeds, Prometheus metrics exist, OTel Collector processed spans, MLflow experiment created.

### Testing Command

```bash
# Run E2E observability test (requires Kind cluster with full stack)
make test-e2e TEST_ARGS="-run TestObservabilityPipeline -timeout 10m"

# Or run directly
go test -tags=e2e ./test/e2e/ -run TestObservabilityPipeline -timeout 10m -v
```

### Pitfalls

- **Timing sensitivity:** Metrics and traces take time to propagate. The `assertEventually` helper with generous timeouts (60s for Prometheus, 30s for MLflow) is essential. Do not use immediate assertions.
- **Port-forward flakiness:** Port-forwards can drop during long tests. The `portForward` helper should retry on connection errors and use random local ports to avoid conflicts.
- **Cluster prerequisites:** This test requires Prometheus Operator and Grafana already installed. Document the Kind cluster setup script in `hack/setup-kind-observability.sh`.

### Progress Marker

- [ ] E2E test compiles and runs against Kind cluster
- [ ] All five verification steps pass
- [ ] Test runs in CI pipeline
- [ ] Test cleanup removes all resources
- [ ] Test is skipped gracefully when prerequisites are missing
