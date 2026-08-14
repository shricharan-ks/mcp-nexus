# MCP Control Plane — Market Analysis, Feasibility & Architecture Plan

> **Project**: MCP Gateway (working title — final name TBD)
> **Date**: August 14, 2026
> **Author**: Chief System Architect
> **Audience**: Internal stakeholders + open-source community
> **Status**: Idea pitch — pre-implementation

---

## Executive Summary

The Model Context Protocol (MCP) has become the universal standard for connecting AI agents to external tools and data sources — 97M monthly SDK downloads, 10,000+ public servers, 300+ clients, governed by the Linux Foundation. But production-grade management tooling has not kept pace. Organizations deploying MCP servers face a fragmented landscape of 49+ point solutions, none of which delivers a unified Kubernetes-native control plane.

We propose building an open-source, vendor-neutral MCP Control Plane that fills three critical gaps no existing solution addresses:

1. **A Kubernetes Operator** treating MCP servers as first-class CRDs (deploy, scale, health-check — like Deployments)
2. **An agent-centric governance model** where AI agents are first-class entities with identity, per-tool RBAC, rate limits, and audit trails
3. **A security-vetted marketplace** for 1-click deployment of community MCP servers with vulnerability scanning

The architecture composes battle-tested open-source components (Envoy AI Gateway, Cerbos, Istio, MLflow) rather than reinventing them. MVP (deploy + auth + agent RBAC) is achievable in 13 weeks with 2-3 engineers. The 6-9 month window before incumbent API gateway vendors close the gap makes this the right time to establish the category-defining platform.

---

## Table of Contents

