#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CLUSTER_NAME="${1:-mcp-gateway}"
CONFIG_FILE="${2:-${SCRIPT_DIR}/cluster-config.yaml}"
KIND_IMAGE="${3:-kindest/node:v1.32.0}"

echo "==> Checking for existing kind cluster '${CLUSTER_NAME}'..."
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo "    Cluster '${CLUSTER_NAME}' already exists, skipping creation."
else
  echo "==> Creating kind cluster '${CLUSTER_NAME}'..."
  kind create cluster \
    --name "${CLUSTER_NAME}" \
    --config "${CONFIG_FILE}" \
    --image "${KIND_IMAGE}"
  echo "    Cluster '${CLUSTER_NAME}' created successfully."
fi

echo "==> Setting kubectl context to kind-${CLUSTER_NAME}..."
kubectl cluster-info --context "kind-${CLUSTER_NAME}"

echo "==> Creating mcp-system namespace (if not exists)..."
kubectl create namespace mcp-system --dry-run=client -o yaml | kubectl apply -f -

echo "==> Cluster '${CLUSTER_NAME}' is ready."
