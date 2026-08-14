# MCP Gateway — Implementation Guide

This directory contains step-by-step implementation guides for each phase of the MCP Gateway platform. Each guide is precise enough for an AI coding agent to execute without ambiguity.

## Progress Tracker

| Phase | Weeks | Goal | Status | Progress |
|-------|-------|------|--------|----------|
| **0: Bootstrap** | 1 | `make kind-up && make dev-deploy` shows running operator | [ ] Not Started | 0% |
| **1: Core Operator** | 2-5 | `kubectl apply` MCPServer deploys a working pod | [ ] Not Started | 0% |
| **2: Gateway** | 6-9 | JWT-authenticated MCP calls through Envoy | [ ] Not Started | 0% |
| **3: Agent RBAC** | 10-13 | Per-tool access control with Cerbos (MVP complete) | [ ] Not Started | 0% |
| **4: Observability** | 14-16 | End-to-end traces in MLflow + Grafana dashboards | [ ] Not Started | 0% |
| **5: Marketplace** | 17-20 | Browse catalog, 1-click deploy in <60s | [ ] Not Started | 0% |
| **6: UI Dashboard** | 21-24 | Web GUI for servers, agents, marketplace | [ ] Not Started | 0% |
| **7: Hardening** | 25-28 | HA, scale-to-zero, 1000 agents at p99 <500ms | [ ] Not Started | 0% |

**MVP = Phases 0-3** (13 weeks). Everything after is incremental value.

## Step Progress Detail

### Phase 0: Bootstrap
| Step | Description | Status |
|------|-------------|--------|
| 0.1 | Git init, .gitignore, LICENSE, DCO | [ ] |
| 0.2 | Go module init | [ ] |
| 0.3 | Kubebuilder scaffold | [ ] |
| 0.4 | Directory structure | [ ] |
| 0.5 | Makefile with all targets | [ ] |
| 0.6 | Dockerfile (multi-stage, distroless) | [ ] |
| 0.7 | Kind cluster config | [ ] |
| 0.8 | Kind setup scripts | [ ] |
| 0.9 | Helm chart skeleton | [ ] |
| 0.10 | docker-compose.yaml | [ ] |
| 0.11 | GitHub Actions CI | [ ] |
| 0.12 | CLAUDE.md | [ ] |
| 0.13 | CONTRIBUTING.md | [ ] |

### Phase 1: Core Operator
| Step | Description | Status |
|------|-------------|--------|
| 1.1 | MCPServer CRD types with kubebuilder markers | [ ] |
| 1.2 | MCPServerReconciler with state machine | [ ] |
| 1.3 | MCP server discovery client | [ ] |
| 1.4 | Unit tests (table-driven) | [ ] |
| 1.5 | envtest integration tests | [ ] |
| 1.6 | Example MCPServer CRs (3) | [ ] |
| 1.7 | E2E test script for Kind | [ ] |

### Phase 2: Gateway Integration
| Step | Description | Status |
|------|-------------|--------|
| 2.1 | Envoy Gateway Helm dependency + Gateway/GatewayClass | [ ] |
| 2.2 | HTTPRoute generation in MCPServerReconciler | [ ] |
| 2.3 | Keycloak deployment + realm bootstrap | [ ] |
| 2.4 | JWT validation via SecurityPolicy | [ ] |
| 2.5 | Rate limiting (Envoy rate limit service + Redis) | [ ] |
| 2.6 | E2E tests (auth, routing, rate limit) | [ ] |

### Phase 3: Agent & RBAC
| Step | Description | Status |
|------|-------------|--------|
| 3.1 | MCPAgent CRD + validation webhook | [ ] |
| 3.2 | MCPPolicy CRD + Cerbos translation | [ ] |
| 3.3 | MCPAgentReconciler (Keycloak lifecycle) | [ ] |
| 3.4 | Cerbos deployment + ext_authz adapter | [ ] |
| 3.5 | Per-agent rate limiting (Redis) | [ ] |
| 3.6 | Audit logging (structured JSON + webhook) | [ ] |
| 3.7 | Comprehensive tests (unit + envtest + E2E) | [ ] |

