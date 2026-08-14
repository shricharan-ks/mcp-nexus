# Phase 0: Project Bootstrap

**Goal:** `make kind-up && make dev-deploy` produces a 1/1 READY operator pod in a local Kind cluster.

**Definition of Done:** CI passes on GitHub Actions, operator pod runs in Kind, all foundational tooling is in place.

---

## Step 0.1: Git Init, License, and DCO

### Overview

Initialize the Git repository with an Apache-2.0 license and Developer Certificate of Origin (DCO) sign-off requirement.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `.gitignore` | Ignore Go build artifacts, IDE files, cluster state |
| `LICENSE` | Apache License 2.0 full text |
| `DCO` | Developer Certificate of Origin v1.1 |

### Key Code/Config

**.gitignore:**

```gitignore
# Binaries
bin/
*.exe
*.exe~
*.dll
*.so
*.dylib

# Go
*.test
*.out
go.work
go.work.sum
vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Kubernetes / Kind
kubeconfig
*.kubeconfig
kind-kubeconfig

# Helm
deploy/helm/charts/
*.tgz

# Coverage
cover.out
coverage.html
coverage.txt

# Environment
.env
.env.local

# Testbin (envtest)
testbin/

# Docker Compose volumes
data/

# Generated
config/crd/bases/
config/rbac/
config/webhook/
```

**LICENSE:** Use the full Apache License, Version 2.0 text from https://www.apache.org/licenses/LICENSE-2.0.txt. The file must begin with:

```
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/
```

**DCO:**

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

### Quality Gate

- `git log --oneline` shows initial commit
- LICENSE file present and contains "Apache License"
- DCO file present

### Testing Command

```bash
git init
git add .gitignore LICENSE DCO
git commit -s -m "chore: initial commit with Apache-2.0 license and DCO"
# Verify
head -1 LICENSE | grep -q "Apache" && echo "PASS" || echo "FAIL"
test -f DCO && echo "PASS" || echo "FAIL"
```

### Common Pitfalls

- Forgetting `-s` flag on commits (DCO sign-off). Configure a git hook or use `git config --local commit.gpgsign true` if needed.
- Using the wrong license text (e.g., MIT). Triple-check it is Apache-2.0.
- Not including `vendor/` in `.gitignore` if you ever run `go mod vendor`.

### Progress Marker

```
[x] 0.1 Git init, LICENSE, DCO
```

---

## Step 0.2: Go Module Init

### Overview

Initialize the Go module with the canonical import path. Set the minimum Go version to 1.23.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `go.mod` | Go module definition |

### Key Code/Config

```bash
go mod init github.com/mcp-gateway/mcp-gateway
```

This produces:

```go
module github.com/mcp-gateway/mcp-gateway

go 1.23
```

### Quality Gate

- `go.mod` exists with correct module path
- `go env GOMOD` points to the correct file

### Testing Command

```bash
go mod init github.com/mcp-gateway/mcp-gateway
cat go.mod
# Verify
grep -q "github.com/mcp-gateway/mcp-gateway" go.mod && echo "PASS" || echo "FAIL"
```

### Common Pitfalls

- Running `go mod init` from the wrong directory.
- Using a different module path than what Kubebuilder expects (must match exactly).
- Forgetting to set Go version to 1.23+ (required for some controller-runtime features).

### Progress Marker

```
[x] 0.2 Go module initialized
```

---

## Step 0.3: Kubebuilder Scaffold

### Overview

Use Kubebuilder to scaffold the operator project structure. This generates the base controller-runtime wiring, main.go, and Kustomize configs.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `PROJECT` | Kubebuilder project metadata |
| `cmd/main.go` | Operator entrypoint (will be moved to `cmd/operator/main.go`) |
| `config/` | Kustomize base configs |
| `internal/controller/` | Controller stubs |
| `Makefile` (generated) | Will be replaced in step 0.5 |

### Key Code/Config

```bash
# Install kubebuilder if not present
# curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
# chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

kubebuilder init \
  --domain mcp-gateway.io \
  --repo github.com/mcp-gateway/mcp-gateway \
  --project-name mcp-gateway \
  --owner "MCP Gateway Contributors"
```

After scaffold, relocate the entrypoint:

```bash
mkdir -p cmd/operator
mv cmd/main.go cmd/operator/main.go
```

Update any references in the generated Makefile/Dockerfile to point to `cmd/operator/main.go`.

In `cmd/operator/main.go`, update the import path and ensure the package is `main`:

```go
package main

import (
	"crypto/tls"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager, ensuring only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true, "If set, the metrics endpoint is served securely via HTTPS.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false, "If set, HTTP/2 will be enabled for the metrics and webhook servers.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		if !enableHTTP2 {
			c.NextProtos = []string{"http/1.1"}
		}
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: []func(*tls.Config){disableHTTP2},
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       []func(*tls.Config){disableHTTP2},
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "mcp-gateway.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
```

### Quality Gate

- `PROJECT` file exists with `domain: mcp-gateway.io`
- `cmd/operator/main.go` compiles
- `go build ./cmd/operator/` succeeds

### Testing Command

```bash
kubebuilder init --domain mcp-gateway.io --repo github.com/mcp-gateway/mcp-gateway --project-name mcp-gateway
mkdir -p cmd/operator && mv cmd/main.go cmd/operator/main.go
go build ./cmd/operator/
echo $?  # Should be 0
```

### Common Pitfalls

- Kubebuilder version mismatch: use v4.x for the latest scaffolding patterns.
- Forgetting to move `cmd/main.go` to `cmd/operator/main.go` before updating the Makefile.
- Module path in `PROJECT` file not matching `go.mod`. They must be identical.
- Running `kubebuilder init` in a directory that already has a `go.mod` with a different module path.

### Progress Marker

```
[x] 0.3 Kubebuilder scaffold complete
```

---

## Step 0.4: Directory Structure

### Overview

Create the full project directory layout. Some directories are scaffolded by Kubebuilder; others must be created manually.

### Files to Create/Modify

| Directory | Purpose |
|-----------|---------|
| `api/` | CRD Go types (versioned: `api/v1alpha1/`) |
| `cmd/operator/` | Operator binary entrypoint |
| `cmd/apiserver/` | API server binary entrypoint (future) |
| `internal/controller/` | Reconciler implementations |
| `internal/discovery/` | MCP server discovery and capability introspection |
| `internal/envoy/` | Envoy xDS / Gateway API integration |
| `internal/cerbos/` | Cerbos policy generation |
| `internal/keycloak/` | Keycloak OIDC integration |
| `internal/marketplace/` | Marketplace catalog logic |
| `internal/observability/` | Metrics, tracing, logging helpers |
| `deploy/helm/` | Helm chart |
| `scripts/kind/` | Kind cluster management scripts |
| `ui/` | Frontend (future) |
| `examples/` | Example CRs and configurations |

### Key Code/Config

```bash
# Create directories (some may already exist from Kubebuilder)
mkdir -p api/v1alpha1
mkdir -p cmd/operator
mkdir -p cmd/apiserver
mkdir -p internal/controller
mkdir -p internal/discovery
mkdir -p internal/envoy
mkdir -p internal/cerbos
mkdir -p internal/keycloak
mkdir -p internal/marketplace
mkdir -p internal/observability
mkdir -p deploy/helm/mcp-gateway/templates
mkdir -p scripts/kind
mkdir -p ui
mkdir -p examples
mkdir -p test/e2e
```

