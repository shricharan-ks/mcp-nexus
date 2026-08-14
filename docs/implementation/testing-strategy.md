# Cross-Cutting Testing Strategy

This document defines the testing approach for the entire MCP Gateway project: what is tested at each layer, how tests are structured, where they run, and what coverage is expected.

---

## Testing Layers

| Layer | Scope | Tool | Runs On | Timeout | Build Tag |
|-------|-------|------|---------|---------|-----------|
| Unit | Single function/method | `go test`, Vitest | Every PR | 5m | (none) |
| Integration / envtest | Controller + CRDs + fake API server | `go test` + envtest | Every PR | 10m | `integration` |
| E2E / Kind | Full stack in a real cluster | `go test` + Kind | Push to main | 20m | `e2e` |
| UI Unit | React components + hooks | Vitest + Testing Library | Every PR | 3m | (none) |
| UI E2E | Browser flows | Playwright | Push to main | 15m | (none) |
| Contract | Protobuf compatibility | `buf breaking` | Every PR | 1m | (none) |
| Load | Performance under stress | k6 | Nightly | 30m | (none) |
| Security | Vulnerabilities + misconfig | Trivy, govulncheck, OSSF | Weekly + PR | 10m | (none) |

---

## Unit Testing Conventions

All Go unit tests follow these conventions.

### Files

```
internal/controller/mcpserver_controller_test.go
internal/xds/snapshot_test.go
internal/observability/metrics_test.go
internal/marketplace/validation_test.go
```

### Key Code

**Table-driven test pattern**

```go
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEnvoyCluster(t *testing.T) {
	tests := []struct {
		name      string
		server    *v1alpha1.MCPServer
		wantName  string
		wantHost  string
		wantPort  uint32
		wantErr   bool
	}{
		{
			name: "SSE transport with URL",
			server: &v1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{Name: "test-server", Namespace: "default"},
				Spec: v1alpha1.MCPServerSpec{
					Transport: "sse",
					URL:       "http://backend:8080/sse",
				},
			},
			wantName: "mcp_default_test-server",
			wantHost: "backend",
			wantPort: 8080,
			wantErr:  false,
		},
		{
			name: "missing URL for SSE",
			server: &v1alpha1.MCPServer{
				Spec: v1alpha1.MCPServerSpec{
					Transport: "sse",
				},
			},
			wantErr: true,
		},
		{
			name: "streamable-http transport",
			server: &v1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{Name: "stream", Namespace: "prod"},
				Spec: v1alpha1.MCPServerSpec{
					Transport: "streamable-http",
					URL:       "http://stream-backend:9090/mcp",
				},
			},
			wantName: "mcp_prod_stream",
			wantHost: "stream-backend",
			wantPort: 9090,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster, err := buildEnvoyCluster(tt.server)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, cluster.Name)
			assert.Equal(t, tt.wantHost, cluster.LoadAssignment.Endpoints[0].LbEndpoints[0].GetEndpoint().Address.GetSocketAddress().Address)
			assert.Equal(t, tt.wantPort, cluster.LoadAssignment.Endpoints[0].LbEndpoints[0].GetEndpoint().Address.GetSocketAddress().GetPortValue())
		})
	}
}
```

**Rules**

- All tests are table-driven with named sub-tests.
- Use `testify/assert` for non-fatal checks and `testify/require` for fatal preconditions.
- No external dependencies: no network calls, no database, no file I/O beyond `testdata/`.
- Use `t.Parallel()` on tests that do not share mutable state.
- Mock interfaces, not implementations. Define interfaces at the consumer site.

### Quality Gate

- `go test ./... -short` passes in under 5 minutes.
- No test uses `time.Sleep` (use `clock` interfaces or channels).
- No test reads from environment variables (use constructor injection).

### Testing Command

```bash
# Run all unit tests
go test ./... -short -count=1 -race

# Run with verbose output for a specific package
go test ./internal/controller/... -v -count=1

# Run a specific test
go test ./internal/xds/... -run TestBuildEnvoyCluster -v
```

