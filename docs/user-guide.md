# User Guide

This guide covers day-to-day operations with MCP Gateway: managing servers,
agents, policies, the marketplace, and monitoring.

---

## MCPServer Management

### Creating an MCP server

An MCPServer CR describes a single MCP server. The operator creates a Deployment,
Service, and health-check configuration from the spec.

```yaml
apiVersion: mcp.mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: my-server
  namespace: mcp-servers
spec:
  source:
    image: ghcr.io/example/my-mcp-server:v1.0.0
    port: 8080
    healthCheck:
      path: /health
      periodSeconds: 10
  protocol:
    transport: streamable-http
    endpoint: /mcp
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "256Mi"
```

### Updating an MCP server

Change any field in `spec` and re-apply. The operator performs a rolling update:

```bash
kubectl edit mcps my-server -n mcp-servers
# or
kubectl apply -f my-server.yaml
```

The server transitions through phases: `Running -> Updating -> Running`.

### Scaling

Add a `scaling` block to enable horizontal autoscaling:

```yaml
spec:
  scaling:
    minReplicas: 2
    maxReplicas: 10
    scaleToZero:
      enabled: true
      idleTimeoutSeconds: 300
```

When `scaleToZero.enabled` is true, the server scales down to 0 replicas after
the idle timeout and wakes on the next incoming request.

### Deleting an MCP server

```bash
kubectl delete mcps my-server -n mcp-servers
```

The operator cleans up the Deployment, Service, and any related routing
configuration through owner references.

### Health checks

The operator configures liveness and readiness probes from `spec.source.healthCheck`.
If the health check fails repeatedly, the server enters the `Failed` phase.

### Stdio bridge

For MCP servers that communicate over stdin/stdout instead of HTTP, set the
transport to `stdio`:

```yaml
spec:
  protocol:
    transport: stdio
  source:
    image: ghcr.io/example/stdio-server:latest
    port: 8080
```

The operator deploys a stdio-to-HTTP bridge sidecar so the server is accessible
over the network like any streamable-http server.

### Secrets

Inject secrets into the MCP server container without exposing them to agents:

```yaml
spec:
  secrets:
    - envVar: GITHUB_TOKEN
      secretRef:
        name: github-mcp-creds
        key: GITHUB_TOKEN
    - envVar: DATABASE_URL
      secretRef:
        name: db-credentials
        key: url
```

Each entry maps a Kubernetes Secret key to an environment variable in the
server pod.

---

## MCPAgent Management

### Creating an agent

An MCPAgent registers an AI agent identity, binds it to one or more MCP servers,
and optionally sets rate limits and quotas.

```yaml
apiVersion: mcp.mcp-gateway.io/v1alpha1
kind: MCPAgent
metadata:
  name: research-agent
  namespace: mcp-servers
spec:
  identity:
    oidcClientId: agent-research
  serverAccess:
    - serverRef:
        name: github-mcp
      policyRef:
        name: github-readonly
    - serverRef:
        name: echo-server
  rateLimits:
    global:
      requestsPerMinute: 120
    perTool:
      - tool: search_code
        requestsPerMinute: 30
  quota:
    maxConcurrentConnections: 5
    maxMonthlyToolCalls: 10000
```

### Rate limits

- **Global**: caps total requests per minute across all servers.
- **Per-tool**: caps requests per minute for a specific tool name.

### Quotas

- **maxConcurrentConnections**: maximum simultaneous WebSocket/SSE connections.
- **maxMonthlyToolCalls**: hard cap on tool invocations per calendar month.
  The counter resets on the first of each month. The current count is visible
  in `status.currentMonthToolCalls`.

---

## MCPPolicy Management

### Creating a policy

An MCPPolicy defines ALLOW or DENY rules that are synced to the Cerbos policy
engine.

```yaml
apiVersion: mcp.mcp-gateway.io/v1alpha1
kind: MCPPolicy
metadata:
  name: data-team-policy
  namespace: mcp-servers
spec:
  rules:
    - effect: ALLOW
      principals:
        roles:
          - data-engineer
      actions:
        - tools/call
        - resources/read
      resources:
        serverRef:
          name: database-mcp
        tools:
          - query
          - list_tables
    - effect: DENY
      principals:
        roles:
          - data-engineer
      actions:
        - tools/call
      resources:
        serverRef:
          name: database-mcp
        tools:
          - drop_table
          - truncate
```