Place a `.gitkeep` in empty directories that need to be tracked:

```bash
for dir in cmd/apiserver internal/envoy internal/cerbos internal/keycloak \
           internal/marketplace internal/observability ui test/e2e; do
  touch "$dir/.gitkeep"
done
```

### Quality Gate

- All directories exist
- `find . -type d -name "internal" | head -1` returns `./internal`
- `ls internal/` shows all subdirectories

### Testing Command

```bash
# Verify all directories exist
for dir in api/v1alpha1 cmd/operator cmd/apiserver internal/controller \
           internal/discovery internal/envoy internal/cerbos internal/keycloak \
           internal/marketplace internal/observability deploy/helm/mcp-gateway/templates \
           scripts/kind ui examples test/e2e; do
  test -d "$dir" && echo "OK: $dir" || echo "MISSING: $dir"
done
```

### Common Pitfalls

- Creating `pkg/` instead of `internal/`. This project uses `internal/` to enforce Go's visibility rules.
- Forgetting `api/v1alpha1/` versioned directory. CRD types must live in a versioned package.
- Not creating `test/e2e/` which is needed for end-to-end test scaffolding.

### Progress Marker

```
[x] 0.4 Directory structure created
```

---

## Step 0.5: Makefile

### Overview

Create a comprehensive Makefile with all required targets. This replaces the Kubebuilder-generated Makefile with a more complete version.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `Makefile` | Build, test, deploy automation |

### Key Code/Config

```makefile
# ==============================================================================
# MCP Gateway Makefile
# ==============================================================================

# Project settings
PROJECT_NAME     := mcp-gateway
MODULE           := github.com/mcp-gateway/mcp-gateway
BINARY_OPERATOR  := bin/operator
BINARY_APISERVER := bin/apiserver

# Go settings
GO               := go
GOFLAGS          := -trimpath
CGO_ENABLED      := 0
GOOS             ?= $(shell go env GOOS)
GOARCH           ?= $(shell go env GOARCH)

# Container settings
IMG_REGISTRY     ?= ghcr.io/mcp-gateway
IMG_OPERATOR     ?= $(IMG_REGISTRY)/operator
IMG_APISERVER    ?= $(IMG_REGISTRY)/apiserver
IMG_TAG          ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKERFILE       ?= Dockerfile

# Kind settings
KIND_CLUSTER     ?= mcp-gateway
KIND_CONFIG      ?= scripts/kind/kind-config.yaml
KIND_IMAGE       ?= kindest/node:v1.31.0

# Helm settings
HELM_CHART       := deploy/helm/mcp-gateway
HELM_RELEASE     := mcp-gateway
HELM_NAMESPACE   := mcp-gateway-system

# Tool versions
CONTROLLER_GEN_VERSION  ?= v0.16.4
ENVTEST_VERSION         ?= release-0.19
GOLANGCI_LINT_VERSION   ?= v1.62.2
KUSTOMIZE_VERSION       ?= v5.5.0

# Tool binaries
LOCALBIN     ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST        ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT  ?= $(LOCALBIN)/golangci-lint
KUSTOMIZE      ?= $(LOCALBIN)/kustomize

.PHONY: all
all: lint test build ## Run lint, test, build (default target)

# ==============================================================================
# Help
# ==============================================================================

.PHONY: help
help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==============================================================================
# Development
# ==============================================================================

.PHONY: generate
generate: controller-gen ## Generate code (DeepCopy, etc.)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD manifests, RBAC, webhook configs
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." \
		output:crd:artifacts:config=config/crd/bases

.PHONY: fmt
fmt: ## Run go fmt
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

# ==============================================================================
# Lint
# ==============================================================================

.PHONY: lint
lint: golangci-lint ## Run golangci-lint
	$(GOLANGCI_LINT) run --timeout 5m ./...

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint with --fix
	$(GOLANGCI_LINT) run --fix --timeout 5m ./...

# ==============================================================================
# Test
# ==============================================================================

.PHONY: test
test: test-unit ## Run all tests (alias for test-unit)

.PHONY: test-unit
test-unit: generate fmt vet ## Run unit tests
	CGO_ENABLED=1 $(GO) test ./internal/... ./api/... -coverprofile cover.out -race -count=1
	@echo ""
	@echo "Coverage:"
	@$(GO) tool cover -func=cover.out | tail -1

.PHONY: test-integration
test-integration: generate fmt vet envtest ## Run integration tests with envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		CGO_ENABLED=1 $(GO) test ./internal/controller/... -coverprofile cover-integration.out -race -count=1 -tags=integration

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests against a running cluster
	$(GO) test ./test/e2e/... -v -count=1 -timeout 30m -tags=e2e

.PHONY: test-coverage
test-coverage: test-unit ## Generate HTML coverage report
	$(GO) tool cover -html=cover.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ==============================================================================
# Build
# ==============================================================================

.PHONY: build
build: build-operator ## Build all binaries

.PHONY: build-operator
build-operator: generate fmt vet ## Build the operator binary
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(GOFLAGS) -o $(BINARY_OPERATOR) ./cmd/operator/

.PHONY: build-apiserver
build-apiserver: fmt vet ## Build the apiserver binary
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(GOFLAGS) -o $(BINARY_APISERVER) ./cmd/apiserver/

# ==============================================================================
# Docker
# ==============================================================================

.PHONY: docker-build
docker-build: ## Build Docker image for the operator
	docker build -t $(IMG_OPERATOR):$(IMG_TAG) \
		--build-arg BINARY=operator \
		--build-arg CMD_PATH=./cmd/operator/ \
		-f $(DOCKERFILE) .

.PHONY: docker-build-all
docker-build-all: ## Build all Docker images
	docker build -t $(IMG_OPERATOR):$(IMG_TAG) \
		--build-arg BINARY=operator \
		--build-arg CMD_PATH=./cmd/operator/ \
		-f $(DOCKERFILE) .

.PHONY: docker-push
docker-push: ## Push Docker image
	docker push $(IMG_OPERATOR):$(IMG_TAG)

.PHONY: docker-load-kind
docker-load-kind: docker-build ## Load Docker image into Kind cluster
	kind load docker-image $(IMG_OPERATOR):$(IMG_TAG) --name $(KIND_CLUSTER)

# ==============================================================================
# Kind Cluster
# ==============================================================================

.PHONY: kind-up
kind-up: ## Create Kind cluster
	./scripts/kind/create-cluster.sh

.PHONY: kind-down
kind-down: ## Delete Kind cluster
	kind delete cluster --name $(KIND_CLUSTER)

# ==============================================================================
# Deploy (Dev)
# ==============================================================================

.PHONY: dev-deploy
dev-deploy: docker-load-kind ## Deploy to Kind for development
	helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) \
		--create-namespace \
		--set image.repository=$(IMG_OPERATOR) \
		--set image.tag=$(IMG_TAG) \
		--set image.pullPolicy=Never \
		--wait --timeout 120s
	@echo ""
	@echo "Operator deployed. Checking pod status..."
	@kubectl -n $(HELM_NAMESPACE) get pods

.PHONY: dev-teardown
dev-teardown: ## Remove dev deployment
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE) || true
	kubectl delete namespace $(HELM_NAMESPACE) || true

.PHONY: dev-redeploy
dev-redeploy: dev-teardown dev-deploy ## Teardown and redeploy

.PHONY: dev-logs
dev-logs: ## Tail operator logs
	kubectl -n $(HELM_NAMESPACE) logs -f deployment/$(HELM_RELEASE)-operator

# ==============================================================================
# Helm
# ==============================================================================

.PHONY: helm-lint
helm-lint: ## Lint Helm chart
	helm lint $(HELM_CHART)

.PHONY: helm-template
helm-template: ## Render Helm templates locally
	helm template $(HELM_RELEASE) $(HELM_CHART) --namespace $(HELM_NAMESPACE)

.PHONY: helm-package
helm-package: ## Package Helm chart
	helm package $(HELM_CHART)

# ==============================================================================
# Install Tools
# ==============================================================================

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_GEN_VERSION))

.PHONY: envtest
envtest: $(ENVTEST)
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

.PHONY: kustomize
kustomize: $(KUSTOMIZE)
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

# ==============================================================================
# Utilities
# ==============================================================================

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/ cover.out cover-integration.out coverage.html testbin/

# go-install-tool will 'go install' any package with custom target and target version.
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Installing $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
}
endef
```

