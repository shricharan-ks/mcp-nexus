# Phase 5: Marketplace (Weeks 17-20)

Build a curated marketplace for discovering, validating, and deploying MCP servers with one click. This includes a CRD for catalog entries, a gRPC indexing service with full-text search, a security scanning pipeline, and a streamlined deploy flow.

---

## Step 1: MCPMarketplaceEntry CRD

Define the CustomResourceDefinition for marketplace entries. Each entry describes an MCP server that can be installed from the marketplace.

### Files

```
api/v1alpha1/mcpmarketplaceentry_types.go
api/v1alpha1/zz_generated.deepcopy.go          (auto-generated)
config/crd/bases/mcp.gateway.io_mcpmarketplaceentries.yaml  (auto-generated)
internal/controller/mcpmarketplaceentry_controller.go
```

### Key Code

**api/v1alpha1/mcpmarketplaceentry_types.go**

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPMarketplaceEntrySpec defines a catalog entry for an MCP server.
type MCPMarketplaceEntrySpec struct {
	// DisplayName is the human-readable name shown in the marketplace UI.
	// +kubebuilder:validation:MinLength=3
	// +kubebuilder:validation:MaxLength=64
	DisplayName string `json:"displayName"`

	// Vendor is the organization or individual publishing this entry.
	// +kubebuilder:validation:MinLength=1
	Vendor string `json:"vendor"`

	// Version is the semantic version of this marketplace entry.
	// +kubebuilder:validation:Pattern=`^v\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`
	Version string `json:"version"`

	// Category classifies the MCP server for filtering.
	// +kubebuilder:validation:Enum=ai-ml;data;developer-tools;communication;security;monitoring;infrastructure;custom
	Category string `json:"category"`

	// Description provides a short summary of the MCP server.
	// +kubebuilder:validation:MaxLength=512
	// +optional
	Description string `json:"description,omitempty"`

	// Icon is a URL to the entry's icon image (PNG or SVG, max 256x256).
	// +optional
	Icon string `json:"icon,omitempty"`

	// Source describes where the MCP server image comes from.
	Source MCPMarketplaceSource `json:"source"`

	// InstallTemplate is the template used to create MCPServer and related
	// resources when this entry is deployed.
	InstallTemplate MCPInstallTemplate `json:"installTemplate"`

	// Security holds the security verification status and metadata.
	Security MCPMarketplaceSecurity `json:"security"`

	// Verified indicates whether this entry has passed the marketplace
	// review process and security scanning.
	// +kubebuilder:default=false
	Verified bool `json:"verified"`

	// Tags are free-form labels for search and discovery.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Documentation is a URL to the server's documentation.
	// +optional
	Documentation string `json:"documentation,omitempty"`

	// SupportURL is a URL where users can get support.
	// +optional
	SupportURL string `json:"supportURL,omitempty"`
}

// MCPMarketplaceSource describes the container image source.
type MCPMarketplaceSource struct {
	// Image is the container image reference (registry/repo:tag).
	Image string `json:"image"`

	// SourceRepo is the URL of the source code repository.
	// +optional
	SourceRepo string `json:"sourceRepo,omitempty"`

	// Digest is the image digest for pinning.
	// +optional
	Digest string `json:"digest,omitempty"`
}

// MCPInstallTemplate defines the resources created on deploy.
type MCPInstallTemplate struct {
	// MCPServerSpec is the template for the MCPServer resource.
	MCPServerSpec MCPServerSpec `json:"mcpServerSpec"`

	// RequiredSecrets lists secrets the user must provide at deploy time.
	// Each entry has a name and description.
	// +optional
	RequiredSecrets []RequiredSecret `json:"requiredSecrets,omitempty"`

	// DefaultPolicy is an optional MCPPolicy to create alongside the server.
	// +optional
	DefaultPolicy *MCPPolicySpec `json:"defaultPolicy,omitempty"`
}

// RequiredSecret describes a secret needed by the MCP server.
type RequiredSecret struct {
	// Name is the secret name referenced in the MCPServer spec.
	Name string `json:"name"`

	// Description explains what this secret is used for.
	Description string `json:"description"`

	// Keys lists the required data keys in the secret.
	Keys []string `json:"keys"`
}

// MCPMarketplaceSecurity holds security scanning results.
type MCPMarketplaceSecurity struct {
	// LastScanned is the timestamp of the last security scan.
	// +optional
	LastScanned *metav1.Time `json:"lastScanned,omitempty"`

	// VulnerabilityCount is the number of known vulnerabilities.
	// +optional
	VulnerabilityCount *int32 `json:"vulnerabilityCount,omitempty"`

	// SBOMRef is a reference to the SBOM artifact.
	// +optional
	SBOMRef string `json:"sbomRef,omitempty"`

	// SignatureRef is a cosign signature reference.
	// +optional
	SignatureRef string `json:"signatureRef,omitempty"`

	// ScanStatus indicates the result of the last scan.
	// +kubebuilder:validation:Enum=passed;failed;pending;not-scanned
	// +kubebuilder:default=not-scanned
	ScanStatus string `json:"scanStatus"`
}

// MCPMarketplaceEntryStatus defines the observed state.
type MCPMarketplaceEntryStatus struct {
	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// InstallCount tracks how many times this entry has been deployed.
	InstallCount int64 `json:"installCount,omitempty"`

	// Phase represents the current lifecycle phase.
	// +kubebuilder:validation:Enum=Pending;Published;Deprecated;Removed
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcpme
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Vendor",type=string,JSONPath=`.spec.vendor`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Verified",type=boolean,JSONPath=`.spec.verified`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
type MCPMarketplaceEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPMarketplaceEntrySpec   `json:"spec,omitempty"`
	Status MCPMarketplaceEntryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type MCPMarketplaceEntryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPMarketplaceEntry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPMarketplaceEntry{}, &MCPMarketplaceEntryList{})
}
```

### Quality Gate

- `make generate` produces CRD YAML without errors.
- `make manifests` generates the CRD with all print columns.
- `kubectl apply -f config/crd/bases/mcp.gateway.io_mcpmarketplaceentries.yaml` succeeds.
- A sample CR validates and is accepted by the API server.

### Testing Command

```bash
# Generate and validate CRD
make generate manifests

# Apply CRD to cluster
kubectl apply -f config/crd/bases/mcp.gateway.io_mcpmarketplaceentries.yaml

