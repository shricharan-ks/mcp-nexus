#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# MCP Gateway -- Backup Script
# ---------------------------------------------------------------------------
# Usage: ./backup.sh [backup-name]
#
# Creates a point-in-time backup of all MCP Gateway resources:
#   - Custom resources (MCPServers, MCPAgents, MCPPolicies, MCPMarketplaceEntries)
#   - Managed secrets
#   - Helm release values
# ---------------------------------------------------------------------------

BACKUP_NAME="${1:-mcp-backup-$(date +%Y%m%d-%H%M%S)}"
BACKUP_DIR="/tmp/mcp-backups/${BACKUP_NAME}"

echo "==> Creating backup: ${BACKUP_NAME}"
mkdir -p "${BACKUP_DIR}"

# -- Custom Resources -------------------------------------------------------
echo "  Exporting custom resources ..."
kubectl get mcpservers,mcpagents,mcppolicies,mcpmarketplaceentries \
  -A -o yaml > "${BACKUP_DIR}/crds.yaml"

# -- Secrets -----------------------------------------------------------------
echo "  Exporting managed secrets ..."
kubectl get secrets -n mcp-system \
  -l app.kubernetes.io/managed-by=mcp-gateway \
  -o yaml > "${BACKUP_DIR}/secrets.yaml"

# -- Helm Values -------------------------------------------------------------
echo "  Exporting Helm values ..."
helm get values mcp-gateway -n mcp-system -o yaml > "${BACKUP_DIR}/helm-values.yaml"

# -- Summary -----------------------------------------------------------------
echo ""
echo "==> Backup complete: ${BACKUP_DIR}"
echo ""
echo "Files:"
ls -lh "${BACKUP_DIR}"
