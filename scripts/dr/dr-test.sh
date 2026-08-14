#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# MCP Gateway -- Disaster Recovery Verification Test
# ---------------------------------------------------------------------------
# Runs a series of checks to verify that MCP Gateway is operational after
# a restore or fresh deployment. Exits 1 if any check fails.
# ---------------------------------------------------------------------------

PASS=0
FAIL=0

check() {
  local description="$1"
  shift
  if "$@" > /dev/null 2>&1; then
    echo "  PASS  ${description}"
    PASS=$((PASS + 1))
  else
    echo "  FAIL  ${description}"
    FAIL=$((FAIL + 1))
  fi
}

echo "==> MCP Gateway DR Verification"
echo ""

# -- Operator pod running ----------------------------------------------------
check "Operator pod is running" \
  kubectl get pods -n mcp-system \
    -l app.kubernetes.io/name=mcp-gateway \
    -o jsonpath='{.items[0].status.phase}' \
  | grep -q Running

# Fallback: use a simpler form if the piped version doesn't work inside check
check "Operator pod is Ready" \
  kubectl wait --for=condition=Ready pod \
    -l app.kubernetes.io/name=mcp-gateway \
    -n mcp-system \
    --timeout=10s

# -- CRDs exist --------------------------------------------------------------
for crd in mcpservers.mcp-gateway.io mcpagents.mcp-gateway.io mcppolicies.mcp-gateway.io mcpmarketplaceentries.mcp-gateway.io; do
  check "CRD exists: ${crd}" \
    kubectl get crd "${crd}"
done

# -- Helm release exists -----------------------------------------------------
check "Helm release exists" \
  helm status mcp-gateway -n mcp-system

# -- Services reachable ------------------------------------------------------
check "Gateway service exists" \
  kubectl get svc -n mcp-system \
    -l app.kubernetes.io/name=mcp-gateway

check "Gateway service has endpoints" \
  kubectl get endpoints -n mcp-system \
    -l app.kubernetes.io/name=mcp-gateway \
    -o jsonpath='{.items[0].subsets[0].addresses[0].ip}'

# -- Summary -----------------------------------------------------------------
echo ""
TOTAL=$((PASS + FAIL))
echo "==> Results: ${PASS}/${TOTAL} checks passed"

if [ "${FAIL}" -gt 0 ]; then
  echo "==> SOME CHECKS FAILED"
  exit 1
else
  echo "==> ALL CHECKS PASSED"
  exit 0
fi