### Rule evaluation

Rules are evaluated in order. The first matching rule wins. If no rule matches,
the request is denied by default (deny-by-default).

### Principals

- **roles**: match the `role` claim in the agent's JWT.
- **agentRefs**: match specific MCPAgent resources by name.

### Actions

Standard MCP actions:

| Action | Description |
|--------|-------------|
| `tools/call` | Invoke a tool |
| `tools/list` | List available tools |
| `resources/read` | Read a resource |
| `resources/list` | List available resources |
| `prompts/get` | Retrieve a prompt |
| `prompts/list` | List available prompts |

### Verifying sync

Once applied, check that the policy synced to Cerbos:

```bash
kubectl get mcpp data-team-policy -n mcp-servers
```

The `Phase` column shows `Synced` when the policy is active in Cerbos.

---

## Marketplace

### Browsing the catalog

The marketplace is populated by MCPMarketplaceEntry CRs. List available entries:

```bash
kubectl get mcpme -n mcp-marketplace
```

Or use the web dashboard at `http://localhost:3000/marketplace`.

### Deploying from the catalog

Each marketplace entry contains an `installTemplate` with a pre-configured
MCPServerSpec. To deploy:

1. Find the entry: `kubectl get mcpme github-mcp -n mcp-marketplace -o yaml`
2. Review the `spec.installTemplate.requiredSecrets` -- create any required
   Kubernetes Secrets first.
3. Apply the install (the operator or dashboard creates the MCPServer from the
   template).

### Security scanning

Every marketplace entry tracks:

- **scanStatus**: `passed`, `failed`, `warning`, `pending`, or `not-scanned`
- **cveCount** / **criticalCveCount**: vulnerability counts from Trivy scans
- **sbomRef**: reference to the SBOM artifact

Entries with `scanStatus: failed` or critical CVEs are blocked from installation
by default.

---

## Monitoring

### OpenTelemetry traces

Every tool call produces a distributed trace spanning:
`Agent -> Envoy -> Cerbos -> MCP Server`

Traces are exported via OTLP to the configured collector endpoint.

### Prometheus metrics

The operator exports metrics at `/metrics` on port 8080:

| Metric | Type | Description |
|--------|------|-------------|
| `mcpgateway_tool_calls_total` | Counter | Total tool invocations by server, tool, agent, status |
| `mcpgateway_tool_call_duration_seconds` | Histogram | Tool call latency |
| `mcpgateway_active_connections` | Gauge | Current active agent connections |
| `mcpgateway_server_health_status` | Gauge | Server health (1=healthy, 0=unhealthy) |
| `mcpgateway_policy_decisions_total` | Counter | Policy allow/deny decisions |

### Grafana dashboards

Pre-built dashboards are available in `deploy/helm/dashboards/`:

- **MCP Gateway Overview** -- server health, request rates, error rates
- **Agent Activity** -- per-agent call volumes, rate limit hits, quota usage
- **Policy Decisions** -- allow/deny breakdown by policy, tool, and agent

### Alerts

Recommended Prometheus alert rules:

- MCP server unhealthy for more than 5 minutes
- Tool call error rate above 5%
- Agent approaching monthly quota (>90%)
- Policy sync failures

---

## Secret Management

### Kubernetes Secrets

The standard approach: create a Secret and reference it in `spec.secrets[]`:

```bash
kubectl create secret generic my-creds \
  --from-literal=API_KEY=sk-xxx \
  -n mcp-servers
```

Then reference in the MCPServer:

```yaml
spec:
  secrets:
    - envVar: API_KEY
      secretRef:
        name: my-creds
        key: API_KEY
```

Secrets are injected as environment variables into the server pod. They are
never exposed to agents or included in MCP protocol responses.

### External Secrets Operator

For production, use the External Secrets Operator to sync secrets from Vault,
AWS Secrets Manager, or other backends. The MCPServer secret references work
with any Kubernetes Secret regardless of how it was provisioned.