Also create the boilerplate header file:

```bash
mkdir -p hack
```

**hack/boilerplate.go.txt:**

```
/*
Copyright 2024 MCP Gateway Contributors.

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
```

### Quality Gate

- `make help` prints all targets
- `make lint` runs (may have no Go files to lint yet, which is fine)
- `make build` succeeds (after step 0.3 provides `cmd/operator/main.go`)

### Testing Command

```bash
make help
make build
echo $?  # Should be 0
```

### Common Pitfalls

- Tabs vs spaces: Makefiles **require** tabs for indentation. If you copy-paste from this doc, ensure tabs are preserved.
- Missing `$(LOCALBIN)` directory creation: the tool install targets depend on it.
- `CGO_ENABLED=1` is needed for tests with `-race` flag, but `CGO_ENABLED=0` for binary builds.
- The `go-install-tool` macro uses `@` prefix to suppress echoing; some shells handle this differently.

### Progress Marker

```
[x] 0.5 Makefile complete
```

---

## Step 0.6: Dockerfile

### Overview

Create a multi-stage Dockerfile using distroless base image, building with `CGO_ENABLED=0` and running as nonroot user (UID 65532).

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `Dockerfile` | Multi-stage container build |
| `.dockerignore` | Exclude unnecessary files from build context |

### Key Code/Config

**Dockerfile:**

```dockerfile
# ==============================================================================
# Stage 1: Build
# ==============================================================================
FROM golang:1.23-alpine AS builder

ARG BINARY=operator
ARG CMD_PATH=./cmd/operator/
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /workspace

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /workspace/bin/${BINARY} ${CMD_PATH}

# ==============================================================================
# Stage 2: Runtime
# ==============================================================================
FROM gcr.io/distroless/static:nonroot

ARG BINARY=operator

WORKDIR /

COPY --from=builder /workspace/bin/${BINARY} /manager

# 65532 is the UID for nonroot user in distroless
USER 65532:65532

ENTRYPOINT ["/manager"]
```

**.dockerignore:**

```
# Version control
.git
.gitignore

# IDE
.idea/
.vscode/

# Documentation
docs/
*.md
!README.md

# CI
.github/

# Testing
test/
testbin/
cover.out
coverage.html

# Local dev
kind-kubeconfig
data/
bin/

# Docker Compose
docker-compose.yaml
docker-compose.yml
```

### Quality Gate

- `docker build -t mcp-gateway-operator:test .` succeeds
- Container runs as nonroot: `docker run --rm mcp-gateway-operator:test whoami` should fail (no shell in distroless)
- Image size is small (< 50MB)

### Testing Command

```bash
docker build -t mcp-gateway-operator:test \
  --build-arg BINARY=operator \
  --build-arg CMD_PATH=./cmd/operator/ .

# Check image size
docker images mcp-gateway-operator:test --format '{{.Size}}'

# Verify nonroot
docker inspect mcp-gateway-operator:test --format '{{.Config.User}}'
# Should output: 65532:65532
```

### Common Pitfalls

- Using `golang:1.23` (Debian-based) instead of `golang:1.23-alpine` for the builder. Alpine is smaller and faster to pull.
- Forgetting `CGO_ENABLED=0`. Without this, the binary may dynamically link libc and crash in distroless.
- Not using `-trimpath` and `-ldflags="-s -w"` for production builds. These reduce binary size significantly.
- Copying the entire build context before `go mod download`. Always copy `go.mod` and `go.sum` first for layer caching.
- Using `scratch` instead of `distroless/static:nonroot`. Distroless includes CA certificates and timezone data.

### Progress Marker

```
[x] 0.6 Dockerfile complete
```

---

## Step 0.7: Kind Cluster Config

### Overview

Create a Kind cluster configuration with 3 nodes (1 control-plane, 2 workers) and port mappings for development access.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `scripts/kind/kind-config.yaml` | Kind cluster configuration |

### Key Code/Config

**scripts/kind/kind-config.yaml:**

```yaml
# Kind cluster configuration for MCP Gateway development
# Creates a 3-node cluster with port mappings for local access
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: mcp-gateway
networking:
  # Allow access to services via NodePort
  apiServerAddress: "127.0.0.1"
  apiServerPort: 6443
nodes:
  # Control plane node with port mappings
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      # HTTP
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      # HTTPS
      - containerPort: 443
        hostPort: 443
        protocol: TCP
      # NodePort for direct service access
      - containerPort: 30080
        hostPort: 30080
        protocol: TCP
  # Worker nodes
  - role: worker
  - role: worker
```

### Quality Gate

- YAML is valid: `python3 -c "import yaml; yaml.safe_load(open('scripts/kind/kind-config.yaml'))"`
- Kind can parse it: `kind create cluster --config scripts/kind/kind-config.yaml --dry-run` (if Kind supports dry-run)

### Testing Command

```bash
# Validate YAML
python3 -c "import yaml; yaml.safe_load(open('scripts/kind/kind-config.yaml')); print('VALID')"

# Quick test: create and delete
kind create cluster --config scripts/kind/kind-config.yaml --name mcp-gateway-test
kubectl get nodes  # Should show 3 nodes
kind delete cluster --name mcp-gateway-test
```

### Common Pitfalls

- Port 80/443 already in use by another service (e.g., a local web server or another Kind cluster). Check with `lsof -i :80`.
- Using `apiServerAddress: "0.0.0.0"` which exposes the API server to all network interfaces. Use `127.0.0.1` for security.
- Forgetting the `ingress-ready=true` label on the control-plane node, which is required for nginx-ingress or Gateway API controllers.
- Kind cluster name conflicts. The `name` field in the YAML must match what scripts and Makefile use.

### Progress Marker

```
[x] 0.7 Kind cluster config complete
```

---

## Step 0.8: Kind Scripts

### Overview

Create shell scripts for creating the Kind cluster and deploying platform dependencies (metrics-server, ingress controller, etc.).

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `scripts/kind/create-cluster.sh` | Create Kind cluster with config |
| `scripts/kind/deploy-platform.sh` | Deploy platform dependencies |

### Key Code/Config

**scripts/kind/create-cluster.sh:**