# Create a test entry
kubectl apply -f - <<EOF
apiVersion: mcp.gateway.io/v1alpha1
kind: MCPMarketplaceEntry
metadata:
  name: test-entry
spec:
  displayName: "Test MCP Server"
  vendor: "test-vendor"
  version: "v1.0.0"
  category: developer-tools
  source:
    image: "ghcr.io/test/mcp-test:v1.0.0"
  installTemplate:
    mcpServerSpec:
      transport: sse
      url: "http://mcp-test:8080"
  security:
    scanStatus: not-scanned
  verified: false
EOF

# Verify
kubectl get mcpme
```

### Pitfalls

- **Deep copy generation:** Run `make generate` after every change to types.go. Missing deep copy methods cause runtime panics when the controller copies resources.
- **Enum validation strictness:** The `+kubebuilder:validation:Enum` marker rejects values not in the list. Plan the category list carefully because adding categories later is a CRD schema change.
- **Version field semantics:** The `version` field is for the marketplace entry itself, not the MCP server's version. Document this distinction clearly.

### Progress Marker

- [ ] Types defined with all kubebuilder markers
- [ ] CRD YAML generated successfully
- [ ] CRD applies to cluster without errors
- [ ] Sample CR passes validation
- [ ] Print columns display correctly in `kubectl get`

---

## Step 2: Catalog YAML Schema

Define a JSON Schema for the marketplace catalog file format, provide ten example entries, and create a validation script.

### Files

```
marketplace/schema/catalog-schema.json
marketplace/catalog.yaml
marketplace/scripts/validate-catalog.sh
```

### Key Code

**marketplace/schema/catalog-schema.json**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://mcp-gateway.io/schemas/catalog.json",
  "title": "MCP Marketplace Catalog",
  "description": "Schema for the MCP marketplace catalog file",
  "type": "object",
  "required": ["apiVersion", "entries"],
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "marketplace.mcp.gateway.io/v1alpha1"
    },
    "entries": {
      "type": "array",
      "items": { "$ref": "#/$defs/CatalogEntry" },
      "minItems": 1
    }
  },
  "$defs": {
    "CatalogEntry": {
      "type": "object",
      "required": ["name", "displayName", "vendor", "version", "category", "source", "installTemplate"],
      "properties": {
        "name": {
          "type": "string",
          "pattern": "^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$",
          "description": "DNS-compatible unique identifier"
        },
        "displayName": {
          "type": "string",
          "minLength": 3,
          "maxLength": 64
        },
        "vendor": {
          "type": "string",
          "minLength": 1
        },
        "version": {
          "type": "string",
          "pattern": "^v\\d+\\.\\d+\\.\\d+(-[a-zA-Z0-9.]+)?$"
        },
        "category": {
          "type": "string",
          "enum": ["ai-ml", "data", "developer-tools", "communication", "security", "monitoring", "infrastructure", "custom"]
        },
        "description": {
          "type": "string",
          "maxLength": 512
        },
        "icon": {
          "type": "string",
          "format": "uri"
        },
        "tags": {
          "type": "array",
          "items": { "type": "string" },
          "maxItems": 10
        },
        "source": { "$ref": "#/$defs/Source" },
        "installTemplate": { "$ref": "#/$defs/InstallTemplate" },
        "security": { "$ref": "#/$defs/Security" },
        "verified": {
          "type": "boolean",
          "default": false
        },
        "documentation": {
          "type": "string",
          "format": "uri"
        }
      }
    },
    "Source": {
      "type": "object",
      "required": ["image"],
      "properties": {
        "image": { "type": "string" },
        "sourceRepo": { "type": "string", "format": "uri" },
        "digest": { "type": "string", "pattern": "^sha256:[a-f0-9]{64}$" }
      }
    },
    "InstallTemplate": {
      "type": "object",
      "required": ["mcpServerSpec"],
      "properties": {
        "mcpServerSpec": {
          "type": "object",
          "required": ["transport"],
          "properties": {
            "transport": { "type": "string", "enum": ["sse", "streamable-http", "stdio"] },
            "url": { "type": "string" },
            "command": { "type": "array", "items": { "type": "string" } },
            "args": { "type": "array", "items": { "type": "string" } }
          }
        },
        "requiredSecrets": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name", "description", "keys"],
            "properties": {
              "name": { "type": "string" },
              "description": { "type": "string" },
              "keys": { "type": "array", "items": { "type": "string" }, "minItems": 1 }
            }
          }
        }
      }
    },
    "Security": {
      "type": "object",
      "properties": {
        "scanStatus": { "type": "string", "enum": ["passed", "failed", "pending", "not-scanned"] },
        "sbomRef": { "type": "string" },
        "signatureRef": { "type": "string" }
      }
    }
  }
}
```

**marketplace/catalog.yaml** (10 example entries)

