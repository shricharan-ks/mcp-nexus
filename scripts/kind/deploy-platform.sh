#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CLUSTER_NAME="${1:-mcp-gateway}"
HELM_RELEASE="${2:-mcp-gateway}"
NAMESPACE="${3:-mcp-system}"
CHART_PATH="${4:-${PROJECT_ROOT}/deploy/helm/mcp-gateway}"
IMG="${5:-ghcr.io/mcp-gateway/mcp-gateway-operator:dev}"

# Extract repository and tag from the image
IMAGE_REPO="${IMG%:*}"
IMAGE_TAG="${IMG##*:}"

echo "==> Verifying kind cluster '${CLUSTER_NAME}' exists..."
if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo "ERROR: Cluster '${CLUSTER_NAME}' does not exist. Run create-cluster.sh first."
  exit 1
fi

echo "==> Setting kubectl context to kind-${CLUSTER_NAME}..."
kubectl config use-context "kind-${CLUSTER_NAME}"

# Apply CRDs if they exist
CRD_DIR="${PROJECT_ROOT}/config/crd/bases"
if [ -d "${CRD_DIR}" ] && ls "${CRD_DIR}"/*.yaml 1>/dev/null 2>&1; then
  echo "==> Applying CRDs from ${CRD_DIR}..."
  kubectl apply -f "${CRD_DIR}/"
else
  echo "==> No CRDs found in ${CRD_DIR}, skipping."
fi

echo "==> Loading image '${IMG}' into kind cluster '${CLUSTER_NAME}'..."
kind load docker-image "${IMG}" --name "${CLUSTER_NAME}"

echo "==> Deploying Helm release '${HELM_RELEASE}' to namespace '${NAMESPACE}'..."
helm upgrade --install "${HELM_RELEASE}" "${CHART_PATH}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --set image.repository="${IMAGE_REPO}" \
  --set image.tag="${IMAGE_TAG}" \
  --wait \
  --timeout 120s

echo "==> Waiting for operator pod to be ready..."
kubectl wait --for=condition=Ready pods \
  --selector="app.kubernetes.io/name=mcp-gateway" \
  --namespace "${NAMESPACE}" \
  --timeout=120s

echo "==> Platform deployed successfully."
kubectl get pods -n "${NAMESPACE}"