```bash
#!/usr/bin/env bash
# =============================================================================
# create-cluster.sh - Create a Kind cluster for MCP Gateway development
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CLUSTER_NAME="${KIND_CLUSTER:-mcp-gateway}"
KIND_CONFIG="${SCRIPT_DIR}/kind-config.yaml"
KIND_IMAGE="${KIND_IMAGE:-kindest/node:v1.31.0}"
KUBECONFIG_PATH="${ROOT_DIR}/kind-kubeconfig"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Check prerequisites
check_prerequisites() {
    local missing=0
    for cmd in kind kubectl docker; do
        if ! command -v "$cmd" &>/dev/null; then
            log_error "$cmd is not installed"
            missing=1
        fi
    done
    if [ "$missing" -eq 1 ]; then
        exit 1
    fi

    if ! docker info &>/dev/null; then
        log_error "Docker is not running"
        exit 1
    fi
}

# Check if cluster already exists
cluster_exists() {
    kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"
}

main() {
    log_info "MCP Gateway Kind Cluster Setup"
    log_info "=============================="

    check_prerequisites

    if cluster_exists; then
        log_warn "Cluster '${CLUSTER_NAME}' already exists"
        read -r -p "Delete and recreate? [y/N] " response
        case "$response" in
            [yY][eE][sS]|[yY])
                log_info "Deleting existing cluster..."
                kind delete cluster --name "${CLUSTER_NAME}"
                ;;
            *)
                log_info "Using existing cluster"
                exit 0
                ;;
        esac
    fi

    log_info "Creating Kind cluster '${CLUSTER_NAME}'..."
    kind create cluster \
        --name "${CLUSTER_NAME}" \
        --config "${KIND_CONFIG}" \
        --image "${KIND_IMAGE}" \
        --wait 120s

    # Export kubeconfig
    kind get kubeconfig --name "${CLUSTER_NAME}" > "${KUBECONFIG_PATH}"
    export KUBECONFIG="${KUBECONFIG_PATH}"

    log_info "Cluster created successfully!"
    log_info ""

    # Show cluster info
    log_info "Cluster nodes:"
    kubectl get nodes -o wide

    log_info ""
    log_info "Kubeconfig exported to: ${KUBECONFIG_PATH}"
    log_info "To use: export KUBECONFIG=${KUBECONFIG_PATH}"
    log_info ""

    # Deploy platform components
    log_info "Deploying platform components..."
    "${SCRIPT_DIR}/deploy-platform.sh"

    log_info ""
    log_info "Cluster is ready! Run 'make dev-deploy' to deploy the operator."
}

main "$@"
```

**scripts/kind/deploy-platform.sh:**

```bash
#!/usr/bin/env bash
# =============================================================================
# deploy-platform.sh - Deploy platform dependencies to the Kind cluster
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CLUSTER_NAME="${KIND_CLUSTER:-mcp-gateway}"
KUBECONFIG_PATH="${ROOT_DIR}/kind-kubeconfig"

# Use the Kind kubeconfig if it exists
if [ -f "${KUBECONFIG_PATH}" ]; then
    export KUBECONFIG="${KUBECONFIG_PATH}"
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC}  $*"; }

# Deploy metrics-server (required for HPA)
deploy_metrics_server() {
    log_info "Deploying metrics-server..."
    kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

    # Patch for Kind (insecure TLS)
    kubectl -n kube-system patch deployment metrics-server \
        --type='json' \
        -p='[
          {
            "op": "add",
            "path": "/spec/template/spec/containers/0/args/-",
            "value": "--kubelet-insecure-tls"
          }
        ]' 2>/dev/null || true

    log_info "Waiting for metrics-server to be ready..."
    kubectl -n kube-system wait --for=condition=available deployment/metrics-server \
        --timeout=120s 2>/dev/null || log_warn "metrics-server not ready yet (non-blocking)"
}

# Deploy nginx ingress controller
deploy_ingress() {
    log_info "Deploying nginx ingress controller..."
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

    log_info "Waiting for ingress controller to be ready..."
    kubectl -n ingress-nginx wait --for=condition=available deployment/ingress-nginx-controller \
        --timeout=120s 2>/dev/null || log_warn "ingress controller not ready yet (non-blocking)"
}

# Create the operator namespace
create_namespace() {
    log_info "Creating mcp-gateway-system namespace..."
    kubectl create namespace mcp-gateway-system --dry-run=client -o yaml | kubectl apply -f -
}

main() {
    log_info "Deploying platform components..."

    deploy_metrics_server
    deploy_ingress
    create_namespace

    log_info ""
    log_info "Platform components deployed."
    log_info "Cluster is ready for operator deployment."
}

main "$@"
```

Make scripts executable:

```bash
chmod +x scripts/kind/create-cluster.sh
chmod +x scripts/kind/deploy-platform.sh
```

### Quality Gate

- Both scripts are executable: `test -x scripts/kind/create-cluster.sh`
- `shellcheck scripts/kind/*.sh` passes (install shellcheck if needed)
- `make kind-up` creates a 3-node cluster

### Testing Command

```bash
# Check scripts are executable
test -x scripts/kind/create-cluster.sh && echo "PASS" || echo "FAIL"
test -x scripts/kind/deploy-platform.sh && echo "PASS" || echo "FAIL"

# Run shellcheck (if available)
shellcheck scripts/kind/*.sh

# Full integration test
make kind-up
kubectl get nodes  # Should show 3 nodes
make kind-down
```

### Common Pitfalls

- Forgetting `chmod +x` on the scripts.
- `set -euo pipefail` with uninitialized variables: use `${VAR:-default}` pattern.
- metrics-server patch failing on re-runs (idempotency). Use `|| true` for patch commands.
- `read -r -p` in `create-cluster.sh` hangs in CI. In CI, set `KIND_CLUSTER_EXISTS=replace` env var or skip the prompt.

### Progress Marker

```
[x] 0.8 Kind scripts complete
```

---

## Step 0.9: Helm Chart Skeleton

### Overview

Create the Helm chart for deploying the MCP Gateway operator. This is a minimal chart that deploys the operator Deployment, ServiceAccount, RBAC, and Service.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `deploy/helm/mcp-gateway/Chart.yaml` | Chart metadata |
| `deploy/helm/mcp-gateway/values.yaml` | Default configuration values |
| `deploy/helm/mcp-gateway/templates/_helpers.tpl` | Template helper functions |
| `deploy/helm/mcp-gateway/templates/deployment.yaml` | Operator Deployment |
| `deploy/helm/mcp-gateway/templates/serviceaccount.yaml` | ServiceAccount |
| `deploy/helm/mcp-gateway/templates/rbac.yaml` | ClusterRole and ClusterRoleBinding |
| `deploy/helm/mcp-gateway/templates/service.yaml` | Metrics/health Service |
| `deploy/helm/mcp-gateway/templates/NOTES.txt` | Post-install notes |

### Key Code/Config

**deploy/helm/mcp-gateway/Chart.yaml:**

```yaml
apiVersion: v2
name: mcp-gateway
description: A Kubernetes operator for managing MCP (Model Context Protocol) server lifecycles
type: application
version: 0.1.0
appVersion: "0.1.0"
kubeVersion: ">=1.28.0"
home: https://github.com/mcp-gateway/mcp-gateway
sources:
  - https://github.com/mcp-gateway/mcp-gateway
maintainers:
  - name: MCP Gateway Contributors
    url: https://github.com/mcp-gateway
keywords:
  - mcp
  - model-context-protocol
  - kubernetes
  - operator
  - ai
  - llm
```