```yaml
apiVersion: marketplace.mcp.gateway.io/v1alpha1
entries:
  - name: github-mcp-server
    displayName: "GitHub MCP Server"
    vendor: "GitHub"
    version: "v1.2.0"
    category: developer-tools
    description: "Access GitHub repositories, issues, PRs, and actions through MCP."
    tags: ["git", "ci-cd", "version-control"]
    source:
      image: "ghcr.io/github/mcp-server:v1.2.0"
      sourceRepo: "https://github.com/github/mcp-server"
      digest: "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://github-mcp:8080/sse"
      requiredSecrets:
        - name: github-token
          description: "GitHub personal access token with repo scope"
          keys: ["GITHUB_TOKEN"]
    security:
      scanStatus: passed
    verified: true
    documentation: "https://github.com/github/mcp-server/blob/main/README.md"

  - name: slack-mcp-server
    displayName: "Slack MCP Server"
    vendor: "Slack Technologies"
    version: "v1.0.0"
    category: communication
    description: "Send messages, manage channels, and search Slack workspaces via MCP."
    tags: ["messaging", "chat", "collaboration"]
    source:
      image: "ghcr.io/slack/mcp-server:v1.0.0"
      sourceRepo: "https://github.com/slackapi/mcp-server"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://slack-mcp:8080/sse"
      requiredSecrets:
        - name: slack-credentials
          description: "Slack Bot OAuth token"
          keys: ["SLACK_BOT_TOKEN", "SLACK_SIGNING_SECRET"]
    security:
      scanStatus: passed
    verified: true

  - name: postgres-mcp-server
    displayName: "PostgreSQL MCP Server"
    vendor: "MCP Community"
    version: "v0.9.0"
    category: data
    description: "Query and manage PostgreSQL databases through MCP tools."
    tags: ["database", "sql", "postgresql"]
    source:
      image: "ghcr.io/mcp-community/postgres-mcp:v0.9.0"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://postgres-mcp:8080/sse"
      requiredSecrets:
        - name: postgres-credentials
          description: "PostgreSQL connection credentials"
          keys: ["POSTGRES_HOST", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"]
    security:
      scanStatus: passed
    verified: true

  - name: openai-mcp-server
    displayName: "OpenAI MCP Server"
    vendor: "MCP Community"
    version: "v1.1.0"
    category: ai-ml
    description: "Access OpenAI models (GPT, DALL-E, Whisper) through MCP."
    tags: ["llm", "gpt", "ai", "embeddings"]
    source:
      image: "ghcr.io/mcp-community/openai-mcp:v1.1.0"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://openai-mcp:8080/sse"
      requiredSecrets:
        - name: openai-api-key
          description: "OpenAI API key"
          keys: ["OPENAI_API_KEY"]
    security:
      scanStatus: passed
    verified: true

  - name: prometheus-mcp-server
    displayName: "Prometheus MCP Server"
    vendor: "MCP Community"
    version: "v0.5.0"
    category: monitoring
    description: "Query Prometheus metrics and alerts via MCP tools."
    tags: ["metrics", "alerting", "observability"]
    source:
      image: "ghcr.io/mcp-community/prometheus-mcp:v0.5.0"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://prometheus-mcp:8080/sse"
    security:
      scanStatus: pending
    verified: false

  - name: vault-mcp-server
    displayName: "HashiCorp Vault MCP Server"
    vendor: "HashiCorp"
    version: "v1.0.0"
    category: security
    description: "Manage secrets and encryption through HashiCorp Vault via MCP."
    tags: ["secrets", "encryption", "pki"]
    source:
      image: "ghcr.io/hashicorp/vault-mcp:v1.0.0"
      sourceRepo: "https://github.com/hashicorp/vault-mcp"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://vault-mcp:8200/sse"
      requiredSecrets:
        - name: vault-credentials
          description: "Vault token or AppRole credentials"
          keys: ["VAULT_TOKEN"]
    security:
      scanStatus: passed
    verified: true

  - name: kubernetes-mcp-server
    displayName: "Kubernetes MCP Server"
    vendor: "MCP Community"
    version: "v0.8.0"
    category: infrastructure
    description: "Manage Kubernetes resources (pods, deployments, services) via MCP."
    tags: ["k8s", "containers", "orchestration"]
    source:
      image: "ghcr.io/mcp-community/k8s-mcp:v0.8.0"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://k8s-mcp:8080/sse"
    security:
      scanStatus: passed
    verified: false

  - name: jira-mcp-server
    displayName: "Jira MCP Server"
    vendor: "Atlassian"
    version: "v1.0.0"
    category: developer-tools
    description: "Create and manage Jira issues, sprints, and boards through MCP."
    tags: ["project-management", "agile", "issues"]
    source:
      image: "ghcr.io/atlassian/jira-mcp:v1.0.0"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://jira-mcp:8080/sse"
      requiredSecrets:
        - name: jira-credentials
          description: "Jira API token and instance URL"
          keys: ["JIRA_URL", "JIRA_EMAIL", "JIRA_API_TOKEN"]
    security:
      scanStatus: passed
    verified: true

  - name: s3-mcp-server
    displayName: "AWS S3 MCP Server"
    vendor: "MCP Community"
    version: "v0.7.0"
    category: data
    description: "Read, write, and manage objects in AWS S3 buckets through MCP."
    tags: ["aws", "storage", "cloud"]
    source:
      image: "ghcr.io/mcp-community/s3-mcp:v0.7.0"
    installTemplate:
      mcpServerSpec:
        transport: sse
        url: "http://s3-mcp:8080/sse"
      requiredSecrets:
        - name: aws-credentials
          description: "AWS access key ID and secret access key"
          keys: ["AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"]
    security:
      scanStatus: not-scanned
    verified: false

  - name: custom-rag-mcp-server
    displayName: "Custom RAG Pipeline"
    vendor: "Internal"
    version: "v0.1.0"
    category: custom
    description: "Example custom MCP server template for RAG pipelines."
    tags: ["rag", "embeddings", "vector-search"]
    source:
      image: "internal-registry.example.com/rag-mcp:v0.1.0"
    installTemplate:
      mcpServerSpec:
        transport: streamable-http
        url: "http://rag-mcp:8080/mcp"
      requiredSecrets:
        - name: rag-config
          description: "Vector database connection and embedding model API key"
          keys: ["VECTOR_DB_URL", "EMBEDDING_API_KEY"]
    security:
      scanStatus: not-scanned
    verified: false
```

**marketplace/scripts/validate-catalog.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMA_FILE="${SCRIPT_DIR}/../schema/catalog-schema.json"
CATALOG_FILE="${1:-${SCRIPT_DIR}/../catalog.yaml}"

# Require yq and ajv-cli
for cmd in yq ajv; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd is required but not installed."
    echo "  Install yq: brew install yq"
    echo "  Install ajv: npm install -g ajv-cli ajv-formats"
    exit 1
  fi
done

echo "Validating catalog: $CATALOG_FILE"
echo "Using schema: $SCHEMA_FILE"

# Convert YAML to JSON for validation
TEMP_JSON=$(mktemp)
trap "rm -f $TEMP_JSON" EXIT
yq -o json "$CATALOG_FILE" > "$TEMP_JSON"

# Validate against JSON Schema
ajv validate -s "$SCHEMA_FILE" -d "$TEMP_JSON" --spec=draft2020 --all-errors

# Additional checks beyond schema validation
echo ""
echo "Running additional checks..."

# Check for duplicate names
DUPLICATES=$(yq '.entries[].name' "$CATALOG_FILE" | sort | uniq -d)
if [ -n "$DUPLICATES" ]; then
  echo "ERROR: Duplicate entry names found:"
  echo "$DUPLICATES"
  exit 1
fi

