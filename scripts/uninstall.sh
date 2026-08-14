#!/usr/bin/env bash
#
# uninstall.sh -- Remove MCP Gateway from OpenShift
#
# Usage:
#   ./scripts/uninstall.sh [--namespace mcp-system]
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
NAMESPACE="mcp-system"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

warn() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $*"
}

safe_delete() {
  local resource="$1"
  local name="$2"
  local ns_flag="${3:-}"

  if oc get "${resource}" "${name}" ${ns_flag} >/dev/null 2>&1; then
    oc delete "${resource}" "${name}" ${ns_flag} --ignore-not-found
    log "  Deleted ${resource}/${name}"
  else
    log "  ${resource}/${name} not found -- skipping"
  fi
}

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --namespace=*)
      NAMESPACE="${1#*=}"
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--namespace <namespace>]"
      echo ""
      echo "Remove MCP Gateway from an OpenShift cluster."
      echo ""
      echo "Options:"
      echo "  --namespace   Kubernetes namespace to uninstall from (default: mcp-system)"
      exit 0
      ;;
    *)
      echo "ERROR: Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

# ---------------------------------------------------------------------------
# Confirmation
# ---------------------------------------------------------------------------
echo ""
echo "This will remove MCP Gateway from namespace '${NAMESPACE}'."
echo "The following resources will be deleted:"
echo "  - Route: mcp-gateway-ui"
echo "  - Deployments: mcp-gateway-ui, mcp-gateway-apiserver, mcp-gateway-operator"
echo "  - Services: mcp-gateway-ui, mcp-gateway-apiserver"
echo "  - ServiceAccounts: mcp-gateway-operator, mcp-gateway-apiserver"
echo "  - ClusterRoleBindings: mcp-gateway-operator-admin, mcp-gateway-apiserver-admin"
echo "  - All MCPServer, MCPAgent, MCPPolicy, MCPMarketplaceEntry CRs (all namespaces)"
echo "  - CRDs: mcpservers, mcpagents, mcppolicies, mcpmarketplaceentries"
echo "  - BuildConfigs and ImageStreams"
echo ""
read -r -p "Type 'yes' to continue: " CONFIRM

if [[ "${CONFIRM}" != "yes" ]]; then
  echo "Aborted."
  exit 0
fi

echo ""

# ---------------------------------------------------------------------------
# Step 1: Delete Route
# ---------------------------------------------------------------------------
log "Removing Route..."
safe_delete route mcp-gateway-ui "-n ${NAMESPACE}"

# ---------------------------------------------------------------------------
# Step 2: Delete UI Deployment + Service
# ---------------------------------------------------------------------------
log "Removing UI..."
safe_delete deployment mcp-gateway-ui "-n ${NAMESPACE}"
safe_delete service mcp-gateway-ui "-n ${NAMESPACE}"

# ---------------------------------------------------------------------------
# Step 3: Delete API server Deployment + Service
# ---------------------------------------------------------------------------
log "Removing API server..."
safe_delete deployment mcp-gateway-apiserver "-n ${NAMESPACE}"
safe_delete service mcp-gateway-apiserver "-n ${NAMESPACE}"

# ---------------------------------------------------------------------------
# Step 4: Delete Operator Deployment + ServiceAccounts
# ---------------------------------------------------------------------------
log "Removing Operator..."
safe_delete deployment mcp-gateway-operator "-n ${NAMESPACE}"
safe_delete serviceaccount mcp-gateway-operator "-n ${NAMESPACE}"
safe_delete serviceaccount mcp-gateway-apiserver "-n ${NAMESPACE}"

# ---------------------------------------------------------------------------
# Step 5: Delete ClusterRoleBindings
# ---------------------------------------------------------------------------
log "Removing ClusterRoleBindings..."
safe_delete clusterrolebinding mcp-gateway-operator-admin
safe_delete clusterrolebinding mcp-gateway-apiserver-admin

# ---------------------------------------------------------------------------
# Step 6: Delete all custom resources in all namespaces
# ---------------------------------------------------------------------------
log "Removing all MCP custom resources across all namespaces..."

for crd_kind in mcpservers mcpagents mcppolicies mcpmarketplaceentries; do
  if oc get crd "${crd_kind}.mcp.mcp-gateway.io" >/dev/null 2>&1; then
    CR_COUNT=$(oc get "${crd_kind}.mcp.mcp-gateway.io" --all-namespaces --no-headers 2>/dev/null | wc -l | tr -d ' ')
    if [[ "${CR_COUNT}" -gt 0 ]]; then
      oc delete "${crd_kind}.mcp.mcp-gateway.io" --all --all-namespaces 2>/dev/null || true
      log "  Deleted ${CR_COUNT} ${crd_kind} resource(s)"
    else
      log "  No ${crd_kind} resources found"
    fi
  else
    log "  CRD ${crd_kind}.mcp.mcp-gateway.io not found -- skipping"
  fi
done

# ---------------------------------------------------------------------------
# Step 7: Delete CRDs
# ---------------------------------------------------------------------------
log "Removing CRDs..."
for crd in mcpservers.mcp.mcp-gateway.io mcpagents.mcp.mcp-gateway.io mcppolicies.mcp.mcp-gateway.io mcpmarketplaceentries.mcp.mcp-gateway.io; do
  safe_delete crd "${crd}"
done

# ---------------------------------------------------------------------------
# Step 8: Delete BuildConfigs and ImageStreams
# ---------------------------------------------------------------------------
log "Removing BuildConfigs..."
for bc in mcp-gateway-operator mcp-gateway-apiserver mcp-gateway-ui; do
  safe_delete buildconfig "${bc}" "-n ${NAMESPACE}"
done

log "Removing ImageStreams..."
for is in mcp-gateway-operator mcp-gateway-apiserver mcp-gateway-ui; do
  safe_delete imagestream "${is}" "-n ${NAMESPACE}"
done

# ---------------------------------------------------------------------------
# Step 9: Optionally delete namespace
# ---------------------------------------------------------------------------
echo ""
read -r -p "Delete namespace '${NAMESPACE}'? (yes/no): " DELETE_NS

if [[ "${DELETE_NS}" == "yes" ]]; then
  oc delete namespace "${NAMESPACE}" --ignore-not-found
  log "Namespace '${NAMESPACE}' deleted"
else
  log "Namespace '${NAMESPACE}' preserved"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo "  MCP Gateway uninstall complete"
echo "============================================================"
echo ""
echo "  Removed from namespace: ${NAMESPACE}"
echo ""
echo "  Deleted:"
echo "    - Route (mcp-gateway-ui)"
echo "    - Deployments (operator, apiserver, ui)"
echo "    - Services (apiserver, ui)"
echo "    - ServiceAccounts (operator, apiserver)"
echo "    - ClusterRoleBindings (operator-admin, apiserver-admin)"
echo "    - Custom Resources (all namespaces)"
echo "    - CRDs (mcpservers, mcpagents, mcppolicies, mcpmarketplaceentries)"
echo "    - BuildConfigs and ImageStreams"
if [[ "${DELETE_NS}" == "yes" ]]; then
  echo "    - Namespace '${NAMESPACE}'"
fi
echo ""
echo "============================================================"