### Pitfalls

- **Test pollution from shared state:** Package-level variables can leak between tests. Use `t.Cleanup()` to reset any package state, or restructure to pass state via function parameters.
- **Race detector overhead:** The `-race` flag increases test time by 2-10x. Run it on every PR but allow longer timeouts.
- **Testify version mismatch:** Ensure `testify` v1.9+ is used across all packages. Older versions have `assert.ErrorIs` bugs.

### Progress Marker

- [ ] All packages have `_test.go` files
- [ ] Table-driven pattern used consistently
- [ ] No external dependencies in unit tests
- [ ] `-race` passes on all tests
- [ ] Test names follow `TestFunction_Scenario` convention

---

## envtest Setup

Integration tests use controller-runtime's envtest to run a real API server with CRDs, without needing a full cluster.

### Files

```
internal/controller/suite_test.go
internal/controller/mcpserver_controller_integration_test.go
internal/controller/testdata/                  (test CRDs and fixtures)
```

### Key Code

**internal/controller/suite_test.go**

```go
//go:build integration

package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Integration Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = mcpv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Start the controller manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())

	// Mock external services
	mockXDSServer := newMockXDSServer()
	mockRedis := newMockRedis()

	err = (&MCPServerReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		XDS:       mockXDSServer,
		Cache:     mockRedis,
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
```

**Mock external services pattern**

```go
// internal/controller/mock_test.go

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
)

// mockXDSServer simulates the xDS control plane for integration tests.
type mockXDSServer struct {
	mu        sync.Mutex
	snapshots map[string]interface{}
}

func newMockXDSServer() *mockXDSServer {
	return &mockXDSServer{
		snapshots: make(map[string]interface{}),
	}
}

func (m *mockXDSServer) SetSnapshot(ctx context.Context, nodeID string, snapshot interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots[nodeID] = snapshot
	return nil
}

func (m *mockXDSServer) GetSnapshot(nodeID string) (interface{}, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.snapshots[nodeID]
	return s, ok
}

// mockRedis simulates Redis for integration tests.
type mockRedis struct {
	mu   sync.Mutex
	data map[string]string
}

func newMockRedis() *mockRedis {
	return &mockRedis{data: make(map[string]string)}
}

func (m *mockRedis) Get(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[key], nil
}

func (m *mockRedis) Set(ctx context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

// newMockHTTPServer creates an httptest server that simulates an MCP backend.
func newMockHTTPServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","result":{"tools":[]},"id":1}`))
	})
	return httptest.NewServer(mux)
}
```

### Quality Gate

- `suite_test.go` starts envtest environment without errors.
- CRD directory path resolves correctly relative to the test file.
- Controller reconciliation works end-to-end against the envtest API server.
- Mock external services respond correctly.
- Tests complete in under 10 minutes.

### Testing Command

```bash
# Run integration tests (requires envtest binaries)
make test-integration

# Or directly with build tag
go test -tags=integration ./internal/controller/... -v -count=1 -timeout 10m