# Check that verified entries have passed security scan
VERIFIED_UNSCANNED=$(yq '.entries[] | select(.verified == true and .security.scanStatus != "passed") | .name' "$CATALOG_FILE")
if [ -n "$VERIFIED_UNSCANNED" ]; then
  echo "ERROR: Verified entries without passed security scan:"
  echo "$VERIFIED_UNSCANNED"
  exit 1
fi

# Check image references are not using :latest
LATEST_IMAGES=$(yq '.entries[] | select(.source.image | test(":latest$")) | .name' "$CATALOG_FILE")
if [ -n "$LATEST_IMAGES" ]; then
  echo "WARNING: Entries using :latest image tag (not recommended):"
  echo "$LATEST_IMAGES"
fi

echo ""
echo "Catalog validation passed."
```

### Quality Gate

- `validate-catalog.sh` exits 0 with the provided 10 entries.
- The JSON Schema correctly rejects entries with invalid versions, missing required fields, or unknown categories.
- No duplicate names in the catalog.

### Testing Command

```bash
# Validate the catalog
chmod +x marketplace/scripts/validate-catalog.sh
./marketplace/scripts/validate-catalog.sh

# Test rejection of invalid entries
cat > /tmp/bad-catalog.yaml <<EOF
apiVersion: marketplace.mcp.gateway.io/v1alpha1
entries:
  - name: "BAD NAME WITH SPACES"
    version: "not-semver"
    category: "nonexistent"
EOF
./marketplace/scripts/validate-catalog.sh /tmp/bad-catalog.yaml  # should fail
```

### Pitfalls

- **YAML-to-JSON conversion edge cases:** `yq` versions differ significantly (Mike Farah's vs. Python yq). Pin to `mikefarah/yq` v4+. The `yq -o json` flag is v4-specific.
- **Schema evolution:** Adding new required fields to the schema is a breaking change for existing catalogs. Use `required` sparingly; make new fields optional with sensible defaults.
- **Image digest pinning:** The `digest` field is optional, but unverified entries without digests should trigger a warning. Digest pinning prevents supply-chain attacks via tag mutation.

### Progress Marker

- [ ] JSON Schema covers all CRD fields
- [ ] 10 example entries span all 8 categories
- [ ] Validation script passes with example catalog
- [ ] Validation script rejects malformed entries
- [ ] Script runs in CI pipeline

---

## Step 3: Catalog Indexer gRPC Service

Build a gRPC service that indexes the catalog into PostgreSQL with full-text search and serves catalog queries.

### Files

```
api/proto/marketplace/v1/marketplace.proto
internal/marketplace/indexer.go
internal/marketplace/store.go
internal/marketplace/store_test.go
cmd/marketplace-indexer/main.go
deploy/helm/templates/marketplace-indexer-deployment.yaml
deploy/helm/templates/marketplace-indexer-service.yaml
migrations/003_marketplace.sql
```

### Key Code

**api/proto/marketplace/v1/marketplace.proto**

```protobuf
syntax = "proto3";

package marketplace.v1;

option go_package = "github.com/mcp-gateway/mcp-gateway/gen/marketplace/v1;marketplacev1";

service MarketplaceService {
  // ListCatalog returns all catalog entries with pagination.
  rpc ListCatalog(ListCatalogRequest) returns (ListCatalogResponse);

  // GetCatalogEntry returns a single catalog entry by name.
  rpc GetCatalogEntry(GetCatalogEntryRequest) returns (CatalogEntry);

  // SearchCatalog performs full-text search across catalog entries.
  rpc SearchCatalog(SearchCatalogRequest) returns (SearchCatalogResponse);

  // DeployCatalogEntry triggers the deployment of a catalog entry.
  rpc DeployCatalogEntry(DeployCatalogEntryRequest) returns (DeployCatalogEntryResponse);
}

message ListCatalogRequest {
  int32 page_size = 1;
  string page_token = 2;
  string category_filter = 3;
  bool verified_only = 4;
}

message ListCatalogResponse {
  repeated CatalogEntry entries = 1;
  string next_page_token = 2;
  int32 total_count = 3;
}

message GetCatalogEntryRequest {
  string name = 1;
}

message SearchCatalogRequest {
  string query = 1;
  int32 page_size = 2;
  string page_token = 3;
  string category_filter = 4;
}

message SearchCatalogResponse {
  repeated CatalogEntry entries = 1;
  string next_page_token = 2;
  int32 total_count = 3;
}

message DeployCatalogEntryRequest {
  string entry_name = 1;
  string target_namespace = 2;
  map<string, string> secret_values = 3;
  string instance_name = 4;
}

message DeployCatalogEntryResponse {
  string mcpserver_name = 1;
  string namespace = 2;
  DeployStatus status = 3;
  string message = 4;
}

enum DeployStatus {
  DEPLOY_STATUS_UNSPECIFIED = 0;
  DEPLOY_STATUS_CREATED = 1;
  DEPLOY_STATUS_FAILED = 2;
  DEPLOY_STATUS_ALREADY_EXISTS = 3;
}

message CatalogEntry {
  string name = 1;
  string display_name = 2;
  string vendor = 3;
  string version = 4;
  string category = 5;
  string description = 6;
  string icon = 7;
  repeated string tags = 8;
  Source source = 9;
  SecurityInfo security = 10;
  bool verified = 11;
  string documentation = 12;
  int64 install_count = 13;
}

message Source {
  string image = 1;
  string source_repo = 2;
  string digest = 3;
}

