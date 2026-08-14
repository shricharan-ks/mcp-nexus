#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# MCP Gateway -- Restore Script
# ---------------------------------------------------------------------------
# Usage: ./restore.sh <backup-directory>
#
# Restores MCP Gateway from a backup created by backup.sh.
# ---------------------------------------------------------------------------

if [ $# -lt 1 ]; then
  echo "Usage: $0 <backup-directory>"
  exit 1
fi

BACKUP_DIR="$1"

# -- Verify backup files ----------------------------------------------------
echo "==> Verifying backup at ${BACKUP_DIR} ..."

for f in crds.yaml secrets.yaml helm-values.yaml; do
  if [ ! -f "${BACKUP_DIR}/${f}" ]; then
    echo "ERROR: Missing required file: ${BACKUP_DIR}/${f}"
    exit 1
  fi
done

echo "  All required files present."

# -- Restore CRDs -----------------------------------------------------------
echo "==> Restoring custom resources ..."
kubectl apply -f "${BACKUP_DIR}/crds.yaml"

# -- Restore Secrets --------------------------------------------------------
echo "==> Restoring secrets ..."
kubectl apply -f "${BACKUP_DIR}/secrets.yaml"

# -- Reinstall Helm release -------------------------------------------------
echo "==> Reinstalling Helm release ..."
helm upgrade --install mcp-gateway deploy/helm/mcp-gateway \
  -n mcp-system \
  -f "${BACKUP_DIR}/helm-values.yaml" \
  --wait

# -- Verify operator pod ----------------------------------------------------
echo "==> Verifying operator pod ..."
kubectl wait --for=condition=Ready pod \
  -l app.kubernetes.io/name=mcp-gateway \
  -n mcp-system \
  --timeout=120s

echo ""
echo "==> Restore complete. Listing restored resources:"
echo ""
echo "--- MCPServers ---"
kubectl get mcpservers -A 2>/dev/null || echo "  (none)"
echo ""
echo "--- MCPAgents ---"
kubectl get mcpagents -A 2>/dev/null || echo "  (none)"
echo ""
echo "--- MCPPolicies ---"
kubectl get mcppolicies -A 2>/dev/null || echo "  (none)"
echo ""
echo "--- MCPMarketplaceEntries ---"
kubectl get mcpmarketplaceentries -A 2>/dev/null || echo "  (none)"
