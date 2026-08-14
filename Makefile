## ---------------------------------------------------------------------
## MCP Gateway Operator – development & CI Makefile
## ---------------------------------------------------------------------

# Image
IMG ?= ghcr.io/mcp-gateway/mcp-gateway-operator:dev

# Kind / Helm
KIND_CLUSTER_NAME ?= mcp-gateway
HELM_RELEASE     ?= mcp-gateway
HELM_NAMESPACE   ?= mcp-system
HELM_CHART       ?= deploy/helm/mcp-gateway

# Local tooling
LOCALBIN     ?= $(shell pwd)/bin
CONTROLLER_GEN   ?= $(LOCALBIN)/controller-gen
KUSTOMIZE        ?= $(LOCALBIN)/kustomize
ENVTEST          ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT    ?= $(LOCALBIN)/golangci-lint

# Pinned versions
CONTROLLER_TOOLS_VERSION ?= v0.16.5
KUSTOMIZE_VERSION        ?= v5.5.0
ENVTEST_VERSION          ?= release-0.19
GOLANGCI_LINT_VERSION    ?= v1.62.2

# Go
GOBIN = $(LOCALBIN)

## ---------------------------------------------------------------------
## General
## ---------------------------------------------------------------------

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Development

.PHONY: generate
generate: controller-gen ## Generate deepcopy / runtime.Object implementations
	$(CONTROLLER_GEN) object paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD, RBAC, and webhook manifests
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

##@ Testing

.PHONY: test
test: manifests generate fmt vet envtest ## Run all tests (with envtest)
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: test-unit
test-unit: ## Run unit tests (excludes e2e and controller packages)
	go test $$(go list ./... | grep -v /e2e | grep -v /controller) -coverprofile cover-unit.out

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	go test ./test/e2e/... -tags=e2e -count=1 -v

##@ Build

.PHONY: build
build: ## Build the operator binary
	go build -trimpath -ldflags="-s -w" -o bin/manager ./cmd/operator/

.PHONY: run
run: ## Run the operator from source
	go run ./cmd/operator/

.PHONY: docker-build
docker-build: ## Build the Docker image
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push the Docker image
	docker push $(IMG)

##@ Kind cluster

.PHONY: kind-up
kind-up: ## Create a Kind cluster
	./scripts/kind/create-cluster.sh

.PHONY: kind-down
kind-down: ## Delete the Kind cluster
	kind delete cluster --name $(KIND_CLUSTER_NAME)

.PHONY: kind-load
kind-load: ## Load the Docker image into the Kind cluster
	kind load docker-image $(IMG) --name $(KIND_CLUSTER_NAME)

##@ Deployment

.PHONY: dev-deploy
dev-deploy: manifests kind-load ## Deploy the full platform into Kind
	./scripts/kind/deploy-platform.sh

.PHONY: dev-teardown
dev-teardown: ## Remove the Helm release from the Kind cluster
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)

.PHONY: install-crds
install-crds: manifests kustomize ## Install CRDs into the cluster
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall-crds
uninstall-crds: manifests kustomize ## Uninstall CRDs from the cluster
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found -f -

## ---------------------------------------------------------------------
## Tool dependencies
## ---------------------------------------------------------------------

##@ Tools

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# go-install-tool will 'go install' a Go tool into $(LOCALBIN).
# $1 - target path
# $2 - go install path
# $3 - version
define go-install-tool
@[ -f $(1) ] || { \
	set -e; \
	echo "Installing $(2)@$(3)"; \
	GOBIN=$(LOCALBIN) go install $(2)@$(3); \
}
endef

.PHONY: controller-gen
controller-gen: $(LOCALBIN) ## Install controller-gen
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: kustomize
kustomize: $(LOCALBIN) ## Install kustomize
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: envtest
envtest: $(LOCALBIN) ## Install setup-envtest
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(LOCALBIN) ## Install golangci-lint
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