message SecurityInfo {
  string scan_status = 1;
  string last_scanned = 2;
  int32 vulnerability_count = 3;
}
```

**internal/marketplace/store.go** (PostgreSQL with full-text search)

```go
package marketplace

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Search performs PostgreSQL full-text search across name, displayName,
// description, vendor, and tags.
func (s *Store) Search(ctx context.Context, query string, category string, limit, offset int) ([]CatalogRow, int, error) {
	baseQuery := `
		SELECT name, display_name, vendor, version, category, description,
		       icon, tags, image, source_repo, digest, scan_status,
		       last_scanned, vulnerability_count, verified, documentation,
		       install_count,
		       ts_rank(search_vector, websearch_to_tsquery('english', $1)) AS rank
		FROM marketplace_entries
		WHERE search_vector @@ websearch_to_tsquery('english', $1)
	`
	countQuery := `
		SELECT COUNT(*)
		FROM marketplace_entries
		WHERE search_vector @@ websearch_to_tsquery('english', $1)
	`
	args := []interface{}{query}
	argIdx := 2

	if category != "" {
		filter := fmt.Sprintf(" AND category = $%d", argIdx)
		baseQuery += filter
		countQuery += filter
		args = append(args, category)
		argIdx++
	}

	// Get total count
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting search results: %w", err)
	}

	baseQuery += fmt.Sprintf(" ORDER BY rank DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("searching catalog: %w", err)
	}
	defer rows.Close()

	var results []CatalogRow
	for rows.Next() {
		var row CatalogRow
		var rank float64
		err := rows.Scan(
			&row.Name, &row.DisplayName, &row.Vendor, &row.Version,
			&row.Category, &row.Description, &row.Icon, &row.Tags,
			&row.Image, &row.SourceRepo, &row.Digest, &row.ScanStatus,
			&row.LastScanned, &row.VulnerabilityCount, &row.Verified,
			&row.Documentation, &row.InstallCount, &rank,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning row: %w", err)
		}
		results = append(results, row)
	}

	return results, total, rows.Err()
}
```

**migrations/003_marketplace.sql**

```sql
CREATE TABLE IF NOT EXISTS marketplace_entries (
    name            VARCHAR(63) PRIMARY KEY,
    display_name    VARCHAR(64) NOT NULL,
    vendor          VARCHAR(128) NOT NULL,
    version         VARCHAR(32) NOT NULL,
    category        VARCHAR(32) NOT NULL,
    description     TEXT,
    icon            TEXT,
    tags            TEXT[],
    image           TEXT NOT NULL,
    source_repo     TEXT,
    digest          VARCHAR(128),
    scan_status     VARCHAR(16) DEFAULT 'not-scanned',
    last_scanned    TIMESTAMPTZ,
    vulnerability_count INTEGER DEFAULT 0,
    sbom_ref        TEXT,
    signature_ref   TEXT,
    verified        BOOLEAN DEFAULT FALSE,
    documentation   TEXT,
    support_url     TEXT,
    install_count   BIGINT DEFAULT 0,
    install_template JSONB NOT NULL,
    search_vector   TSVECTOR,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Full-text search index
CREATE INDEX IF NOT EXISTS idx_marketplace_search
    ON marketplace_entries USING GIN(search_vector);

-- Category filter index
CREATE INDEX IF NOT EXISTS idx_marketplace_category
    ON marketplace_entries (category);

-- Verified filter index
CREATE INDEX IF NOT EXISTS idx_marketplace_verified
    ON marketplace_entries (verified) WHERE verified = TRUE;

-- Auto-update search vector on insert/update
CREATE OR REPLACE FUNCTION marketplace_search_vector_update() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.display_name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.name, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.vendor, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.description, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'B');
    NEW.updated_at := NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER marketplace_entries_search_update
    BEFORE INSERT OR UPDATE ON marketplace_entries
    FOR EACH ROW EXECUTE FUNCTION marketplace_search_vector_update();
```

### Quality Gate

- Protobuf compiles with `buf generate`.
- PostgreSQL migration applies without errors.
- Full-text search returns relevant results (e.g., searching "database" returns PostgreSQL entry).
- gRPC health check passes.

### Testing Command

```bash
# Generate protobuf
buf generate

# Run migration
psql -h localhost -U mcpgateway -d mcpgateway -f migrations/003_marketplace.sql

# Run store tests (requires test PostgreSQL)
go test ./internal/marketplace/... -v -count=1 -tags=integration

# Test gRPC service with grpcurl
grpcurl -plaintext localhost:50051 marketplace.v1.MarketplaceService/ListCatalog

grpcurl -plaintext -d '{"query": "database"}' \
  localhost:50051 marketplace.v1.MarketplaceService/SearchCatalog
```

### Pitfalls

- **`websearch_to_tsquery` requires PostgreSQL 11+:** Use `plainto_tsquery` as a fallback for older versions, but it does not support quoted phrases or boolean operators.
- **Search vector stale after bulk import:** If entries are imported via `COPY` instead of `INSERT`, the trigger does not fire. Run `UPDATE marketplace_entries SET updated_at = NOW()` to force trigger execution.
- **gRPC reflection:** Enable server reflection in the indexer so that `grpcurl` works without specifying the proto file. Add `reflection.Register(server)` in main.go.

### Progress Marker

- [ ] Protobuf compiles and Go code generates
- [ ] PostgreSQL migration applies cleanly
- [ ] Full-text search returns ranked results
- [ ] All four RPCs functional
- [ ] gRPC health check passing

---

## Step 4: 1-Click Deploy Flow

Implement the deploy flow that creates a Secret, MCPServer, and optionally an MCPPolicy from a marketplace entry template in a single operation.

### Files

```
internal/marketplace/deployer.go
internal/marketplace/deployer_test.go
internal/marketplace/validation.go
```

### Key Code

**internal/marketplace/deployer.go**

```go
package marketplace