**deploy/helm/mcp-gateway/values.yaml:**

```yaml
# Default values for mcp-gateway operator
# This is a YAML-formatted file.

# -- Number of operator replicas
replicaCount: 1

image:
  # -- Container image repository
  repository: ghcr.io/mcp-gateway/operator
  # -- Image pull policy
  pullPolicy: IfNotPresent
  # -- Overrides the image tag (default is the chart appVersion)
  tag: ""

# -- Image pull secrets
imagePullSecrets: []
# -- Override the chart name
nameOverride: ""
# -- Override the full release name
fullnameOverride: ""

serviceAccount:
  # -- Specifies whether a service account should be created
  create: true
  # -- Annotations to add to the service account
  annotations: {}
  # -- The name of the service account to use.
  # If not set and create is true, a name is generated using the fullname template
  name: ""

# -- Pod annotations
podAnnotations: {}

# -- Pod labels
podLabels: {}

# -- Pod security context
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  fsGroup: 65532
  seccompProfile:
    type: RuntimeDefault

# -- Container security context
securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL

# Operator configuration
operator:
  # -- Enable leader election
  leaderElect: true
  # -- Metrics bind address (set to "0" to disable)
  metricsBindAddress: "0"
  # -- Health probe bind address
  healthProbeBindAddress: ":8081"
  # -- Log level (debug, info, error)
  logLevel: "info"

service:
  # -- Service type
  type: ClusterIP
  # -- Health/readiness port
  port: 8081

resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 100m
    memory: 64Mi

# -- Node selector
nodeSelector: {}

# -- Tolerations
tolerations: []

# -- Affinity rules
affinity: {}
```

**deploy/helm/mcp-gateway/templates/_helpers.tpl:**

```yaml
{{/*
Expand the name of the chart.
*/}}
{{- define "mcp-gateway.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "mcp-gateway.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "mcp-gateway.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "mcp-gateway.labels" -}}
helm.sh/chart: {{ include "mcp-gateway.chart" . }}
{{ include "mcp-gateway.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: mcp-gateway
{{- end }}

{{/*
Selector labels
*/}}
{{- define "mcp-gateway.selectorLabels" -}}
app.kubernetes.io/name: {{ include "mcp-gateway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: operator
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "mcp-gateway.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-operator" (include "mcp-gateway.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Operator image
*/}}
{{- define "mcp-gateway.operatorImage" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end }}
```

**deploy/helm/mcp-gateway/templates/serviceaccount.yaml:**

```yaml
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "mcp-gateway.serviceAccountName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "mcp-gateway.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
automountServiceAccountToken: true
{{- end }}
```

**deploy/helm/mcp-gateway/templates/rbac.yaml:**

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "mcp-gateway.fullname" . }}-manager-role
  labels:
    {{- include "mcp-gateway.labels" . | nindent 4 }}
