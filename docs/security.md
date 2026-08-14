# Security

This document describes the security architecture of MCP Gateway, covering
authentication, authorization, network security, and operational hardening.

---

## Threat Model

MCP Gateway protects against the following threats:

| Threat | Mitigation |
|--------|------------|
| Unauthorized agent accessing MCP servers | OAuth 2.1 + JWT validation at the gateway |
| Agent calling tools it should not access | Per-tool ABAC policies via Cerbos |
| Agent exfiltrating secrets (API keys, tokens) | Secrets injected into server pods only, never exposed via MCP protocol |
| Compromised MCP server container | Distroless images, nonroot UID, PodSecurity restricted profile |
| Lateral movement between MCP servers | NetworkPolicies isolate each server; no pod-to-pod communication |
| Man-in-the-middle attacks | mTLS between all components (via Istio service mesh) |
| Denial of service | Rate limiting and quotas enforced at the Envoy gateway |
| Unaudited tool usage | Every tool call recorded in the audit log with full context |

---

## Authentication

### OAuth 2.1 and Keycloak

All agents authenticate using the OAuth 2.1 client credentials grant:

1. The operator creates an OIDC client in Keycloak for each MCPAgent CR.
2. The agent uses its client ID and secret to obtain a JWT access token.
3. The JWT is included in every request to the gateway as a Bearer token.

Token configuration:
- **Signing algorithm**: RS256
- **Token lifetime**: 5 minutes (short-lived to limit blast radius)
- **Audience**: `mcp-gateway`
- **Required claims**: `client_id`, `scope`, `roles`

### JWT Validation

Envoy AI Gateway validates every incoming JWT:

- Signature verification against Keycloak's JWKS endpoint
- Expiration (`exp`) and not-before (`nbf`) checks
- Audience (`aud`) must match `mcp-gateway`
- Issuer (`iss`) must match the configured Keycloak realm URL

Invalid or expired tokens receive a `401 Unauthorized` response.

---

## Authorization

### Cerbos ABAC

Authorization is handled by Cerbos, a stateless policy decision point.
MCPPolicy CRs are translated into Cerbos policy resources by the operator.

Policy evaluation inputs:

| Input | Source |
|-------|--------|
| Principal ID | `client_id` claim from JWT |
| Principal roles | `roles` claim from JWT |
| Action | MCP method (e.g., `tools/call`) |
| Resource kind | `mcp-server` |
| Resource ID | `<server-name>/<tool-name>` |

### Policy Examples

Allow a role to call specific tools:

```yaml
- effect: ALLOW
  principals:
    roles:
      - developer
  actions:
    - tools/call
  resources:
    serverRef:
      name: github-mcp
    tools:
      - search_code
      - get_file_contents
```

Deny a specific agent from calling destructive tools:

```yaml
- effect: DENY
  principals:
    agentRefs:
      - name: untrusted-agent
  actions:
    - tools/call
  resources:
    serverRef:
      name: database-mcp
    tools:
      - drop_table
      - delete_rows
```

### Default Deny

If no policy rule matches a request, the request is denied. This ensures that
agents cannot access tools unless explicitly permitted.

---

## Network Security

### NetworkPolicies

The operator creates NetworkPolicies for each MCP server:

- **Ingress**: allow traffic only from the Envoy gateway pods
- **Egress**: allow traffic to required external services (configured per server)
- **Default deny**: all other traffic is blocked

### mTLS via Istio

When Istio is enabled, all inter-component communication uses mutual TLS:

- Envoy to MCP server pods
- Operator to Keycloak
- Operator to Cerbos
- Operator to OTel Collector

Certificate rotation is handled automatically by Istio's Citadel.

---

## Secret Management

### Design Principles

1. **Secrets are never exposed to agents.** API keys, tokens, and credentials
   are injected into MCP server pods as environment variables. The MCP protocol
   layer cannot access or return them.

2. **Secrets are not stored in CRDs.** MCPServer CRs reference Kubernetes
   Secrets by name and key. The actual secret values live only in etcd
   (encrypted at rest when configured).

3. **Principle of least privilege.** Each MCP server pod only receives the
   secrets it needs, referenced in `spec.secrets[]`.

### Secret Injection

```yaml
spec:
  secrets:
    - envVar: GITHUB_TOKEN
      secretRef:
        name: github-creds
        key: token
```

The operator mounts the referenced Secret and sets the environment variable in
the container spec. The Secret is not visible in the MCPServer status or in any
API response.

---

## Audit Logging

Every tool call is recorded in the audit log with the following fields:

| Field | Description |
|-------|-------------|
| `timestamp` | When the call occurred (UTC) |
| `agent_id` | The MCPAgent that made the call |
| `server_name` | The target MCPServer |
| `tool_name` | The tool that was invoked |
| `action` | The MCP method (e.g., `tools/call`) |
| `policy_decision` | `ALLOW` or `DENY` |
| `policy_name` | The MCPPolicy that matched |
| `duration_ms` | How long the call took |
| `status_code` | HTTP status code of the response |
| `error` | Error message if the call failed |

Audit logs are:
- Written to structured JSON on stdout (collected by the cluster log pipeline)
- Exported as OTel log records to the configured collector
- Queryable via Grafana Loki or any log aggregation system

---

## Container Security

### Image Hardening

- **Multi-stage builds**: build in a Go builder image, run in `gcr.io/distroless/static-debian12`
- **Nonroot user**: all containers run as UID 65532 (nonroot)
- **No shell**: distroless images contain no shell, package manager, or utilities
- **Read-only filesystem**: containers use `readOnlyRootFilesystem: true`

### PodSecurity

All namespaces managed by MCP Gateway enforce the `restricted` PodSecurity
standard:

- `runAsNonRoot: true`
- `allowPrivilegeEscalation: false`
- `seccompProfile: RuntimeDefault`
- `capabilities: drop: ["ALL"]`

### Pod Configuration

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  fsGroup: 65532
  seccompProfile:
    type: RuntimeDefault
containers:
  - securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
          - ALL
```

---

## Vulnerability Management

### Container Scanning

- **Trivy** scans all container images in CI and on a nightly schedule
- Marketplace entries include scan results in `spec.security`
- Images with critical CVEs are blocked from the marketplace

### Dependency Scanning

- **govulncheck** runs in CI to detect known vulnerabilities in Go dependencies
- **Dependabot** or **Renovate** keeps dependencies up to date
- Go module checksums are verified via the Go checksum database

### Supply Chain

- Container images are signed with Cosign
- Marketplace entries can include an image digest and signature reference
  (`spec.source.digest`, `spec.source.signatureRef`)
- SBOM (Software Bill of Materials) is tracked per marketplace entry
  (`spec.security.sbomRef`)