### Phase 4: Observability
| Step | Description | Status |
|------|-------------|--------|
| 4.1 | OTel Collector DaemonSet | [ ] |
| 4.2 | Instrument operator with OTel SDK | [ ] |
| 4.3 | Envoy MCP tracing + access logs | [ ] |
| 4.4 | MLflow tracking server + OTel converter | [ ] |
| 4.5 | Prometheus + Grafana (3 dashboards + alerts) | [ ] |
| 4.6 | E2E observability test (100 calls) | [ ] |

### Phase 5: Marketplace
| Step | Description | Status |
|------|-------------|--------|
| 5.1 | MCPMarketplaceEntry CRD | [ ] |
| 5.2 | Catalog YAML schema + 10 entries | [ ] |
| 5.3 | Catalog indexer (gRPC service + PostgreSQL) | [ ] |
| 5.4 | 1-click deploy flow | [ ] |
| 5.5 | Security scanning pipeline (Trivy + cosign) | [ ] |
| 5.6 | Tests | [ ] |

### Phase 6: UI Dashboard
| Step | Description | Status |
|------|-------------|--------|
| 6.1 | Next.js setup (shadcn, auth, API client) | [ ] |
| 6.2 | Server Management view | [ ] |
| 6.3 | Agent Management view (permission matrix) | [ ] |
| 6.4 | Monitoring view (Grafana embed + Recharts) | [ ] |
| 6.5 | Marketplace Browser (deploy dialog) | [ ] |
| 6.6 | Playwright E2E tests | [ ] |

### Phase 7: Production Hardening
| Step | Description | Status |
|------|-------------|--------|
| 7.1 | HA (multi-replica, PDB, failover <15s) | [ ] |
| 7.2 | Secret rotation (ESO + Vault) | [ ] |
| 7.3 | Scale-to-zero (KEDA + interceptor proxy) | [ ] |
| 7.4 | Load testing (k6, 1000 agents, p99 <500ms) | [ ] |
| 7.5 | Security hardening (NetworkPolicy, PodSecurity) | [ ] |
| 7.6 | Disaster recovery (Velero, RTO <30min) | [ ] |

## Guide Template

Each `phase-N-*.md` follows this structure:

```
# Phase N: Title (Weeks X-Y)

## Goal
One sentence.

## Prerequisites
What must exist from prior phases.

## Step N.1: Description
### Files to create/modify
| File | Action |
### Key code/config
(Go structs, YAML, scripts)
### Quality gate
### Testing command
### Common pitfalls
### Progress: [ ] Not started
```

## Conventions

- **Go module**: `github.com/mcp-gateway/mcp-gateway`
- **API group**: `mcp.mcp-gateway.io`
- **API version**: `v1alpha1`
- **Domain**: `mcp-gateway.io`
- **Primary namespace**: `mcp-system`
- **MCP servers namespace**: `mcp-servers`
- **Commits**: All signed off (`git commit -s`) per DCO
- **API style**: gRPC + REST gateway via buf/protobuf
- **Progress markers**: `[ ]` Not started, `[~]` In progress, `[x]` Complete

## File Count by Phase

| Phase | Files Created | Estimated LOC |
|-------|--------------|---------------|
| 0 | ~26 | ~1,500 |
| 1 | ~14 | ~2,500 |
| 2 | ~12 | ~1,400 |
| 3 | ~16 | ~2,850 |
| 4 | ~18 | ~2,000 |
| 5 | ~16 | ~2,200 |
| 6 | ~25+ | ~3,000 |
| 7 | ~18 | ~1,800 |
| **Total** | **~145** | **~17,250** |

## Reference Documents

- [Pitch Report](../pitch-report.md) — Market analysis, architecture, CRD designs
- [Testing Strategy](testing-strategy.md) — Cross-cutting testing approach