# Install envtest binaries if not present
make envtest
$(LOCALBIN)/setup-envtest use --bin-dir $(LOCALBIN)
```

### Pitfalls

- **envtest binary version mismatch:** The envtest binaries (etcd, kube-apiserver) must match the Kubernetes version used in production. Pin the version in the Makefile: `setup-envtest use 1.29.x`.
- **CRD path resolution:** The `CRDDirectoryPaths` is relative to the test file, not the project root. Use `filepath.Join("..", "..", "config", "crd", "bases")` from `internal/controller/`.
- **Test isolation:** Each test should create resources in a unique namespace to avoid interference. Use `t.Name()` or a random suffix for namespace names.
- **Manager shutdown:** The manager goroutine must be stopped via context cancellation in `AfterSuite`. If not stopped, subsequent test suites may fail with port conflicts.

### Progress Marker

- [ ] `suite_test.go` bootstraps envtest environment
- [ ] CRDs load from generated manifests
- [ ] Controller reconciles MCPServer CRs
- [ ] Mock XDS and Redis used for external deps
- [ ] Tests run with `-tags=integration`

---

## E2E Testing

Full end-to-end tests run against a real Kind cluster with all components deployed.

### Files

```
test/e2e/suite_test.go
test/e2e/mcpserver_e2e_test.go
test/e2e/policy_e2e_test.go
test/e2e/testdata/
hack/setup-kind.sh
hack/teardown-kind.sh
```

### Key Code

**test/e2e/suite_test.go**

```go
//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// kubectl runs a kubectl command and returns stdout.
func kubectl(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("kubectl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kubectl %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

// kubectlOutput runs kubectl without failing the test on error.
func kubectlOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("kubectl", args...)
	out, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(out))
}

// waitForMCPServer waits until the named MCPServer has Ready=True condition.
func waitForMCPServer(t *testing.T, ctx context.Context, name, namespace string) {
	t.Helper()
	assertEventually(t, ctx, 120*time.Second, func() bool {
		output := kubectlOutput(t, "get", "mcpserver", name, "-n", namespace,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		return output == "True"
	}, fmt.Sprintf("MCPServer %s/%s should become Ready", namespace, name))
}

// assertEventually retries a check function until it returns true or timeout.
func assertEventually(t *testing.T, ctx context.Context, timeout time.Duration, check func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if check() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting: %s", msg)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled while waiting: %s", msg)
		case <-time.After(2 * time.Second):
		}
	}
}

// portForward creates a kubectl port-forward and returns the local address.
func portForward(t *testing.T, target, namespace string, remotePort int) string {
	t.Helper()
	localPort := 30000 + (os.Getpid() % 10000) + remotePort
	cmd := exec.Command("kubectl", "port-forward", target, fmt.Sprintf("%d:%d", localPort, remotePort), "-n", namespace)
	if err := cmd.Start(); err != nil {
		t.Fatalf("port-forward failed: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
	})
	// Wait for port-forward to be ready
	time.Sleep(2 * time.Second)
	return fmt.Sprintf("localhost:%d", localPort)
}
```

**Cleanup pattern**

```go
func TestMCPServerLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create
	kubectl(t, "apply", "-f", "testdata/e2e-server.yaml")
	t.Cleanup(func() {
		kubectl(t, "delete", "-f", "testdata/e2e-server.yaml", "--ignore-not-found")
	})

	// Wait for Ready
	waitForMCPServer(t, ctx, "e2e-test-server", "mcp-system")

	// Verify child resources
	output := kubectl(t, "get", "deploy", "e2e-test-server", "-n", "mcp-system", "-o", "name")
	assert.Contains(t, output, "deployment.apps/e2e-test-server")

	// Update
	kubectl(t, "patch", "mcpserver", "e2e-test-server", "-n", "mcp-system",
		"--type=merge", "-p", `{"spec":{"replicas":2}}`)
	assertEventually(t, ctx, 60*time.Second, func() bool {
		output := kubectlOutput(t, "get", "deploy", "e2e-test-server", "-n", "mcp-system",
			"-o", "jsonpath={.spec.replicas}")
		return output == "2"
	}, "deployment should scale to 2 replicas")

	// Delete (handled by Cleanup)
}
```

### Quality Gate

- All E2E tests pass against a fresh Kind cluster.
- Tests create their own resources and clean up after themselves.
- Tests do not depend on ordering (each is independently runnable).
- Total E2E suite completes in under 20 minutes.

### Testing Command

```bash
# Setup Kind cluster with full stack
chmod +x hack/setup-kind.sh
./hack/setup-kind.sh

# Run E2E tests
go test -tags=e2e ./test/e2e/... -v -timeout 20m -count=1