- [Part 1: Market Analysis](#part-1-market-analysis)
- [Part 2: Idea Feasibility](#part-2-idea-feasibility)
- [Part 3: System Architecture](#part-3-system-architecture)
- [Part 4: Repository Structure](#part-4-repository-structure)
- [Part 5: Iterative Build Plan](#part-5-iterative-build-plan)
- [Part 6: CI/CD, Testing & Release](#part-6-cicd-testing--release)
- [Part 7: Positioning & Differentiation](#part-7-positioning--differentiation)

---

## Part 1: Market Analysis

### 1.1 Market Size & Growth

| Metric | Value | Source |
|--------|-------|--------|
| **TAM** | $8.2B by 2028 | Intersection of API Management ($6.8B, 25.7% CAGR), AI/ML Infra ($4.4B, 32% CAGR), Cloud-Native Platforms ($9.1B, 22% CAGR) |
| **SAM** | $1.4B by 2028 | ~180K enterprises running K8s where 15-20% deploy agentic AI workloads |
| **SOM** | $35-70M by 2028 | 2.5-5% SAM capture based on CNCF open-source-to-commercial conversion rates |

MCP has crossed the adoption inflection point: the ratio of servers-to-management-tools (10,000:49) is comparable to Docker containers circa 2015 before Kubernetes emerged.

### 1.2 Market Segments

| Segment | Est. Orgs | MCP Maturity | Primary Need | Budget Range |
|---------|-----------|-------------|--------------|-------------|
| Enterprise Platform Teams (F2000) | ~8,000 | Early production (5-20 servers) | Governance, security, compliance | $50K-500K/yr |
| AI-Native Startups (Series A-C) | ~12,000 | Heavy production (10-100+ servers) | Speed to deploy, marketplace, DX | $5K-50K/yr |
| Platform Engineering / DevOps | ~45,000 teams | Evaluating / early adoption | K8s-native lifecycle, GitOps, self-service | Infrastructure budget |
| System Integrators & Consultancies | ~3,000 firms | Building for clients | White-label, multi-tenant | Per-deployment |

### 1.3 Competitive Landscape

**49+ MCP gateways exist** (22 open-source + 27 commercial). They cluster into four structural categories:

| Category | Key Players | Strength | Structural Limitation |
|----------|------------|----------|----------------------|
| **A: Gateway/Proxy Only** | AgentGateway (LF, 4.3K stars), Tyk MCP GW, Envoy AI GW (CNCF) | Mature networking, CNCF/LF credibility | Zero server lifecycle management |
| **B: Lifecycle Only** | ToolHive (Stacklok, 2K stars), Microsoft MCP GW (MIT, C#) | Container security, K8s integration | No rate limiting, no agent identity |
| **C: Full-Stack OSS** | IBM ContextForge (4.3K stars, Python) | Most feature-complete on paper | Heavy footprint (PG+Redis+Nginx), no K8s operator, no agent model |
| **D: Commercial SaaS** | Cloudflare, Composio ($25M), Smithery, Turbo MCP ($8.2M), ACI.dev ($3M), Glama | Managed experience, large catalogs | Vendor lock-in, no self-hosted for regulated industries |

#### Detailed Competitor Matrix

| Solution | Stars | Lifecycle | Auth | RBAC | Rate Limit | Registry | Marketplace | K8s Operator | Monitoring | Secrets | Agent Mgmt | License |
|----------|-------|-----------|------|------|------------|----------|-------------|-------------|------------|---------|-----------|---------|
| AgentGateway | 4.3K | No | Yes | Yes (CEL) | Partial | Yes | No | No | OTel | JWT/API key | No | Apache-2.0 |
| IBM ContextForge | 4.3K | Partial | Yes | Partial | Yes | Yes | No | No | OTel | JWT | No | Apache-2.0 |
| ToolHive | 2K | Yes | Yes (OIDC) | Yes | No | Yes | Partial | Yes | OTel+Prom | Encrypted | No | Apache-2.0 |
| Tyk MCP GW | — | No | Yes | Yes | **Yes (5-level)** | Yes | No | No | Analytics | OAuth 2.1 | No | MPL-2.0 |
| Envoy AI GW | — | No | Yes | Yes (CEL+JWT) | Partial | Yes | No | No | Per-tool OTel | OAuth | No | Apache-2.0 |
| Microsoft MCP GW | 783 | Yes | Yes (Entra) | Yes | No | Yes | No | Partial | App Insights | No | Preview | MIT |
| Cloudflare | 729 | Yes | Yes (Zero Trust) | Partial | Yes (WAF) | No | No | No | Full logs | Workers secrets | No | Proprietary |
| Composio | — | No | Yes (OAuth) | Partial | No | Yes | Yes | No | Metadata | Managed OAuth | No | MIT + Commercial |
| Smithery.ai | — | Yes | Yes | No | Unknown | **Yes (6K+)** | Yes | No | Unknown | Partial | No | Proprietary |
| Turbo MCP | — | Yes | Yes (OIDC) | Yes | No | Yes | Partial | Yes | Audit logs | Partial | Yes (kill-switch) | Proprietary |
| ACI.dev / Gate22 | 4.9K | Partial | Yes | Yes (NL) | Unknown | Yes | Yes | No | Yes | Yes | No | Apache-2.0 |
| Glama.ai | — | Yes | Yes | Partial | Unknown | **Yes (72K+)** | Yes | No | Yes | Yes | No | Proprietary |
| **Our Platform** | — | **Yes** | **Yes** | **Yes (ABAC)** | **Yes (per-tool)** | **Yes** | **Yes (vetted)** | **Yes (CRD)** | **OTel+MLflow** | **Vault/ESO** | **Yes (1st class)** | Apache-2.0 |

**Key finding: No single solution combines** a K8s-native control plane (CRD-based lifecycle) with a production-grade gateway (rate limiting, agent identity) and a curated marketplace with security vetting.

### 1.4 Gap Analysis — What No One Does Well

| Gap | Evidence | Who Suffers |
|-----|----------|-------------|
| **No K8s Operator with MCP-specific CRDs** | ToolHive has an operator but no gateway; Envoy has MCPRoute but no lifecycle | Platform engineers write custom Helm for each server |
| **No unified lifecycle + gateway** | Every solution is one or the other | Teams duct-tape 2-3 tools together |
| **No agent-as-entity model** | Zero solutions treat agents as first-class principals with identity and quotas | Cannot audit which agent called which tool |
| **Rate limiting missing or primitive** | Only Tyk has multi-level; AgentGateway/ToolHive/Envoy/IBM all lack it | No blast radius controls in production |
| **Credential management crisis** | 88% of MCP servers need credentials; 53% use insecure static secrets; only 8.5% implement OAuth 2.1 | Compliance blocker for every enterprise |
| **No marketplace with security vetting** | Smithery indexes 6K+ servers, Glama 72K+, but neither scans for vulnerabilities or provides SBOMs | Security-conscious orgs blocked from community servers |
| **No MLflow/observability integration** | Tool call traces invisible to existing ML observability stacks | No cost attribution, no debugging path |

### 1.5 Timing — Why Now

Five converging forces make August 2026 the optimal entry point:

1. **The Stateless Spec (2026-07-28)** — Released 3 weeks ago. Eliminates session management complexity. New `Mcp-Method` and `Mcp-Name` HTTP headers enable routing without JSON body parsing. Gateways built before this carry legacy session code. New entrants build on the clean abstraction.

2. **Linux Foundation Governance (Dec 2025)** — MCP moved under the Agentic AI Foundation, removing single-vendor (Anthropic) risk. Enterprise procurement teams now treat MCP as a viable standard, not a proprietary bet. This unlocks budget.

3. **Adoption Inflection** — 97M monthly SDK downloads, 10K+ servers, 300+ clients. The ecosystem has outgrown ad-hoc management. Gartner projects 75% of API gateway vendors will ship MCP features by EOY 2026.

4. **First-Mover Window** — 6-9 months before incumbent API gateway vendors (Kong, AWS, Google) close the gap with feature additions to existing products. Winning on K8s operator depth is the strategy.

5. **Security Urgency** — 88% credential-requirement / 8.5% OAuth adoption gap is a compliance emergency. NSA published formal MCP security guidance in May 2026. 30+ CVEs filed against MCP ecosystem in Jan-Feb 2026 alone.

### 1.6 Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Spec volatility — MCP spec changes again | Medium | High | Architect around abstractions; stateless shift actually stabilizes for gateways. Participate in AAIF working groups. |
| Incumbent entry — Kong/AWS/Google ship MCP management | High (12-18mo) | High | Move fast on K8s Operator + lifecycle gap. Incumbents add MCP as a feature, not a purpose-built platform. Win on depth. |
| CNCF consolidation — Envoy/AgentGateway expand scope | Medium | Medium | Contribute upstream or position as complementary. Envoy ecosystem historically welcomes extensions. |
| Alternative protocols — A2A (Google), ACP (Cisco) | Low-Medium | Medium | Build protocol-agnostic control plane abstractions. Support A2A agent cards alongside MCP manifests. |
| Open-source sustainability | Medium | High | Open-core model: operator free, enterprise features (multi-tenancy, audit, SLA) licensed. |
| Cloudflare bundles MCP gateway free with Workers | Low | Very High | Cloudflare is edge-only and proprietary. Self-hosted, on-prem, and hybrid-cloud segments structurally unavailable to them. |

### 1.7 Target Personas

#### "Platform Priya" — Sr. Platform Engineer, Fortune 500 Financial Services
- **Context**: Runs 200+ microservices on K8s. AI team asked her to "make MCP servers production-ready."
- **Pain**: No CRDs for MCP. Writing custom Helm charts per server. Compliance team asking about credential rotation — she has no answer.
- **Need**: K8s Operator managing MCP servers like Deployments, with HPA/PDB/NetworkPolicy integration.
- **Budget**: Infrastructure platform, $100K-300K/yr for new tooling.

#### "Aiden the AI Architect" — Head of AI Engineering, Series B Startup (50 engineers)
- **Context**: Building multi-agent system with 30+ MCP servers for customer support automation.
- **Pain**: Runaway agent loop cost $12K in API calls last month. No visibility into which agent called which tool. Uses 4 different tools duct-taped together (ToolHive + custom proxy + Grafana + manual scripts).
- **Need**: Single control plane for deploy, rate limit, trace, alert. Marketplace for discovering vetted servers.
- **Budget**: Engineering tools, $20K-50K/yr, willing to pay for operational sanity.

#### "Govind from GRC" — VP Governance Risk & Compliance, Healthcare Enterprise
- **Context**: Responsible for HIPAA/SOC2 compliance of all AI tool usage.
- **Pain**: Cannot answer "which agent accessed patient data through which tool at what time." 88% of MCP servers use credentials that aren't rotated. Has blocked further MCP adoption until gaps are resolved.
- **Need**: Agent identity with full audit trails, automated credential management with OAuth 2.1 enforcement, policy-as-code.
- **Budget**: Compliance tooling, $200K-500K/yr, driven by audit findings.

#### "Maya the ML Platform Lead" — Staff Engineer, Cloud-Native SaaS
- **Context**: Manages MLOps stack (MLflow, Kubeflow, model serving). Now asked to extend observability to agentic AI.
- **Pain**: Tool calls are a black box. Cannot trace customer-facing errors back through agents to specific MCP tool invocations. Cost attribution per agent workflow is impossible.
- **Need**: OTel-native tool call tracing that plugs into existing observability stack, with MLflow integration for experiment-level attribution.
- **Budget**: MLOps platform, $50K-150K/yr.

---

## Part 2: Idea Feasibility

### 2.1 Value Proposition

> **The only vendor-neutral, Kubernetes-native control plane that unifies MCP server lifecycle management, per-agent RBAC with tool-level granularity, a curated marketplace, and MLflow observability — composing battle-tested open-source components rather than reinventing them.**

### 2.2 Build vs. Integrate Strategy

| Component | Strategy | Technology | Rationale |
|-----------|----------|-----------|-----------|
| K8s Operator (MCPServer CRD) | **BUILD** | Go + controller-runtime | Core gap — doesn't exist anywhere |
| Agent Manager (MCPAgent CRD) | **BUILD** | Go + Keycloak client | Agent-as-entity model — novel concept |
| Control Plane API | **BUILD** | Go + gRPC + REST gateway | Orchestration layer tying everything together |
| Marketplace | **BUILD** | Catalog YAML + scan pipeline | Security-vetted deploy — doesn't exist |
| Data Plane (proxy) | **INTEGRATE** | Envoy AI Gateway | Apache 2.0, MCPRoute CRD, CNCF, GA since June 2026 |
| Policy Engine | **INTEGRATE** | Cerbos | Apache 2.0, per-tool ABAC, sub-1ms, MCP integration exists |
| Rate Limiting | **INTEGRATE** | Envoy rate limit service + Redis | Industry standard, well-documented |
| Observability | **INTEGRATE** | OTel + MLflow + Prometheus/Grafana | Standards-based, widely adopted |
| Service Mesh | **INTEGRATE** | Istio | mTLS, network RBAC, ExtAuthz, CNCF graduated |
| Secrets | **INTEGRATE** | External Secrets Operator + Vault | Runtime injection, never exposed to agents |
| Identity | **INTEGRATE** | Keycloak (or any OIDC) | Open source, DCR support, MCP-compliant |

### 2.3 Feasibility Assessment

| Dimension | Assessment | Confidence |
|-----------|-----------|------------|
| Technical feasibility | High — all building blocks exist and are production-proven | 95% |
| Team size required | 2-3 engineers for MVP (Phase 0-3), 5-7 for full platform | — |
| Time to MVP demo | **13 weeks** (deploy + auth + agent RBAC working) | 80% |
| Time to production GA | **28 weeks** (all phases including hardening) | 70% |
| Key technical risk | Envoy AI Gateway API stability (v1alpha1) | Mitigated by abstraction layer |
| Key market risk | Incumbent entry within 6-9 months | Mitigated by K8s operator depth |

---

## Part 3: System Architecture

### 3.1 High-Level Architecture

```
                          +---------------------------+
                          |     React / Next.js UI    |
                          |  (Dashboard, Marketplace) |
                          +------------+--------------+
                                       |
                                       | REST/gRPC-Web
                                       v
+----------------+      +----------------------------+      +----------------+
|   Keycloak     |<---->|   Control Plane API (Go)   |<---->|   PostgreSQL   |
| (OIDC/OAuth2.1)|      |   gRPC + REST Gateway      |      | (catalog, audit|
+----------------+      +----------------------------+      |  agent registry)
        |                     |              |               +----------------+
        | JWT                 | reconcile    | sync policies
        v                     v              v
+----------------+    +---------------+  +----------+
| Envoy AI GW    |    | K8s Operator  |  | Cerbos   |
| (Data Plane)   |    | (Go, ctrl-rt) |  | (Policy) |
| - MCPRoute     |    +-------+-------+  +----------+
| - ExtAuthz     |            |
| - RateLimit    |            | manages CRDs
| - OTel export  |            v
+-------+--------+    +------------------+
        |              | K8s API Server   |
        | routes to    | MCPServer CR     |
        v              | MCPAgent CR      |
+-------+--------+    | MCPPolicy CR     |
| MCP Server Pods|    | MCPMarketplace CR|
| (workloads)    |    +------------------+
+-------+--------+            |
        |                     | watches
        v                     v
+----------------+    +------------------+
| External APIs  |    | External Secrets |
| (GitHub, Slack |    | Operator -> Vault|
|  DBs, etc.)    |    | / AWS SM / GCP SM|
+----------------+    +------------------+

        Observability Pipeline
        ~~~~~~~~~~~~~~~~~~~~~~
MCP Server Pods --OTel--> OTel Collector --+--> MLflow (traces)
Envoy AI GW    --OTel--> OTel Collector    +--> Prometheus (metrics)
                                           +--> Grafana (dashboards)
```

### 3.2 Data Flow — Agent Tool Call

```
Agent          Envoy AI GW      Cerbos       MCP Server Pod
  |                |               |               |
  |-- POST /mcp -->|               |               |
  |  (JWT + Mcp-Method             |               |
  |   + Mcp-Name headers)          |               |
  |                |               |               |
  |                |-- ExtAuthz -->|               |
  |                |  {agent_id,   |               |
  |                |   method,     |               |
  |                |   tool_name}  |               |
  |                |<-- ALLOW/DENY-|               |
  |                |                               |
  |                |-- rate limit check            |
  |                |                               |
  |                |-- forward (mTLS via Istio) -->|
  |<-- response -----------------------------------|
  |                |                               |
  |                |-- emit OTel span              |
```

The 2026-07-28 MCP spec requires `Mcp-Method` and `Mcp-Name` HTTP headers on every request. This means Envoy can route, authorize, and rate-limit based on headers alone — no JSON body parsing required.

### 3.3 Custom Resource Definitions

#### MCPServer — Deploy and manage MCP server workloads

```yaml
apiVersion: mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: github-mcp
  namespace: mcp-servers
spec:
  source:
    image: ghcr.io/github/mcp-server:v2.1.0
    port: 8080
    healthCheck:
      path: /health
      periodSeconds: 10
  protocol:
    transport: streamable-http
    endpoint: /mcp
  scaling:
    minReplicas: 1
    maxReplicas: 10
    scaleToZero:
      enabled: true
      idleTimeoutSeconds: 300
    metrics:
      - type: custom
        name: mcp_active_connections
        target: { type: AverageValue, averageValue: "50" }
  secrets:
    - envVar: GITHUB_TOKEN
      secretRef: { name: github-mcp-creds, key: token }
  resources:
    requests: { cpu: "100m", memory: "128Mi" }
    limits: { cpu: "500m", memory: "512Mi" }
status:
  phase: Running    # Pending | Deploying | Running | Scaling | Failed | Terminated
  replicas: 2
  readyReplicas: 2
  discoveredCapabilities:
    tools: ["create_issue", "list_repos", "search_code"]
    resources: ["repo://", "issue://"]
    prompts: ["summarize_pr"]
    lastDiscoveredAt: "2026-08-14T10:00:00Z"
    cacheTTLMs: 300000
```

#### MCPAgent — Agent identity, permissions, and quotas

```yaml
apiVersion: mcp-gateway.io/v1alpha1
kind: MCPAgent
metadata:
  name: coding-assistant
spec:
  identity:
    oidcClientId: agent-coding-assistant
  serverAccess:
    - serverRef: { name: github-mcp }
      policyRef: { name: dev-tool-policy }
    - serverRef: { name: slack-mcp }
      policyRef: { name: comms-readonly }
  rateLimits:
    global:
      requestsPerMinute: 600
    perTool:
      - tool: "create_issue"
        requestsPerMinute: 30
      - tool: "search_code"
        requestsPerMinute: 120
  quota:
    maxConcurrentConnections: 10
    maxMonthlyToolCalls: 100000
status:
  registeredAt: "2026-08-14T09:00:00Z"
  currentMonthToolCalls: 4521
  activeConnections: 3
```

#### MCPPolicy — Per-tool access control rules

```yaml
apiVersion: mcp-gateway.io/v1alpha1
kind: MCPPolicy
metadata:
  name: dev-tool-policy
spec:
  rules:
    - effect: ALLOW
      principals:
        roles: ["developer-agent"]
      actions: ["tools/call", "tools/list", "resources/read"]
      resources:
        serverRef: { name: github-mcp }
        tools: ["list_repos", "search_code", "get_file"]
    - effect: DENY
      principals:
        roles: ["developer-agent"]
      resources:
        tools: ["delete_repo", "force_push"]
```

#### MCPMarketplaceEntry — Catalog entries with security metadata

```yaml
apiVersion: mcp-gateway.io/v1alpha1
kind: MCPMarketplaceEntry
metadata:
  name: github-official
spec:
  displayName: "GitHub MCP Server"
  vendor: "GitHub"
  version: "2.1.0"
  category: "developer-tools"
  source:
    image: ghcr.io/github/mcp-server:v2.1.0
    signatureRef: "cosign://ghcr.io/github/mcp-server:sha256-abc.sig"
  installTemplate:
    mcpServerSpec: { ... }
    requiredSecrets:
      - name: GITHUB_TOKEN
        description: "Personal access token with repo scope"
    defaultPolicy:
      allowedTools: ["list_repos", "search_code"]
      deniedTools: ["delete_repo"]
  security:
    scanStatus: passed
    lastScannedAt: "2026-08-10T00:00:00Z"
    cveCount: 0
    sbomRef: "oci://ghcr.io/github/mcp-server:v2.1.0-sbom"
```

### 3.4 Operator State Machine

```
               create CR
                  |
                  v
 +----------+  +----------+  +---------+  +---------+
 | Pending  |->|Deploying |->| Running |->| Scaling |
 +----------+  +----------+  +---------+  +---------+
      |             |            |   ^          |
      v             v            v   |          |
 +--------+    +--------+  +----------+        |
 | Failed |    | Failed |  | Updating |--------+
 +--------+    +--------+  +----------+
      ^                         |
      +-------------------------+
                                |
                         +------------+
          delete CR ---> |Terminating |
                         +------------+
```

**Reconciliation steps:**
1. **Pending** — Validate spec. Resolve ExternalSecret references. Create K8s Secret if needed.
2. **Deploying** — Create Deployment + Service + ServiceMonitor. Inject Istio sidecar annotation.
3. **Running** — Call `server/discover` on MCP server. Populate `status.discoveredCapabilities`. Generate MCPRoute for Envoy. Create HPA.
4. **Updating** — Rolling update of Deployment. Re-run discovery. Regenerate MCPRoute if capabilities changed.
5. **Terminating** — Delete Deployment, Service, HPA, MCPRoute. Clean agent bindings.

### 3.5 Observability — Per Tool Call Capture

| Attribute | Source | Example |
|-----------|--------|---------|
| `mcp.agent.id` | JWT claim | `coding-assistant` |
| `mcp.server.name` | MCPRoute | `github-mcp` |
| `mcp.method` | `Mcp-Method` header | `tools/call` |
| `mcp.tool.name` | `Mcp-Name` header | `create_issue` |
| `mcp.duration_ms` | Envoy timing | `142` |
| `mcp.status` | Response | `success` / `error` |
| `mcp.tokens.in` | Body parsing | `1250` |
| `mcp.tokens.out` | Body parsing | `380` |

MLflow experiments: one per MCPServer, one run per tool call. OTel Collector batch-exports spans via MLflow `log_batch` API.

### 3.6 Secret Management Flow

```
Vault / AWS SM --> External Secrets Operator --> K8s Secret --> MCP Server Pod (env var)
                        ^                                            ^
                 ExternalSecret CR                            secretRef in
                 (created by operator                         Deployment spec
                  from MCPServer.spec.secrets)
```

Secrets never logged, never passed through Envoy, never visible to agents. Rotation via ESO polling — K8s Secret update triggers rolling restart via hash annotation.

---

## Part 4: Repository Structure

### Primary Monorepo: `mcp-gateway`

```
mcp-gateway/
  api/v1alpha1/           # CRD Go types (MCPServer, MCPAgent, MCPPolicy, MCPMarketplace)
  cmd/
    operator/             # Operator binary entrypoint
    apiserver/            # Control Plane API binary entrypoint
  internal/
    controller/           # Reconcilers for each CRD
    cerbos/               # Cerbos policy sync client
    keycloak/             # Keycloak admin client
    discovery/            # MCP server/discover caller
    envoy/                # MCPRoute generator
    marketplace/          # Install/scan pipeline
    observability/        # OTel span -> MLflow converter
  config/
    crd/                  # Generated CRD YAML
    rbac/                 # Operator RBAC manifests
    samples/              # Example CRs
  deploy/
    helm/mcp-gateway/     # Helm chart for full platform
    docker-compose.yaml   # Local dev without K8s
  ui/                     # Next.js dashboard + marketplace browser
  scripts/kind/           # Kind cluster setup/teardown
  docs/                   # Architecture, user guides
```

### Companion Repos

| Repo | Purpose | Why Separate |
|------|---------|-------------|
| `mcp-gateway-data-plane` | Envoy AI Gateway config, MCPRoute templates, rate limit policies | Tracks upstream Envoy releases |
| `mcp-gateway-marketplace-catalog` | Community MCP server definitions (1 YAML per server) | Community contributions via PRs without touching core code |

---

## Part 5: Iterative Build Plan

### MVP Target

**Deploy an MCP server via CRD + OAuth authentication + agent gets scoped tool access via RBAC.** This demonstrates the full value proposition end-to-end in a single demo.

### Phase Overview

| Phase | Weeks | Goal | Key Deliverable |
|-------|-------|------|-----------------|
| **0: Bootstrap** | 1 | Dev environment works | `make kind-up` shows running operator pod |
| **1: Core Operator** | 2-5 | CRD deploys MCP servers | `kubectl apply` MCPServer -> working pod |
| **2: Gateway** | 6-9 | Traffic proxied with auth | JWT-authenticated MCP calls through Envoy |
| **3: Agent RBAC** | 10-13 | Per-tool access control | Agent A allowed, Agent B denied on same tool |
| **4: Observability** | 14-16 | End-to-end traces | MLflow traces + Grafana dashboards |
| **5: Marketplace** | 17-20 | 1-click deploy from catalog | Browse -> deploy -> running in <60s |
| **6: UI Dashboard** | 21-24 | Web management interface | Full GUI for servers, agents, marketplace |
| **7: Hardening** | 25-28 | Production ready | 1000 agents, p99 <500ms, HA, secret rotation |

### Phase 0: Bootstrap (Week 1)

**Goal**: `make kind-up && make dev-deploy` produces a running operator pod.

**Tasks**:
1. Initialize Go module with `kubebuilder init --domain mcp-gateway.io`
2. Scaffold gRPC service with `buf` for proto management
3. Create Next.js UI skeleton with shadcn/ui
4. Write Makefile with targets: `lint`, `test`, `build`, `kind-up`, `kind-down`, `dev-deploy`
5. GitHub Actions CI: Go lint+test, Helm lint, UI lint
6. Helm chart skeleton for operator Deployment + RBAC
7. docker-compose.yaml for non-K8s local dev
8. Kind cluster scripts

**DoD**: CI passes. `kubectl get pods -n mcp-system` shows 1/1 READY.

### Phase 1: Core Operator (Weeks 2-5)

**Goal**: Apply MCPServer CR -> operator deploys working MCP server pod.

**Tasks**:
1. Define MCPServer CRD types in `api/v1alpha1/mcpserver_types.go`
2. Implement MCPServerReconciler: Deployment + Service creation
3. stdio-to-HTTP bridge sidecar for stdio-transport servers
4. Status subresource with phase tracking
5. Health checking: liveness/readiness probes
6. Finalizer for cleanup, leader election for HA
7. 3 example CRs (filesystem, GitHub, PostgreSQL)
8. `envtest` integration tests (80%+ coverage)

**Demo**: `kubectl apply -f examples/github-server.yaml` -> `curl <svc>:8080/mcp` returns MCP response.

### Phase 2: Gateway Integration (Weeks 6-9)

**Goal**: MCP traffic flows through Envoy with auto-routing and JWT auth.

**Tasks**:
1. Envoy Gateway as Helm dependency
2. Auto-generate HTTPRoute/MCPRoute per MCPServer
3. JWT validation via Envoy ext_authz + Keycloak
4. TLS termination (cert-manager / OpenShift Route)
5. Basic rate limiting via Envoy rate limit service
6. Istio ambient mesh integration

**Demo**: Authenticated curl through gateway routes to MCP server. No JWT = 401.

### Phase 3: Agent & RBAC (Weeks 10-13) — MVP COMPLETE

**Goal**: MCPAgent CR defines per-tool access enforced by Cerbos.

**Tasks**:
1. MCPAgent CRD with allowed servers, tools, rate limits, quotas
2. MCPAgentReconciler: Keycloak client per agent
3. Cerbos deployment + YAML policy generation from MCPPolicy CRs
4. Authorization middleware in gateway path
5. Redis-backed quota tracking per agent
6. Audit logging: every tool call recorded
7. Integration tests for permission boundaries

**Demo**: Two agents — one gets 403 on restricted tools, other has full access. Rate limiting visible.

### Phase 4: Observability (Weeks 14-16)

**Goal**: End-to-end traces with metrics dashboards.

- OTel Collector DaemonSet, pipelines to MLflow + Prometheus
- Instrument operator, gateway, and API
- MLflow tracking server for traces
- Grafana dashboards: Platform Overview, Per-Server, Per-Agent
- Alerting rules: server down, error rate, quota exhaustion

### Phase 5: Marketplace (Weeks 17-20)

**Goal**: Browse catalog, 1-click deploy.

- Catalog YAML schema, seed with 10-15 popular servers
- Catalog indexer in control-plane API
- 1-click deploy API (generates MCPServer CR + Secret)
- Security scanning: Trivy, cosign, schema validation
- Server versioning

### Phase 6: UI Dashboard (Weeks 21-24)

**Goal**: Web management interface.

- TypeScript types from protobuf via `buf generate`
- Server Management, Agent Management, Monitoring, Marketplace views
- OIDC login via Keycloak
- Playwright E2E tests

### Phase 7: Production Hardening (Weeks 25-28)

**Goal**: Production-ready at scale.

- HA: multi-replica operator/API, PDB, failover <15s
- External Secrets Operator + Vault, rotation without downtime
- KEDA for scale-to-zero (cold start <10s)
- k6 load test: 1000 agents, p99 <500ms
- Security: NetworkPolicies, PodSecurity restricted, pen test
- DR: Velero backup/restore

---

## Part 6: CI/CD, Testing & Release

### CI/CD Pipelines

| Workflow | Trigger | Contents |
|----------|---------|----------|
| `ci.yaml` | PR to main | Go lint (`golangci-lint`), `go test`, Helm lint, UI lint |
| `build.yaml` | Push to main | Build operator/API/UI images, push to ghcr.io |
| `e2e-kind.yaml` | Push to main | Kind cluster + full deploy + E2E suite |
| `security-scans.yaml` | Push + weekly | Trivy image scan, `govulncheck`, OSSF Scorecard |
| `release.yaml` | Tag v* | Helm chart publish (OCI), OLM bundle, GitHub release |

### Testing Strategy

| Layer | Tool | Scope |
|-------|------|-------|
| Unit (Go) | `go test` + testify | Reconciler functions, gRPC handlers |
| Integration (Go) | `envtest` (controller-runtime) | CRD CRUD, reconcile loops against real etcd+apiserver |
| E2E (operator) | Kind cluster + shell scripts | Full CRD apply, verify pods/services, health checks |
| UI unit | Vitest + React Testing Library | Component rendering, state management |
| UI E2E | Playwright | Full dashboard flows against running API |
| Contract | `buf breaking` | Protobuf backward compatibility |

### Release Strategy

- **Platform**: CalVer `YYYY.MM.PATCH`
- **Helm chart**: SemVer, published to `ghcr.io/org/charts/mcp-gateway`
- **OLM (OpenShift)**: Operator bundle via `operator-sdk`, channels: `alpha`, `stable`
- **Go modules**: Tagged releases for CRD type imports

### Local Development

```bash
make dev-setup     # Install Go, Node, Kind, Helm, controller-gen, protoc
make kind-up       # Create Kind cluster, install CRDs + Envoy + Cerbos
make dev-deploy    # Build images, load into Kind, helm install
make dev-teardown  # Destroy Kind cluster
```

Docker-compose alternative for UI developers (operator + API + PG + Redis + OTel, no K8s required).

---

## Part 7: Positioning & Differentiation

### The MCP Management Trilemma

Today, organizations can choose **at most two** of three capabilities:

```
           Lifecycle Management
                  /\
                 /  \
                /    \
               / NONE \
              /  has   \
             /  all 3   \
            /____________\
  Traffic           Security-Vetted
  Governance        Marketplace
```

- **ToolHive**: Lifecycle + partial marketplace, but no traffic governance
- **AgentGateway/Envoy**: Traffic governance, but no lifecycle or marketplace
- **Smithery/Glama**: Marketplace, but no lifecycle or governance

Our platform is the first to deliver all three in a single, Kubernetes-native control plane.

### Five Differentiators

1. **MCPServer CRD** — No production K8s operator for MCP server lifecycle exists. We are the first.
2. **Agent-centric model** — Agents are first-class entities with identity, per-tool quotas, and audit trails. No competitor has this.
3. **Composable architecture** — Pluggable data plane (Envoy), policy engine (Cerbos), observability (MLflow/Prometheus). No vendor lock-in at any layer.
4. **Marketplace with security vetting** — Not just a catalog: deploy + vulnerability scan + SBOM + cosign verification.
5. **OpenShift/K8s-native** — Works where platform engineers already work. No new infrastructure paradigm to adopt.

### Open-Source Strategy

- **Core operator + CRDs**: Apache-2.0, always free
- **Community marketplace catalog**: Apache-2.0, open contributions
- **Enterprise features** (multi-tenancy, advanced audit, SLA-backed support): Commercial license
- **Governance**: DCO sign-off on all contributions, CNCF-style contributor ladder

### Success Metrics (12 months post-launch)

| Metric | Target |
|--------|--------|
| GitHub stars | 2,000+ |
| Monthly active clusters | 500+ |
| Marketplace catalog entries | 50+ vetted servers |
| Contributing organizations | 10+ |
| Enterprise customers | 5-10 |

---

## Appendix: Technology Reference

| Component | Version | License | Role |
|-----------|---------|---------|------|
| Go | 1.23+ | BSD | Operator + API language |
| controller-runtime | v0.19+ | Apache-2.0 | K8s operator framework |
| Envoy AI Gateway | v1.0+ | Apache-2.0 | MCP traffic proxy (data plane) |
| Cerbos | v0.40+ | Apache-2.0 | Policy decision point (ABAC) |
| Keycloak | 26+ | Apache-2.0 | OIDC/OAuth 2.1 identity provider |
| Istio | 1.24+ | Apache-2.0 | Service mesh (mTLS, network RBAC) |
| MLflow | 3.15+ | Apache-2.0 | Tool call trace storage + MCP Registry |
| Prometheus | 2.55+ | Apache-2.0 | Metrics collection |
| Grafana | 11+ | AGPL-3.0 | Dashboards |
| OTel Collector | 0.110+ | Apache-2.0 | Telemetry pipeline |
| External Secrets Operator | 0.10+ | Apache-2.0 | Secret synchronization |
| PostgreSQL | 16+ | PostgreSQL | Control plane data store |
| Redis | 7+ | BSD | Rate limit counters, caching |
| Next.js | 15+ | MIT | Dashboard UI framework |
| KEDA | 2.16+ | Apache-2.0 | Scale-to-zero autoscaling |

---

*This document was prepared on August 14, 2026. Market data reflects the MCP ecosystem as of this date. The MCP specification referenced is version 2026-07-28.*