import (
	"context"
	"fmt"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DeployResult contains the outcome of a deploy operation.
type DeployResult struct {
	MCPServerName string
	Namespace     string
	SecretCreated bool
	PolicyCreated bool
	Error         error
}

// Deployer handles 1-click deployment of marketplace entries.
type Deployer struct {
	client client.Client
}

func NewDeployer(c client.Client) *Deployer {
	return &Deployer{client: c}
}

// Deploy creates all resources for a marketplace entry in the target namespace.
// The operation is best-effort atomic: if any resource fails, previously created
// resources are cleaned up.
func (d *Deployer) Deploy(ctx context.Context, entry *CatalogRow, req DeployRequest) DeployResult {
	logger := log.FromContext(ctx)
	result := DeployResult{
		MCPServerName: req.InstanceName,
		Namespace:     req.TargetNamespace,
	}

	var cleanups []func()
	defer func() {
		if result.Error != nil {
			logger.Error(result.Error, "deploy failed, cleaning up")
			for _, cleanup := range cleanups {
				cleanup()
			}
		}
	}()

	// Step 1: Validate required secrets are provided
	if err := validateSecretValues(entry.InstallTemplate.RequiredSecrets, req.SecretValues); err != nil {
		result.Error = fmt.Errorf("secret validation: %w", err)
		return result
	}

	// Step 2: Create Secret if required
	if len(entry.InstallTemplate.RequiredSecrets) > 0 {
		secret := buildSecret(req.InstanceName, req.TargetNamespace, req.SecretValues)
		if err := d.client.Create(ctx, secret); err != nil {
			if errors.IsAlreadyExists(err) {
				// Update existing secret
				existing := &corev1.Secret{}
				if getErr := d.client.Get(ctx, client.ObjectKeyFromObject(secret), existing); getErr != nil {
					result.Error = fmt.Errorf("getting existing secret: %w", getErr)
					return result
				}
				existing.Data = secret.Data
				if updateErr := d.client.Update(ctx, existing); updateErr != nil {
					result.Error = fmt.Errorf("updating secret: %w", updateErr)
					return result
				}
			} else {
				result.Error = fmt.Errorf("creating secret: %w", err)
				return result
			}
		}
		result.SecretCreated = true
		cleanups = append(cleanups, func() {
			d.client.Delete(ctx, secret) //nolint:errcheck
		})
	}

	// Step 3: Create MCPServer
	mcpServer := buildMCPServer(entry, req)
	if err := d.client.Create(ctx, mcpServer); err != nil {
		if errors.IsAlreadyExists(err) {
			result.Error = fmt.Errorf("MCPServer %s already exists in namespace %s", req.InstanceName, req.TargetNamespace)
		} else {
			result.Error = fmt.Errorf("creating MCPServer: %w", err)
		}
		return result
	}
	cleanups = append(cleanups, func() {
		d.client.Delete(ctx, mcpServer) //nolint:errcheck
	})

	// Step 4: Create MCPPolicy if template provides one
	if entry.InstallTemplate.DefaultPolicy != nil {
		policy := buildPolicy(entry, req)
		if err := d.client.Create(ctx, policy); err != nil && !errors.IsAlreadyExists(err) {
			result.Error = fmt.Errorf("creating MCPPolicy: %w", err)
			return result
		}
		result.PolicyCreated = true
	}

	// Success: clear cleanups
	cleanups = nil
	return result
}

func buildSecret(name, namespace string, values map[string]string) *corev1.Secret {
	data := make(map[string][]byte, len(values))
	for k, v := range values {
		data[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-secrets",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "mcp-marketplace",
				"marketplace.mcp.gateway.io/instance": name,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}

func buildMCPServer(entry *CatalogRow, req DeployRequest) *mcpv1alpha1.MCPServer {
	spec := entry.InstallTemplate.MCPServerSpec.DeepCopy()
	return &mcpv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.InstanceName,
			Namespace: req.TargetNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":         "mcp-marketplace",
				"marketplace.mcp.gateway.io/entry":     entry.Name,
				"marketplace.mcp.gateway.io/version":   entry.Version,
			},
			Annotations: map[string]string{
				"marketplace.mcp.gateway.io/deployed-at": metav1.Now().Format("2006-01-02T15:04:05Z"),
			},
		},
		Spec: *spec,
	}
}

func buildPolicy(entry *CatalogRow, req DeployRequest) *mcpv1alpha1.MCPPolicy {
	spec := entry.InstallTemplate.DefaultPolicy.DeepCopy()
	return &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.InstanceName + "-policy",
			Namespace: req.TargetNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":     "mcp-marketplace",
				"marketplace.mcp.gateway.io/entry": entry.Name,
			},
		},
		Spec: *spec,
	}
}
```

**internal/marketplace/validation.go**

```go
package marketplace

import "fmt"

// validateSecretValues checks that all required secret keys are provided.
func validateSecretValues(required []RequiredSecret, provided map[string]string) error {
	for _, req := range required {
		for _, key := range req.Keys {
			if val, ok := provided[key]; !ok || val == "" {
				return fmt.Errorf("required secret key %q (for %s) is missing or empty", key, req.Name)
			}
		}
	}
	return nil
}
```

### Quality Gate

- Deploying a catalog entry with all required secrets creates Secret + MCPServer + MCPPolicy.
- Missing secret keys return a clear validation error before any resources are created.
- Duplicate deploy returns "already exists" without creating partial resources.
- Failed MCPServer creation cleans up the previously created Secret.

### Testing Command

```bash
# Unit test deployer
go test ./internal/marketplace/... -v -run TestDeploy -count=1

# Integration test with envtest
make test-integration TEST_ARGS="-run TestMarketplaceDeploy"

# Manual verification
grpcurl -plaintext -d '{
  "entry_name": "github-mcp-server",
  "target_namespace": "default",
  "instance_name": "my-github",
  "secret_values": {"GITHUB_TOKEN": "ghp_test123"}
}' localhost:50051 marketplace.v1.MarketplaceService/DeployCatalogEntry
```

### Pitfalls

- **Secret data encoding:** Kubernetes secrets store data as base64-encoded bytes. The `corev1.Secret.Data` field expects `[]byte` values, which the API server base64-encodes automatically. Do not double-encode.
- **Cleanup race condition:** If the operator starts reconciling the MCPServer before the deploy cleanup runs on failure, orphaned child resources (Deployments, Services) may be left behind. Add a finalizer to the MCPServer before creating it, and remove it during cleanup.
- **Namespace validation:** The target namespace must exist. Validate its existence before creating resources to avoid confusing "namespace not found" errors mixed with cleanup errors.

### Progress Marker

- [ ] Secret creation from template values
- [ ] MCPServer creation from install template
- [ ] MCPPolicy creation when template includes one
- [ ] Validation rejects missing secret keys
- [ ] Cleanup on failure removes partial resources

---

## Step 5: Security Scanning Pipeline

Automate security scanning of marketplace entries with Trivy for vulnerability scanning, cosign for image signing, and Syft for SBOM generation.

### Files

```
marketplace/ci/scan-entry.yaml       (GitHub Actions workflow)
marketplace/ci/Makefile
marketplace/scripts/scan-image.sh
marketplace/scripts/generate-sbom.sh
marketplace/scripts/sign-image.sh
```

### Key Code

**marketplace/ci/scan-entry.yaml** (GitHub Actions)

```yaml
name: Marketplace Security Scan

on:
  pull_request:
    paths:
      - 'marketplace/catalog.yaml'
  workflow_dispatch:
    inputs:
      entry_name:
        description: 'Specific entry name to scan'
        required: false

