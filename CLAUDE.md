# MCP Gateway

Kubernetes-native control plane for managing Model Context Protocol (MCP) servers.
The operator watches `MCPServer` custom resources and reconciles Deployments,
Services, and routing configuration so that MCP servers are discoverable and
lifecycle-managed inside any Kubernetes cluster.

## Repository layout

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

## Build commands

| Command                | Purpose                                           |
|------------------------|---------------------------------------------------|
| `make build`           | Compile all binaries                              |
| `make test`            | Run every test (unit + envtest integration)        |
| `make test-unit`       | Run unit tests only                               |
| `make lint`            | golangci-lint                                     |
| `make generate`        | Run controller-gen + deepcopy + client-gen         |
| `make manifests`       | Regenerate CRD and RBAC manifests                  |
| `make docker-build`    | Build container images                            |
| `make kind-up`         | Create a local Kind cluster                       |
| `make kind-down`       | Tear down the Kind cluster                        |
| `make dev-deploy`      | Deploy operator into the Kind cluster             |
| `make dev-teardown`    | Remove the dev deployment from the Kind cluster   |

## Code conventions

- **Go 1.26+** — use standard-library `slices`, `maps`, `slog` where appropriate.
- Follow **controller-runtime** patterns: `Reconcile` returns `(ctrl.Result, error)`,
  use `client.Reader` / `client.Writer` interfaces, never raw REST clients.
- Wrap errors with `fmt.Errorf("context: %w", err)` — always use `%w`.
- Structured logging via **logr** (`log := ctrl.LoggerFrom(ctx)`).
- Table-driven tests with **testify** (`assert` / `require`).
- All CRD types carry **kubebuilder markers** (`+kubebuilder:object:root`,
  `+kubebuilder:subresource:status`, `+kubebuilder:printcolumn`, validation markers).

## Kubernetes conventions

- API group: `mcp.mcp-gateway.io`
- Version: `v1alpha1`
- Domain: `mcp-gateway.io`
- Owner references on every child resource for garbage collection.
- Finalizers for cleanup of external resources (routes, proxy config).
- Status uses `metav1.Condition` (types: `Ready`, `Available`, `Progressing`, `Degraded`).

## Git conventions

- All commits **signed off** — use `git commit -s` (DCO).
- Branch naming: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `chore/<topic>`.
- Commit message: imperative mood, 72-character first line.
- One logical change per commit.

## Testing

- **Unit tests** live next to the source file (`foo_test.go` beside `foo.go`).
- **Integration tests** use controller-runtime **envtest** (real etcd + apiserver, no kubelet).
- **E2E tests** live in `test/e2e/` and run against a Kind cluster.
- Target **80 % coverage** for controllers.

## Docker

- Multi-stage builds: builder stage then **distroless** runtime image.
- Run as nonroot, UID **65532**.

## Key dependencies

- `sigs.k8s.io/controller-runtime`
- `k8s.io/apimachinery`
- `k8s.io/client-go`
- `github.com/stretchr/testify`

## Pitfalls

- Never `List` without a namespace or label selector — unbounded lists can OOM the controller.
- Always set owner references on child objects so they are garbage-collected.
- Always check `apierrors.IsNotFound(err)` before treating an error as fatal.
- JSON struct tags must be **camelCase** (e.g., `json:"healthCheck"`).
- Run `make generate manifests` before running tests after changing API types.
