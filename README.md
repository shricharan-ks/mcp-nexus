# MCP Gateway

**Kubernetes-native control plane for MCP servers**

[![Go Report Card](https://goreportcard.com/badge/github.com/mcp-gateway/mcp-gateway)](https://goreportcard.com/report/github.com/mcp-gateway/mcp-gateway)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![CI](https://github.com/mcp-gateway/mcp-gateway/actions/workflows/ci.yaml/badge.svg)](https://github.com/mcp-gateway/mcp-gateway/actions/workflows/ci.yaml)

MCP Gateway is a Kubernetes operator and gateway stack that manages the full
lifecycle of [Model Context Protocol](https://modelcontextprotocol.io/) servers.
It provides declarative CRDs for deploying MCP servers, registering AI agents,
enforcing per-tool access policies, and operating a curated marketplace --
all backed by Envoy AI Gateway for traffic management, Cerbos for authorization,
and OpenTelemetry for end-to-end observability.

## Features

- **4 Custom Resource Definitions** -- MCPServer, MCPAgent, MCPPolicy, and MCPMarketplaceEntry -- for declarative lifecycle management
- **Envoy AI Gateway integration** -- JWT authentication, request routing, and rate limiting at the edge
- **Per-tool RBAC via Cerbos** -- fine-grained ALLOW/DENY policies scoped to individual tools, roles, and agents
- **Rate limiting and quotas** -- global and per-tool rate limits, monthly call quotas, concurrent connection caps
- **OpenTelemetry observability** -- distributed traces, Prometheus metrics, and pre-built Grafana dashboards for every tool call
- **Marketplace with 1-click deploy** -- browse a curated catalog of MCP servers with security scanning, SBOM tracking, and install templates
- **Stdio bridge** -- first-class support for stdio-based MCP servers alongside streamable-http
- **Scale to zero** -- idle MCP servers scale down automatically and wake on demand

## Architecture

```
Agent -> Envoy AI GW (JWT+RBAC+Rate Limit) -> MCP Server Pods
              |                                      |
         Cerbos (policy)                    K8s Operator (lifecycle)
              |                                      |
         OTel Collector -> Prometheus/Grafana/MLflow
```

## Quick Start

### Prerequisites

| Tool      | Version  | Install                              |
|-----------|----------|--------------------------------------|
| Go        | 1.26+    | https://go.dev/dl/                   |
| Docker    | 24+      | https://docs.docker.com/get-docker/  |
| Kind      | 0.20+    | `go install sigs.k8s.io/kind@latest` |
| Helm      | 3.12+    | https://helm.sh/docs/intro/install/  |
| kubectl   | 1.28+    | https://kubernetes.io/docs/tasks/tools/ |

### Build and deploy

```bash
# Clone the repository
git clone https://github.com/mcp-gateway/mcp-gateway.git
cd mcp-gateway

# Build binaries
make build

# Create a local Kind cluster
make kind-up

# Deploy the operator into the cluster
make dev-deploy

# Deploy an example MCP server
kubectl apply -f examples/echo-server.yaml

# Verify it is running
kubectl get mcps -n mcp-servers
```

## CRD Reference

| CRD | Short Name | Description | Key Fields |
|-----|------------|-------------|------------|
| `MCPServer` | `mcps` | Manages the lifecycle of a single MCP server (Deployment, Service, health checks, scaling) | `spec.source.image`, `spec.protocol.transport`, `spec.scaling`, `spec.secrets[]` |
| `MCPAgent` | `mcpa` | Registers an AI agent identity with OIDC credentials, server access bindings, rate limits, and quotas | `spec.identity.oidcClientId`, `spec.serverAccess[]`, `spec.rateLimits`, `spec.quota` |
| `MCPPolicy` | `mcpp` | Declares ALLOW/DENY authorization rules scoped to tools, roles, and agent references; synced to Cerbos | `spec.rules[].effect`, `spec.rules[].actions`, `spec.rules[].resources.tools` |
| `MCPMarketplaceEntry` | `mcpme` | Catalog entry for a shareable MCP server with version, security scan status, and an install template | `spec.source.image`, `spec.category`, `spec.security`, `spec.installTemplate` |

All CRDs belong to API group `mcp.mcp-gateway.io/v1alpha1`.

## Configuration

Key environment variables for the operator:

| Variable | Description | Default |
|----------|-------------|---------|
| `GATEWAY_NAME` | Name of the Envoy AI Gateway instance | `mcp-gateway` |
| `KEYCLOAK_URL` | Keycloak server URL for OIDC | `http://keycloak.mcp-system.svc:8080` |
| `CERBOS_URL` | Cerbos PDP endpoint | `http://cerbos.mcp-system.svc:3592` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP gRPC endpoint for traces and metrics | `otel-collector.mcp-system.svc:4317` |

## Project Structure

```
api/                    CRD type definitions (Group: mcp.mcp-gateway.io, Version: v1alpha1)
cmd/operator/           Operator (controller-manager) entrypoint
cmd/apiserver/          Aggregated API server entrypoint
internal/controller/    Reconciler implementations
internal/discovery/     MCP server capability discovery
internal/envoy/         Envoy AI Gateway integration
internal/keycloak/      Keycloak OIDC client management
internal/cerbos/        Cerbos policy sync
internal/audit/         Audit logging
internal/marketplace/   Marketplace catalog and install logic
internal/observability/ OpenTelemetry metrics and tracing setup
config/                 Kustomize bases for CRDs, RBAC, manager
deploy/helm/            Helm chart for production installs
scripts/                Developer helper scripts (kind-setup, codegen)
examples/               Sample CRD manifests
docs/                   Architecture docs, guides
test/e2e/               End-to-end tests (run against Kind)
ui/                     Web dashboard (Next.js 15 / TypeScript)
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, and
the pull request process. All commits must be signed off (`git commit -s`) per
the [Developer Certificate of Origin](DCO).

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details.