jobs:
  extract-entries:
    runs-on: ubuntu-latest
    outputs:
      entries: ${{ steps.parse.outputs.entries }}
    steps:
      - uses: actions/checkout@v4
      - name: Parse changed entries
        id: parse
        run: |
          if [ -n "${{ inputs.entry_name }}" ]; then
            echo "entries=[\"${{ inputs.entry_name }}\"]" >> "$GITHUB_OUTPUT"
          else
            ENTRIES=$(yq '.entries[].name' marketplace/catalog.yaml | jq -R -s -c 'split("\n") | map(select(. != ""))')
            echo "entries=$ENTRIES" >> "$GITHUB_OUTPUT"
          fi

  scan:
    needs: extract-entries
    runs-on: ubuntu-latest
    strategy:
      matrix:
        entry: ${{ fromJson(needs.extract-entries.outputs.entries) }}
      fail-fast: false
    steps:
      - uses: actions/checkout@v4

      - name: Get image for entry
        id: image
        run: |
          IMAGE=$(yq ".entries[] | select(.name == \"${{ matrix.entry }}\") | .source.image" marketplace/catalog.yaml)
          echo "image=$IMAGE" >> "$GITHUB_OUTPUT"

      - name: Trivy vulnerability scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: ${{ steps.image.outputs.image }}
          format: 'sarif'
          output: 'trivy-${{ matrix.entry }}.sarif'
          severity: 'CRITICAL,HIGH'
          exit-code: '1'

      - name: Generate SBOM with Syft
        uses: anchore/sbom-action@v0
        with:
          image: ${{ steps.image.outputs.image }}
          artifact-name: sbom-${{ matrix.entry }}.spdx.json
          output-file: sbom-${{ matrix.entry }}.spdx.json

      - name: Upload scan results
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: 'trivy-${{ matrix.entry }}.sarif'

      - name: Update catalog scan status
        if: always()
        run: |
          STATUS="passed"
          if [ ${{ job.status }} != "success" ]; then
            STATUS="failed"
          fi
          yq -i "(.entries[] | select(.name == \"${{ matrix.entry }}\") | .security.scanStatus) = \"$STATUS\"" \
            marketplace/catalog.yaml
          yq -i "(.entries[] | select(.name == \"${{ matrix.entry }}\") | .security.lastScanned) = \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"" \
            marketplace/catalog.yaml

  sign:
    needs: scan
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    permissions:
      id-token: write
    strategy:
      matrix:
        entry: ${{ fromJson(needs.extract-entries.outputs.entries) }}
    steps:
      - uses: actions/checkout@v4
      - uses: sigstore/cosign-installer@v3

      - name: Get image for entry
        id: image
        run: |
          IMAGE=$(yq ".entries[] | select(.name == \"${{ matrix.entry }}\") | .source.image" marketplace/catalog.yaml)
          echo "image=$IMAGE" >> "$GITHUB_OUTPUT"

      - name: Sign image with cosign (keyless)
        run: cosign sign --yes ${{ steps.image.outputs.image }}
```

**marketplace/scripts/scan-image.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:?Usage: scan-image.sh <image-ref>}"

echo "=== Trivy Vulnerability Scan ==="
trivy image --severity CRITICAL,HIGH --exit-code 1 "$IMAGE"

echo ""
echo "=== Trivy Secret Scan ==="
trivy image --scanners secret --exit-code 1 "$IMAGE"

echo ""
echo "=== Trivy Misconfiguration Scan ==="
trivy image --scanners misconfig "$IMAGE"

echo ""
echo "Scan complete for: $IMAGE"
```

### Quality Gate

- CI workflow runs on every PR that modifies `catalog.yaml`.
- All verified entries pass Trivy scan with zero CRITICAL/HIGH vulnerabilities.
- SBOM is generated in SPDX format for every scanned entry.
- Cosign signatures are applied to verified images on merge to main.

### Testing Command

```bash
# Local scan of a single image
chmod +x marketplace/scripts/scan-image.sh
./marketplace/scripts/scan-image.sh ghcr.io/github/mcp-server:v1.2.0

# Generate SBOM locally
syft ghcr.io/github/mcp-server:v1.2.0 -o spdx-json > sbom.spdx.json

# Verify cosign signature
cosign verify --certificate-identity-regexp='.*' --certificate-oidc-issuer-regexp='.*' \
  ghcr.io/github/mcp-server:v1.2.0

# Run CI workflow locally with act
act pull_request -W marketplace/ci/scan-entry.yaml
```

### Pitfalls

- **Rate limiting on image pulls:** Scanning all 10+ entries in parallel can hit Docker Hub or GHCR rate limits. Use `fail-fast: false` in the matrix strategy and add retry logic.
- **Trivy DB download in CI:** The Trivy vulnerability DB is ~40MB and downloaded on every run. Cache it with `actions/cache` keyed on `trivy-db-<date>` to speed up runs.
- **Cosign keyless signing requires OIDC:** The `id-token: write` permission is needed for GitHub's OIDC identity. This only works in GitHub Actions, not in local testing. Use key-based signing for development.

### Progress Marker

- [ ] CI workflow triggers on catalog changes
- [ ] Trivy scans complete for all entries
- [ ] SBOMs generated in SPDX format
- [ ] Cosign signatures applied on merge
- [ ] Scan results update catalog.yaml scan status

---

## Step 6: Tests

Comprehensive tests for all marketplace components: CRD validation, store operations, deploy flow, and security scanning.

### Files

```
api/v1alpha1/mcpmarketplaceentry_types_test.go
internal/marketplace/store_test.go
internal/marketplace/deployer_test.go
internal/marketplace/validation_test.go
test/e2e/marketplace_test.go
```

### Key Code

**internal/marketplace/store_test.go**

```go
//go:build integration

package marketplace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Search(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	// Seed test data
	seedTestEntries(t, store)

	tests := []struct {
		name     string
		query    string
		category string
		wantMin  int
		wantName string
	}{
		{
			name:     "search by name",
			query:    "github",
			wantMin:  1,
			wantName: "github-mcp-server",
		},
		{
			name:     "search by description keyword",
			query:    "database",
			wantMin:  1,
			wantName: "postgres-mcp-server",
		},
		{
			name:     "search with category filter",
			query:    "server",
			category: "ai-ml",
			wantMin:  1,
			wantName: "openai-mcp-server",
		},
		{
			name:    "search with no results",
			query:   "nonexistent-thing-xyz",
			wantMin: 0,
		},
		{
			name:     "search by tag",
			query:    "messaging",
			wantMin:  1,
			wantName: "slack-mcp-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := store.Search(ctx, tt.query, tt.category, 10, 0)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, total, tt.wantMin)
			if tt.wantName != "" && len(results) > 0 {
				assert.Equal(t, tt.wantName, results[0].Name)
			}
		})
	}
}

func TestStore_ListWithPagination(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	seedTestEntries(t, store)

	// Page 1
	entries, total, err := store.List(ctx, "", false, 3, 0)
	require.NoError(t, err)
	assert.Equal(t, 10, total)
	assert.Len(t, entries, 3)

	// Page 2
	entries2, _, err := store.List(ctx, "", false, 3, 3)
	require.NoError(t, err)
	assert.Len(t, entries2, 3)
	assert.NotEqual(t, entries[0].Name, entries2[0].Name)
}
```