# Run a specific E2E test
go test -tags=e2e ./test/e2e/ -run TestMCPServerLifecycle -v -timeout 10m

# Teardown Kind cluster
chmod +x hack/teardown-kind.sh
./hack/teardown-kind.sh
```

### Pitfalls

- **Kind cluster startup time:** Kind with all components (Prometheus, Grafana, KEDA, Velero) can take 5+ minutes to start. Cache container images with `kind load docker-image` to speed up.
- **Port-forward port conflicts:** Multiple parallel test runs will conflict on port-forward ports. Use a hash of `os.Getpid()` plus the remote port to generate semi-unique local ports.
- **Cleanup ordering:** `t.Cleanup` runs in LIFO order. Register namespace cleanup last so it runs first, ensuring all resources in the namespace are deleted.

### Progress Marker

- [ ] Kind setup script creates cluster with all dependencies
- [ ] E2E tests compile with `e2e` build tag
- [ ] All tests clean up after themselves
- [ ] Suite completes under 20 minutes
- [ ] Tests are independently runnable

---

## UI Testing

### UI Unit Tests (Vitest)

### Files

```
ui/vitest.config.ts
ui/components/servers/server-table.test.tsx
ui/components/agents/permission-matrix.test.tsx
ui/hooks/use-servers.test.ts
```

### Key Code

**ui/vitest.config.ts**

```typescript
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./test/setup.ts"],
    include: ["**/*.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      include: ["components/**", "hooks/**", "lib/**"],
      thresholds: {
        statements: 70,
        branches: 60,
        functions: 70,
        lines: 70,
      },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "."),
    },
  },
});
```

**ui/components/agents/permission-matrix.test.tsx**

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { PermissionMatrix } from "./permission-matrix";
import { describe, it, expect, vi } from "vitest";

describe("PermissionMatrix", () => {
  const mockServers = [
    {
      name: "server-a",
      namespace: "default",
      tools: [{ name: "tool-1" }, { name: "tool-2" }],
    },
    {
      name: "server-b",
      namespace: "default",
      tools: [{ name: "tool-3" }],
    },
  ];

  it("renders a checkbox for each server/tool combination", () => {
    render(
      <PermissionMatrix
        servers={mockServers}
        permissions={[]}
        onChange={vi.fn()}
      />
    );

    const checkboxes = screen.getAllByRole("checkbox");
    // 2 tools for server-a + 1 tool for server-b = 3 checkboxes
    expect(checkboxes).toHaveLength(3);
  });

  it("calls onChange when a checkbox is toggled", () => {
    const onChange = vi.fn();
    render(
      <PermissionMatrix
        servers={mockServers}
        permissions={[]}
        onChange={onChange}
      />
    );

    const firstCheckbox = screen.getAllByRole("checkbox")[0];
    fireEvent.click(firstCheckbox);

    expect(onChange).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({
          serverName: "server-a",
          allowedTools: ["tool-1"],
        }),
      ])
    );
  });

  it("does not allow toggling when readOnly", () => {
    const onChange = vi.fn();
    render(
      <PermissionMatrix
        servers={mockServers}
        permissions={[]}
        onChange={onChange}
        readOnly
      />
    );

    const firstCheckbox = screen.getAllByRole("checkbox")[0];
    expect(firstCheckbox).toBeDisabled();
  });
});
```

### UI E2E Tests (Playwright)

Auth state persistence pattern is covered in Phase 6, Step 6. Key points:

- **Auth state file** (`.auth/user.json`) is created once in a setup project and reused across all test files.
- **Test data isolation:** Each E2E test creates resources with unique names (timestamp-based) and cleans up in `test.afterEach()`.
- **Screenshot comparison:** Disabled by default. Enable with `expect(page).toHaveScreenshot()` for visual regression only after design stabilization.

### Testing Command

```bash
cd ui

# Unit tests with coverage
npx vitest run --coverage

# Watch mode for development
npx vitest watch

# Playwright E2E
npx playwright install
npx playwright test

# Playwright with UI mode (debugging)
npx playwright test --ui
```

### Pitfalls

- **jsdom limitations:** jsdom does not support `IntersectionObserver`, `ResizeObserver`, or `matchMedia`. Polyfill them in `test/setup.ts`.
- **Connect-ES mock:** gRPC client calls in components must be mocked. Use `vi.mock("@/lib/grpc-client")` and provide typed mock implementations.
- **Playwright auth token expiry:** Keycloak tokens expire after the configured session timeout. If E2E tests take longer than the token lifetime, the auth state file becomes stale. Set the Keycloak client session timeout to at least 1 hour for testing.

### Progress Marker

- [ ] Vitest configured with jsdom environment
- [ ] Component tests cover key interactions
- [ ] Hook tests mock gRPC client
- [ ] Coverage meets 70% threshold
- [ ] Playwright tests pass with persistent auth

---

## Contract Testing

Protobuf backward compatibility is enforced with `buf breaking` to prevent API breakage.

### Files

```
buf.yaml
buf.lock
.github/workflows/buf-breaking.yaml
```

### Key Code

**buf.yaml**

```yaml
version: v2
modules:
  - path: api/proto
lint:
  use:
    - DEFAULT
  except:
    - PACKAGE_VERSION_SUFFIX
breaking:
  use:
    - FILE
  except:
    - EXTENSION_NO_DELETE
```

**.github/workflows/buf-breaking.yaml**

```yaml
name: Protobuf Breaking Change Check

on:
  pull_request:
    paths:
      - 'api/proto/**'

jobs:
  breaking:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: bufbuild/buf-setup-action@v1
      - uses: bufbuild/buf-breaking-action@v1
        with:
          input: api/proto
          against: "https://github.com/${{ github.repository }}.git#branch=${{ github.event.pull_request.base.ref }},subdir=api/proto"
```

### Quality Gate

- `buf breaking` passes on every PR that modifies protobuf files.
- Breaking changes (field removal, type changes, enum value removal) are rejected.
- Non-breaking changes (new fields, new RPCs, new enum values) are allowed.

### Testing Command

```bash
# Check for breaking changes against main
buf breaking api/proto --against '.git#branch=main,subdir=api/proto'

# Lint protobuf files
buf lint api/proto

# Generate code
buf generate
```

### Pitfalls

- **Baseline reference:** `buf breaking` compares against a Git reference. Ensure the `against` reference is the correct branch (usually `main`, not `master`).
- **Reserved fields:** When removing a field, reserve the field number and name to prevent future reuse. `buf breaking` does not check for this, but it prevents subtle bugs.

### Progress Marker

- [ ] `buf.yaml` configured with breaking rules
- [ ] CI workflow runs on proto file changes
- [ ] Breaking changes blocked on PR
- [ ] Team educated on backward-compatible proto evolution

---

## Load Testing

Covered in detail in Phase 7, Step 4. Summary configuration for CI:

### Key Code

**k6 thresholds for CI**

```javascript
export const options = {
  thresholds: {
    http_req_duration: ["p(95)<300", "p(99)<500"],
    http_req_failed: ["rate<0.01"],
    mcp_tool_call_duration: ["p(95)<200", "p(99)<500"],
    mcp_tool_call_error_rate: ["rate<0.005"],
    mcp_discovery_duration: ["p(95)<100", "p(99)<250"],
  },
};
```

**Nightly CI job**

```yaml
# In .github/workflows/nightly.yaml
load-test:
  runs-on: ubuntu-latest
  needs: [deploy-staging]
  steps:
    - uses: actions/checkout@v4
    - uses: grafana/k6-action@v0.3.1
      with:
        filename: test/load/k6-mcp-load.js
      env:
        MCP_GATEWAY_URL: ${{ secrets.STAGING_URL }}
```

### Testing Command

```bash
# Quick smoke load test
k6 run --vus 10 --duration 30s test/load/k6-mcp-load.js

# Full load test
k6 run test/load/k6-mcp-load.js

# With cloud output (if using k6 Cloud)
k6 cloud test/load/k6-mcp-load.js
```

### Progress Marker

- [ ] k6 script with three scenarios
- [ ] Nightly CI job configured
- [ ] Results stored for trending
- [ ] Threshold violations alert the team

---

## Security Testing

### Files

```
.github/workflows/security-scan.yaml
.github/workflows/ossf-scorecard.yaml
hack/audit-rbac.sh
```

### Key Code

**Security scanning tools**

| Tool | What it checks | When it runs |
|------|---------------|-------------|
| Trivy (config) | Helm templates, Dockerfiles, K8s manifests | Every PR |
| Trivy (image) | Container image vulnerabilities | Every PR + nightly |
| govulncheck | Go dependency vulnerabilities | Every PR |
| OSSF Scorecard | Supply chain security posture | Weekly |
| RBAC audit | Wildcard verbs/resources, cluster-admin | Every PR |
| cosign verify | Image signature validation | Release pipeline |

**.github/workflows/ossf-scorecard.yaml**

```yaml
name: OSSF Scorecard

on:
  schedule:
    - cron: "0 0 * * 1"  # Monday midnight
  workflow_dispatch:

permissions:
  security-events: write
  id-token: write

jobs:
  scorecard:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
      - uses: ossf/scorecard-action@v2
        with:
          results_file: scorecard-results.sarif
          results_format: sarif
          publish_results: true
      - uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: scorecard-results.sarif
```

### Quality Gate

- Zero CRITICAL/HIGH vulnerabilities in Go deps and container images.
- RBAC audit passes with zero issues.
- OSSF Scorecard score above 7/10.
- All container images are signed with cosign.

### Testing Command

```bash
# Go vulnerability check
govulncheck ./...

# Trivy config scan
trivy config deploy/

# Trivy image scan
trivy image mcp-gateway-operator:latest

# RBAC audit
./hack/audit-rbac.sh

# OSSF Scorecard (local)
scorecard --repo=github.com/mcp-gateway/mcp-gateway
```

### Progress Marker

- [ ] govulncheck in CI
- [ ] Trivy config + image scanning in CI
- [ ] OSSF Scorecard running weekly
- [ ] RBAC audit in CI
- [ ] All images signed on release

---

## Coverage Targets

| Component | Target | Rationale |
|-----------|--------|-----------|
| Controllers (`internal/controller/`) | 80% | Core reconciliation logic is critical |
| xDS (`internal/xds/`) | 80% | Envoy config generation must be correct |
| Marketplace (`internal/marketplace/`) | 80% | Deploy flow handles user data |
| Observability (`internal/observability/`) | 70% | Mostly SDK wiring |
| API handlers (`internal/api/`) | 75% | User-facing endpoints |
| UI components | 70% | Key interactions covered |
| UI hooks | 70% | Data fetching logic |
| Overall Go | 70% | Balanced coverage |

**Enforcing coverage in CI**

```makefile
# Makefile
.PHONY: test-coverage
test-coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@echo "=== Coverage Report ==="
	@go tool cover -func=coverage.out | tail -1
	@COVERAGE=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	if [ $$(echo "$$COVERAGE < 70" | bc) -eq 1 ]; then \
		echo "FAIL: Overall coverage $$COVERAGE% is below 70% threshold"; \
		exit 1; \
	fi

.PHONY: test-coverage-controllers
test-coverage-controllers:
	go test ./internal/controller/... -coverprofile=controller-coverage.out
	@COVERAGE=$$(go tool cover -func=controller-coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	if [ $$(echo "$$COVERAGE < 80" | bc) -eq 1 ]; then \
		echo "FAIL: Controller coverage $$COVERAGE% is below 80% threshold"; \
		exit 1; \
	fi
```

---

## CI Integration

### Which tests run where

| Trigger | Unit | Integration | E2E | UI Unit | UI E2E | Contract | Load | Security |
|---------|------|-------------|-----|---------|--------|----------|------|----------|
| PR opened/updated | Yes | Yes | No | Yes | No | Yes (if proto changed) | No | Yes (Trivy, govulncheck) |
| Push to main | Yes | Yes | Yes | Yes | Yes | Yes | No | Yes |
| Nightly (cron) | No | No | Yes | No | Yes | No | Yes | Yes (OSSF) |
| Release tag | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes (full) |

### CI workflow structure

```yaml
# .github/workflows/ci.yaml (simplified)
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test ./... -short -race -count=1

  integration-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make envtest
      - run: go test -tags=integration ./internal/controller/... -v -timeout 10m

  e2e-test:
    if: github.event_name == 'push'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: helm/kind-action@v1
      - run: ./hack/setup-kind.sh
      - run: go test -tags=e2e ./test/e2e/... -v -timeout 20m

  ui-unit-test:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ui
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: npm
          cache-dependency-path: ui/package-lock.json
      - run: npm ci
      - run: npx vitest run --coverage

  ui-e2e-test:
    if: github.event_name == 'push'
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: ui
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
      - run: npm ci
      - run: npx playwright install --with-deps
      - run: npx playwright test

  contract-test:
    runs-on: ubuntu-latest
    if: contains(github.event.pull_request.changed_files, 'api/proto')
    steps:
      - uses: actions/checkout@v4
      - uses: bufbuild/buf-setup-action@v1
      - run: buf breaking api/proto --against '.git#branch=main,subdir=api/proto'

  coverage:
    needs: [unit-test, integration-test]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: make test-coverage
```

---

## Makefile Targets

```makefile
# Makefile targets for testing

##@ Testing

.PHONY: test
test: ## Run unit tests
	go test ./... -short -race -count=1

.PHONY: test-v
test-v: ## Run unit tests with verbose output
	go test ./... -short -race -v -count=1

.PHONY: test-integration
test-integration: envtest ## Run integration tests with envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test -tags=integration ./internal/controller/... -v -timeout 10m $(TEST_ARGS)

.PHONY: test-e2e
test-e2e: ## Run E2E tests (requires Kind cluster)
	go test -tags=e2e ./test/e2e/... -v -timeout 20m $(TEST_ARGS)

.PHONY: test-ui
test-ui: ## Run UI unit tests
	cd ui && npx vitest run --coverage

.PHONY: test-ui-e2e
test-ui-e2e: ## Run UI E2E tests with Playwright
	cd ui && npx playwright test

.PHONY: test-contract
test-contract: ## Run protobuf breaking change check
	buf breaking api/proto --against '.git#branch=main,subdir=api/proto'

.PHONY: test-load
test-load: ## Run k6 load tests
	cd test/load && ./run-load-test.sh

.PHONY: test-security
test-security: ## Run security scans
	govulncheck ./...
	trivy config deploy/
	./hack/audit-rbac.sh

.PHONY: test-coverage
test-coverage: ## Run tests with coverage and enforce thresholds
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out | tail -1
	@COVERAGE=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	if [ $$(echo "$$COVERAGE < 70" | bc) -eq 1 ]; then \
		echo "FAIL: Coverage $$COVERAGE%% below 70%% threshold"; \
		exit 1; \
	fi

.PHONY: test-all
test-all: test test-integration test-ui test-contract test-security ## Run all non-cluster tests
	@echo "All tests passed."

.PHONY: lint
lint: ## Run linters
	golangci-lint run ./...
	cd ui && npx next lint
	buf lint api/proto
```

Each target is independently runnable and maps directly to a CI job. Use `TEST_ARGS` to pass additional flags (e.g., `make test-integration TEST_ARGS="-run TestSpecificCase"`).