rules:
  # Core resources
  - apiGroups: [""]
    resources: ["pods", "services", "configmaps", "secrets", "events"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get", "list"]
  # Apps resources
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  # MCP Gateway CRDs
  - apiGroups: ["gateway.mcp-gateway.io"]
    resources: ["mcpservers"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["gateway.mcp-gateway.io"]
    resources: ["mcpservers/status"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["gateway.mcp-gateway.io"]
    resources: ["mcpservers/finalizers"]
    verbs: ["update"]
  # Coordination (leader election)
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "mcp-gateway.fullname" . }}-manager-rolebinding
  labels:
    {{- include "mcp-gateway.labels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "mcp-gateway.fullname" . }}-manager-role
subjects:
  - kind: ServiceAccount
    name: {{ include "mcp-gateway.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
```

**deploy/helm/mcp-gateway/templates/deployment.yaml:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "mcp-gateway.fullname" . }}-operator
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "mcp-gateway.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "mcp-gateway.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      {{- with .Values.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      labels:
        {{- include "mcp-gateway.labels" . | nindent 8 }}
        {{- with .Values.podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "mcp-gateway.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      terminationGracePeriodSeconds: 10
      containers:
        - name: operator
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          image: {{ include "mcp-gateway.operatorImage" . }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - --leader-elect={{ .Values.operator.leaderElect }}
            - --metrics-bind-address={{ .Values.operator.metricsBindAddress }}
            - --health-probe-bind-address={{ .Values.operator.healthProbeBindAddress }}
          ports:
            - name: health
              containerPort: 8081
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: health
            initialDelaySeconds: 15
            periodSeconds: 20
            timeoutSeconds: 3
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: health
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
```

**deploy/helm/mcp-gateway/templates/service.yaml:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ include "mcp-gateway.fullname" . }}-operator
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "mcp-gateway.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.port }}
      targetPort: health
      protocol: TCP
      name: health
  selector:
    {{- include "mcp-gateway.selectorLabels" . | nindent 4 }}
```

**deploy/helm/mcp-gateway/templates/NOTES.txt:**

```
================================================================================
  MCP Gateway Operator has been deployed!
================================================================================

Namespace: {{ .Release.Namespace }}

To check the operator status:

  kubectl -n {{ .Release.Namespace }} get pods

To view operator logs:

  kubectl -n {{ .Release.Namespace }} logs -f deployment/{{ include "mcp-gateway.fullname" . }}-operator

To create your first MCP Server:

  cat <<EOF | kubectl apply -f -
  apiVersion: gateway.mcp-gateway.io/v1alpha1
  kind: MCPServer
  metadata:
    name: echo-server
    namespace: default
  spec:
    image: ghcr.io/modelcontextprotocol/echo-server:latest
    transport: stdio
  EOF

To check MCP Server status:

  kubectl get mcpservers

For more information, visit:
  https://github.com/mcp-gateway/mcp-gateway
```

### Quality Gate

- `helm lint deploy/helm/mcp-gateway` passes
- `helm template test deploy/helm/mcp-gateway` renders valid YAML
- All templates contain correct label references

### Testing Command

```bash
# Lint the chart
helm lint deploy/helm/mcp-gateway

# Template rendering
helm template test deploy/helm/mcp-gateway --namespace mcp-gateway-system

# Verify specific template
helm template test deploy/helm/mcp-gateway -s templates/deployment.yaml

# Validate output is valid YAML
helm template test deploy/helm/mcp-gateway | python3 -c "
import sys, yaml
try:
    list(yaml.safe_load_all(sys.stdin))
    print('VALID YAML')
except yaml.YAMLError as e:
    print(f'INVALID: {e}')
    sys.exit(1)
"
```

### Common Pitfalls

- Mismatched indentation in `_helpers.tpl` causing invalid YAML output. Use `nindent` carefully.
- Forgetting `{{ .Release.Namespace }}` in metadata, causing resources to deploy to the wrong namespace.
- Not including `automountServiceAccountToken: true` on the ServiceAccount when the operator needs API access.
- RBAC rules not covering all resources the operator needs. The ClusterRole must include CRD resources.
- Missing `terminationGracePeriodSeconds` on the Deployment, which defaults to 30s. Set explicitly for operator pods.

### Progress Marker

```
[x] 0.9 Helm chart skeleton complete
```

---

## Step 0.10: docker-compose.yaml

### Overview

Create a Docker Compose file for running the operator and its dependencies locally without Kubernetes.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `docker-compose.yaml` | Local development stack |

### Key Code/Config

**docker-compose.yaml:**

```yaml
# Docker Compose for local development
# Usage: docker compose up -d
#
# This runs supporting services for local development.
# The operator itself typically runs via `go run` or in Kind.

services:
  # ============================================================================
  # Operator (optional - usually run via `go run` or Kind)
  # ============================================================================
  operator:
    build:
      context: .
      dockerfile: Dockerfile
      args:
        BINARY: operator
        CMD_PATH: ./cmd/operator/
    image: mcp-gateway-operator:dev
    ports:
      - "8081:8081"  # Health probes
    environment:
      - KUBECONFIG=/etc/kubernetes/kubeconfig
      - LOG_LEVEL=debug
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    profiles:
      - full  # Only start with: docker compose --profile full up

  # ============================================================================
  # PostgreSQL - State store
  # ============================================================================
  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: mcpgateway
      POSTGRES_USER: mcpgateway
      POSTGRES_PASSWORD: mcpgateway-dev
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U mcpgateway -d mcpgateway"]
      interval: 5s
      timeout: 5s
      retries: 5
      start_period: 10s
    restart: unless-stopped

  # ============================================================================
  # Redis - Caching and pub/sub
  # ============================================================================
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes --maxmemory 128mb --maxmemory-policy allkeys-lru
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
      start_period: 5s
    restart: unless-stopped

  # ============================================================================
  # OpenTelemetry Collector - Observability pipeline
  # ============================================================================
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.112.0
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
      - "8888:8888"   # Prometheus metrics (collector)
      - "8889:8889"   # Prometheus exporter
      - "13133:13133" # Health check
    volumes:
      - ./config/otel-collector.yaml:/etc/otelcol-contrib/config.yaml:ro
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:13133/"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 10s
    restart: unless-stopped
    depends_on:
      - postgres

volumes:
  postgres-data:
    driver: local
  redis-data:
    driver: local
```

Also create the OTel collector config:

**config/otel-collector.yaml:**

```yaml
# OpenTelemetry Collector configuration for local development
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1024
  memory_limiter:
    check_interval: 5s
    limit_mib: 256
    spike_limit_mib: 128

exporters:
  debug:
    verbosity: detailed
  prometheus:
    endpoint: 0.0.0.0:8889
    namespace: mcp_gateway

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug, prometheus]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug]
```

### Quality Gate

- `docker compose config` validates the compose file
- `docker compose up -d postgres redis` starts both services
- PostgreSQL is reachable: `psql -h localhost -U mcpgateway -d mcpgateway`
- Redis is reachable: `redis-cli ping` returns `PONG`

### Testing Command

```bash
# Validate compose file
docker compose config --quiet && echo "VALID" || echo "INVALID"

# Start dependencies only
docker compose up -d postgres redis otel-collector

# Wait for health checks
docker compose ps

# Test PostgreSQL
docker compose exec postgres pg_isready -U mcpgateway && echo "PG READY" || echo "PG NOT READY"

# Test Redis
docker compose exec redis redis-cli ping

# Cleanup
docker compose down -v
```

### Common Pitfalls

- Using `docker-compose` (v1) instead of `docker compose` (v2). The Compose V2 plugin uses `docker compose` (space, not hyphen).
- Port conflicts: 5432 (PostgreSQL), 6379 (Redis), 4317/4318 (OTel) may already be in use.
- Forgetting `condition: service_healthy` on `depends_on` causes race conditions.
- Volume data persisting between runs. Use `docker compose down -v` to clean up.
- Missing `config/otel-collector.yaml` causes the collector to fail to start. Create the config directory and file before running.

### Progress Marker

```
[x] 0.10 docker-compose.yaml complete
```

---

## Step 0.11: GitHub Actions CI

### Overview

Set up a comprehensive GitHub Actions CI pipeline with lint, test, build, helm-lint, and e2e jobs.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `.github/workflows/ci.yaml` | Main CI pipeline |

### Key Code/Config

**.github/workflows/ci.yaml:**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

env:
  GO_VERSION: "1.23"
  KIND_VERSION: "v0.24.0"
  KIND_CLUSTER: "mcp-gateway-ci"

jobs:
  # ============================================================================
  # Lint
  # ============================================================================
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: v1.62.2
          args: --timeout 5m

      - name: Run go vet
        run: go vet ./...

      - name: Check go mod tidy
        run: |
          go mod tidy
          git diff --exit-code go.mod go.sum

  # ============================================================================
  # Unit Tests
  # ============================================================================
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Run unit tests
        run: |
          CGO_ENABLED=1 go test ./internal/... ./api/... \
            -coverprofile=cover.out \
            -race \
            -count=1 \
            -v

      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: cover.out

      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=cover.out | tail -1 | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: ${COVERAGE}%"
          # Fail if coverage is below 50% (will increase over time)
          if (( $(echo "$COVERAGE < 50" | bc -l) )); then
            echo "::error::Coverage ${COVERAGE}% is below 50% threshold"
            exit 1
          fi

  # ============================================================================
  # Integration Tests (envtest)
  # ============================================================================
  test-integration:
    name: Integration Tests
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Install envtest binaries
        run: |
          go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.19
          setup-envtest use --bin-dir ./testbin

      - name: Run integration tests
        run: |
          export KUBEBUILDER_ASSETS="$(setup-envtest use --bin-dir ./testbin -p path)"
          CGO_ENABLED=1 go test ./internal/controller/... \
            -coverprofile=cover-integration.out \
            -race \
            -count=1 \
            -v \
            -tags=integration

  # ============================================================================
  # Build
  # ============================================================================
  build:
    name: Build
    runs-on: ubuntu-latest
    strategy:
      matrix:
        goos: [linux]
        goarch: [amd64, arm64]
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Build operator
        run: |
          CGO_ENABLED=0 GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} \
            go build -trimpath -ldflags="-s -w" -o bin/operator-${{ matrix.goos }}-${{ matrix.goarch }} \
            ./cmd/operator/

      - name: Upload binary
        uses: actions/upload-artifact@v4
        with:
          name: operator-${{ matrix.goos }}-${{ matrix.goarch }}
          path: bin/operator-${{ matrix.goos }}-${{ matrix.goarch }}

  # ============================================================================
  # Docker Build
  # ============================================================================
  docker-build:
    name: Docker Build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build Docker image
        uses: docker/build-push-action@v6
        with:
          context: .
          push: false
          tags: ghcr.io/mcp-gateway/operator:ci
          build-args: |
            BINARY=operator
            CMD_PATH=./cmd/operator/
          cache-from: type=gha
          cache-to: type=gha,mode=max

  # ============================================================================
  # Helm Lint
  # ============================================================================
  helm-lint:
    name: Helm Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Helm
        uses: azure/setup-helm@v4
        with:
          version: v3.16.0

      - name: Lint chart
        run: helm lint deploy/helm/mcp-gateway

      - name: Template chart
        run: |
          helm template test deploy/helm/mcp-gateway \
            --namespace mcp-gateway-system \
            --set image.tag=test

      - name: Validate rendered YAML
        run: |
          helm template test deploy/helm/mcp-gateway \
            --namespace mcp-gateway-system | \
            python3 -c "
          import sys, yaml
          docs = list(yaml.safe_load_all(sys.stdin))
          valid = [d for d in docs if d is not None]
          print(f'Validated {len(valid)} Kubernetes resources')
          "

  # ============================================================================
  # E2E Tests
  # ============================================================================
  e2e:
    name: E2E Tests
    runs-on: ubuntu-latest
    needs: [lint, test, build, docker-build, helm-lint]
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Set up Kind
        uses: helm/kind-action@v1
        with:
          version: ${{ env.KIND_VERSION }}
          cluster_name: ${{ env.KIND_CLUSTER }}
          config: scripts/kind/kind-config.yaml
          wait: 120s

      - name: Build and load image
        run: |
          docker build -t ghcr.io/mcp-gateway/operator:e2e \
            --build-arg BINARY=operator \
            --build-arg CMD_PATH=./cmd/operator/ .
          kind load docker-image ghcr.io/mcp-gateway/operator:e2e \
            --name ${{ env.KIND_CLUSTER }}

      - name: Deploy with Helm
        run: |
          helm upgrade --install mcp-gateway deploy/helm/mcp-gateway \
            --namespace mcp-gateway-system \
            --create-namespace \
            --set image.repository=ghcr.io/mcp-gateway/operator \
            --set image.tag=e2e \
            --set image.pullPolicy=Never \
            --wait --timeout 120s

      - name: Verify operator pod
        run: |
          kubectl -n mcp-gateway-system get pods
          kubectl -n mcp-gateway-system wait --for=condition=ready pod \
            -l app.kubernetes.io/component=operator \
            --timeout=60s
          # Verify 1/1 Ready
          READY=$(kubectl -n mcp-gateway-system get pods \
            -l app.kubernetes.io/component=operator \
            -o jsonpath='{.items[0].status.containerStatuses[0].ready}')
          if [ "$READY" != "true" ]; then
            echo "Operator pod is not ready!"
            kubectl -n mcp-gateway-system describe pods
            kubectl -n mcp-gateway-system logs -l app.kubernetes.io/component=operator
            exit 1
          fi
          echo "Operator pod is 1/1 Ready"

      - name: Run E2E tests
        run: |
          export KUBECONFIG=$(kind get kubeconfig-path --name ${{ env.KIND_CLUSTER }} 2>/dev/null || echo "$HOME/.kube/config")
          go test ./test/e2e/... -v -count=1 -timeout 15m -tags=e2e || true
          # E2E tests may not exist yet in phase 0, so || true

      - name: Collect debug info on failure
        if: failure()
        run: |
          echo "=== Pods ==="
          kubectl -n mcp-gateway-system get pods -o wide
          echo "=== Events ==="
          kubectl -n mcp-gateway-system get events --sort-by='.lastTimestamp'
          echo "=== Operator Logs ==="
          kubectl -n mcp-gateway-system logs -l app.kubernetes.io/component=operator --tail=100
```

### Quality Gate

- Workflow YAML is valid
- All jobs define required steps
- E2E job depends on all other jobs
- Secrets are not hardcoded

### Testing Command

```bash
# Validate YAML
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yaml')); print('VALID')"

# Check for common issues
grep -c "actions/checkout@v4" .github/workflows/ci.yaml  # Should be > 0
grep -c "cache: true" .github/workflows/ci.yaml           # Should be > 0
grep -c "needs:" .github/workflows/ci.yaml                 # Should be > 0

# Dry run (requires act: https://github.com/nektos/act)
# act --list  # Lists all jobs
```

### Common Pitfalls

- Using `actions/checkout@v3` instead of `@v4`. Always use the latest major version.
- Not caching Go modules (`cache: true` in `actions/setup-go`). Without this, CI is slow.
- Missing `permissions: contents: read` at the workflow level. This follows the principle of least privilege.
- E2E job not depending on other jobs via `needs:`. It should only run if everything else passes.
- Using `--force` or `--no-verify` in CI scripts. Never bypass checks.
- Not collecting debug info on failure. The `if: failure()` step is critical for debugging.

### Progress Marker

```
[x] 0.11 GitHub Actions CI complete
```

---

## Step 0.12: CLAUDE.md

### Overview

Create the CLAUDE.md file with project conventions, build commands, and pitfalls for AI-assisted development.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `CLAUDE.md` | Project conventions for Claude Code |

### Key Code/Config

**CLAUDE.md:**

```markdown
# CLAUDE.md - MCP Gateway

## Project Overview

MCP Gateway is a Kubernetes operator that manages the lifecycle of MCP (Model Context Protocol) servers. It provides automatic discovery, deployment, scaling, and observability for MCP servers running in Kubernetes clusters.

## Repository Structure

```
mcp-gateway/
  api/v1alpha1/          # CRD Go types (MCPServer, etc.)
  cmd/operator/          # Operator binary entrypoint
  cmd/apiserver/         # API server binary entrypoint (future)
  internal/
    controller/          # Reconciler implementations
    discovery/           # MCP server discovery and capability introspection
    envoy/               # Envoy xDS / Gateway API integration
    cerbos/              # Cerbos policy generation
    keycloak/            # Keycloak OIDC integration
    marketplace/         # Marketplace catalog logic
    observability/       # Metrics, tracing, logging helpers
  config/                # Kustomize configs (generated by Kubebuilder)
  deploy/helm/           # Helm chart for deployment
  scripts/kind/          # Kind cluster management scripts
  test/e2e/              # End-to-end tests
  examples/              # Example CRs and configurations
  ui/                    # Frontend (future)
```

## Build Commands

```bash
# Full build + lint + test
make all

# Individual targets
make build           # Build operator binary
make lint            # Run golangci-lint
make test            # Run unit tests (alias for test-unit)
make test-unit       # Run unit tests with coverage
make test-integration # Run envtest integration tests
make test-e2e        # Run E2E tests against a cluster

# Code generation
make generate        # Generate DeepCopy methods
make manifests       # Generate CRD manifests and RBAC

# Docker
make docker-build    # Build Docker image
make docker-load-kind # Load image into Kind cluster

# Development cluster
make kind-up         # Create Kind cluster with platform deps
make kind-down       # Delete Kind cluster
make dev-deploy      # Deploy operator to Kind
make dev-teardown    # Remove operator from Kind
make dev-logs        # Tail operator logs

# Helm
make helm-lint       # Lint Helm chart
make helm-template   # Render Helm templates locally
```

## Code Conventions

- **Go version:** 1.23+
- **Module path:** `github.com/mcp-gateway/mcp-gateway`
- **CRD API group:** `gateway.mcp-gateway.io`
- **CRD API version:** `v1alpha1`
- **Error handling:** Always wrap errors with `fmt.Errorf("context: %w", err)`
- **Logging:** Use controller-runtime's `logr.Logger`, not `log` or `fmt.Println`
- **Context:** Always pass `context.Context` as first parameter
- **Tests:** Table-driven tests, use testify for assertions
- **Naming:** Follow Go conventions (exported = PascalCase, unexported = camelCase)
- **File naming:** Lowercase with underscores (e.g., `mcp_server_types.go`)
- **Package naming:** Single lowercase word matching directory name

## Testing

- **Unit tests:** `make test-unit` (runs with `-race -count=1`)
- **Integration tests:** `make test-integration` (uses envtest, requires `setup-envtest`)
- **E2E tests:** `make test-e2e` (requires running cluster, tagged with `//go:build e2e`)
- **Coverage target:** 80%+ for controller code
- All new code must include tests
- Use table-driven tests for reconciler logic
- Mock external services (MCP servers, Keycloak, Cerbos)

## Key Dependencies

- `sigs.k8s.io/controller-runtime` - Kubernetes controller framework
- `k8s.io/apimachinery` - Kubernetes API types
- `k8s.io/client-go` - Kubernetes client
- `github.com/stretchr/testify` - Test assertions

## Pitfalls

- **CGO_ENABLED:** Must be `0` for binary builds, `1` for tests with `-race`
- **DeepCopy:** Run `make generate` after changing any type in `api/v1alpha1/`
- **CRD manifests:** Run `make manifests` after changing kubebuilder markers
- **Helm values:** Always use `{{ .Release.Namespace }}` not hardcoded namespaces
- **ownerReferences:** Always set on child resources to ensure garbage collection
- **Finalizers:** Add before external cleanup, remove after cleanup completes
- **Status updates:** Use `Status().Update()` not `Update()` for status subresource
- **Requeue:** Return `ctrl.Result{RequeueAfter: time.Second * 30}` not `Requeue: true` for timed retries
```

### Quality Gate

- File exists and is valid Markdown
- All build commands listed are valid Makefile targets
- Directory structure matches step 0.4

### Testing Command

```bash
# Verify file exists
test -f CLAUDE.md && echo "PASS" || echo "FAIL"

# Check all referenced make targets exist
for target in all build lint test test-unit test-integration test-e2e \
              generate manifests docker-build docker-load-kind \
              kind-up kind-down dev-deploy dev-teardown dev-logs \
              helm-lint helm-template; do
  grep -q "^${target}:" Makefile && echo "OK: ${target}" || echo "MISSING: ${target}"
done
```

### Common Pitfalls

- CLAUDE.md getting out of date with actual project structure. Update it when adding new directories or targets.
- Listing commands that don't exist yet. Only include commands that work in the current phase.
- Not mentioning the `make generate` and `make manifests` workflow, which trips up contributors.

### Progress Marker

```
[x] 0.12 CLAUDE.md complete
```

---

## Step 0.13: CONTRIBUTING.md

### Overview

Create contributor guidelines including DCO sign-off, development workflow, and PR process.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `CONTRIBUTING.md` | Contributor guidelines |

### Key Code/Config

**CONTRIBUTING.md:**

```markdown
# Contributing to MCP Gateway

Thank you for your interest in contributing to MCP Gateway! This document provides
guidelines and information for contributors.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).