**internal/marketplace/deployer_test.go**

```go
package marketplace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeployer_Deploy(t *testing.T) {
	tests := []struct {
		name        string
		entry       CatalogRow
		request     DeployRequest
		wantErr     bool
		errContains string
	}{
		{
			name: "successful deploy with secrets",
			entry: CatalogRow{
				Name: "github-mcp-server",
				InstallTemplate: InstallTemplate{
					RequiredSecrets: []RequiredSecret{
						{Name: "github-token", Keys: []string{"GITHUB_TOKEN"}},
					},
				},
			},
			request: DeployRequest{
				InstanceName:    "my-github",
				TargetNamespace: "default",
				SecretValues:    map[string]string{"GITHUB_TOKEN": "ghp_test"},
			},
			wantErr: false,
		},
		{
			name: "missing required secret key",
			entry: CatalogRow{
				Name: "github-mcp-server",
				InstallTemplate: InstallTemplate{
					RequiredSecrets: []RequiredSecret{
						{Name: "github-token", Keys: []string{"GITHUB_TOKEN"}},
					},
				},
			},
			request: DeployRequest{
				InstanceName:    "my-github",
				TargetNamespace: "default",
				SecretValues:    map[string]string{},
			},
			wantErr:     true,
			errContains: "GITHUB_TOKEN",
		},
		{
			name: "deploy without secrets",
			entry: CatalogRow{
				Name: "prometheus-mcp-server",
			},
			request: DeployRequest{
				InstanceName:    "my-prom",
				TargetNamespace: "monitoring",
				SecretValues:    map[string]string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := setupScheme(t)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			deployer := NewDeployer(fakeClient)

			result := deployer.Deploy(context.Background(), &tt.entry, tt.request)

			if tt.wantErr {
				require.Error(t, result.Error)
				assert.Contains(t, result.Error.Error(), tt.errContains)
			} else {
				require.NoError(t, result.Error)
				assert.Equal(t, tt.request.InstanceName, result.MCPServerName)

				// Verify resources were created
				if len(tt.entry.InstallTemplate.RequiredSecrets) > 0 {
					secret := &corev1.Secret{}
					err := fakeClient.Get(context.Background(),
						client.ObjectKey{Name: tt.request.InstanceName + "-secrets", Namespace: tt.request.TargetNamespace},
						secret)
					require.NoError(t, err)
					assert.True(t, result.SecretCreated)
				}
			}
		})
	}
}
```

**test/e2e/marketplace_test.go**

```go
//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	marketplacev1 "github.com/mcp-gateway/mcp-gateway/gen/marketplace/v1"
)

func TestMarketplaceE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Connect to marketplace indexer
	conn, err := grpc.DialContext(ctx, marketplaceAddr(t),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := marketplacev1.NewMarketplaceServiceClient(conn)

	t.Run("list_catalog", func(t *testing.T) {
		resp, err := client.ListCatalog(ctx, &marketplacev1.ListCatalogRequest{PageSize: 5})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.TotalCount, int32(1))
		assert.NotEmpty(t, resp.Entries)
	})

	t.Run("search_catalog", func(t *testing.T) {
		resp, err := client.SearchCatalog(ctx, &marketplacev1.SearchCatalogRequest{
			Query:    "github",
			PageSize: 10,
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Entries), 1)
		assert.Equal(t, "github-mcp-server", resp.Entries[0].Name)
	})

	t.Run("deploy_entry", func(t *testing.T) {
		resp, err := client.DeployCatalogEntry(ctx, &marketplacev1.DeployCatalogEntryRequest{
			EntryName:       "prometheus-mcp-server",
			TargetNamespace: "default",
			InstanceName:    "e2e-test-prom",
		})
		require.NoError(t, err)
		assert.Equal(t, marketplacev1.DeployStatus_DEPLOY_STATUS_CREATED, resp.Status)

		t.Cleanup(func() {
			kubectl(t, "delete", "mcpserver", "e2e-test-prom", "-n", "default", "--ignore-not-found")
		})

		// Verify MCPServer was created
		waitForMCPServer(t, ctx, "e2e-test-prom", "default")
	})
}
```

### Quality Gate

- All unit tests pass with `go test ./internal/marketplace/...`.
- Integration tests pass against a real PostgreSQL (can use testcontainers).
- E2E test passes in a Kind cluster with the marketplace stack deployed.
- Code coverage above 80% for deployer and validation packages.

### Testing Command

```bash
# Unit tests
go test ./internal/marketplace/... -v -count=1

# Integration tests (requires PostgreSQL)
go test ./internal/marketplace/... -v -count=1 -tags=integration

# E2E tests (requires Kind cluster)
go test -tags=e2e ./test/e2e/ -run TestMarketplace -v -timeout 5m

# Coverage report
go test ./internal/marketplace/... -coverprofile=marketplace-coverage.out
go tool cover -func=marketplace-coverage.out
```

### Pitfalls

- **Test database isolation:** Integration tests that share a PostgreSQL instance must use unique table names or `CREATE SCHEMA` per test to prevent interference. Alternatively, use `testcontainers-go` for per-test databases.
- **Fake client limitations:** The `controller-runtime` fake client does not enforce CRD validation. Tests that rely on schema validation must use envtest instead.
- **gRPC client connection management:** Always close gRPC connections in test cleanup. Leaked connections cause port exhaustion in long CI runs.

### Progress Marker

- [ ] CRD validation tests cover all field constraints
- [ ] Store tests cover search, list, and pagination
- [ ] Deployer tests cover success, validation errors, and cleanup
- [ ] E2E test deploys from catalog and verifies MCPServer creation
- [ ] Coverage report generated and meets 80% target