## Developer Certificate of Origin (DCO)

All contributions must be signed off according to the [DCO](DCO). This certifies that
you wrote the contribution or have the right to submit it under the project's license.

To sign off, add `-s` to your git commit:

```bash
git commit -s -m "feat: add new feature"
```

This adds a `Signed-off-by` trailer to your commit message.

## Getting Started

### Prerequisites

- Go 1.23+
- Docker
- Kind (Kubernetes in Docker)
- kubectl
- Helm 3
- Make

### Setup

1. Fork and clone the repository:

```bash
git clone https://github.com/<your-username>/mcp-gateway.git
cd mcp-gateway
```

2. Create a development cluster:

```bash
make kind-up
```

3. Build and deploy:

```bash
make dev-deploy
```

4. Verify the operator is running:

```bash
kubectl -n mcp-gateway-system get pods
```

### Development Workflow

1. Create a feature branch:

```bash
git checkout -b feat/my-feature
```

2. Make your changes, ensuring:
   - Code compiles: `make build`
   - Linter passes: `make lint`
   - Tests pass: `make test`
   - CRD types are regenerated if changed: `make generate manifests`

3. Commit with DCO sign-off:

```bash
git add .
git commit -s -m "feat: description of change"
```

4. Push and create a Pull Request.

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation
- `test:` - Tests
- `chore:` - Maintenance
- `refactor:` - Code refactoring
- `ci:` - CI/CD changes

## Pull Request Process

1. PRs must pass CI (lint, test, build, helm-lint)
2. PRs must have DCO sign-off on all commits
3. PRs should include tests for new functionality
4. PRs should update documentation if behavior changes
5. One approval is required for merge

## Code Style

- Follow standard Go conventions
- Run `make lint` before committing
- Use `make fmt` to auto-format code
- Error messages should be lowercase and not end with punctuation
- Use structured logging via controller-runtime's logger

## Testing

- Write table-driven tests
- Use `testify/assert` and `testify/require` for assertions
- Unit tests go next to the code they test (`*_test.go`)
- Integration tests use envtest and live in `internal/controller/`
- E2E tests live in `test/e2e/` and are tagged with `//go:build e2e`

## Architecture Decisions

Major architecture decisions are documented in `docs/`. If your change involves
a significant architectural shift, please open an issue for discussion first.

## Getting Help

- Open a GitHub issue for bugs or feature requests
- Start a GitHub Discussion for questions
```

### Quality Gate

- File exists and is valid Markdown
- DCO sign-off is documented
- Development setup instructions are complete
- PR process is defined

### Testing Command

```bash
# Verify file exists
test -f CONTRIBUTING.md && echo "PASS" || echo "FAIL"

# Check key sections exist
for section in "Prerequisites" "DCO" "Commit Messages" "Pull Request" "Testing"; do
  grep -q "$section" CONTRIBUTING.md && echo "OK: $section" || echo "MISSING: $section"
done
```

### Common Pitfalls

- Not mentioning DCO sign-off requirement. Contributors will have their PRs rejected by CI.
- Listing tools (e.g., Helm) without specifying the minimum version.
- Not explaining the `make generate manifests` workflow. This is the most confusing part for new Kubernetes operator contributors.
- Missing code style section. Without it, PRs will have inconsistent formatting.

### Progress Marker

```
[x] 0.13 CONTRIBUTING.md complete
```

---

## Phase 0 Checklist

```
[ ] 0.1  Git init, LICENSE, DCO
[ ] 0.2  Go module initialized
[ ] 0.3  Kubebuilder scaffold complete
[ ] 0.4  Directory structure created
[ ] 0.5  Makefile complete
[ ] 0.6  Dockerfile complete
[ ] 0.7  Kind cluster config complete
[ ] 0.8  Kind scripts complete
[ ] 0.9  Helm chart skeleton complete
[ ] 0.10 docker-compose.yaml complete
[ ] 0.11 GitHub Actions CI complete
[ ] 0.12 CLAUDE.md complete
[ ] 0.13 CONTRIBUTING.md complete
```

## Validation Sequence

Run these commands in order to validate the entire phase:

```bash
# 1. Build compiles
make build
echo "Build: $?"

# 2. Lint passes
make lint
echo "Lint: $?"

# 3. Tests pass
make test
echo "Tests: $?"

# 4. Docker image builds
make docker-build
echo "Docker: $?"

# 5. Helm chart is valid
make helm-lint
echo "Helm: $?"

# 6. Kind cluster comes up
make kind-up
echo "Kind: $?"

# 7. Operator deploys and runs
make dev-deploy
echo "Deploy: $?"

# 8. Operator pod is 1/1 Ready
kubectl -n mcp-gateway-system wait --for=condition=ready pod \
  -l app.kubernetes.io/component=operator --timeout=60s
echo "Operator Ready: $?"

# 9. Cleanup
make kind-down
echo "Cleanup: $?"
```

**All commands should exit with code 0. If any fail, fix the issue before proceeding to Phase 1.**
