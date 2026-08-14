#!/usr/bin/env bash
#
# install.sh -- Install MCP Gateway on OpenShift
#
# Usage:
#   ./scripts/install.sh [--namespace mcp-system]
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
NAMESPACE="mcp-system"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REGISTRY="image-registry.openshift-image-registry.svc:5000"
LABEL_KEY="app.kubernetes.io/part-of"
LABEL_VAL="mcp-gateway"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

die() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2
  exit 1
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
      echo "Install MCP Gateway on an OpenShift cluster."
      echo ""
      echo "Options:"
      echo "  --namespace   Kubernetes namespace to install into (default: mcp-system)"
      exit 0
      ;;
    *)
      die "Unknown argument: $1"
      ;;
  esac
done

IMAGE_OPERATOR="${REGISTRY}/${NAMESPACE}/mcp-gateway-operator:latest"
IMAGE_APISERVER="${REGISTRY}/${NAMESPACE}/mcp-gateway-apiserver:latest"
IMAGE_UI="${REGISTRY}/${NAMESPACE}/mcp-gateway-ui:latest"

# ---------------------------------------------------------------------------
# Step 1: Check prerequisites
# ---------------------------------------------------------------------------
log "Checking prerequisites..."

command -v oc >/dev/null 2>&1 || die "'oc' CLI is not installed. Install it from https://mirror.openshift.com/pub/openshift-v4/clients/ocp/"

OC_VERSION=$(oc version --client -o json 2>/dev/null | grep -o '"gitVersion":"[^"]*"' | head -1 || true)
log "  oc version: ${OC_VERSION:-unknown}"

command -v go >/dev/null 2>&1 || die "'go' is not installed. Install Go 1.22+ from https://go.dev/dl/"

GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+')
GO_MINOR=$(echo "${GO_VERSION}" | grep -oE '[0-9]+$')
if [[ "${GO_MINOR}" -lt 22 ]]; then
  die "Go 1.22+ is required (found ${GO_VERSION})"
fi
log "  go version: $(go version)"

command -v helm >/dev/null 2>&1 || log "  WARNING: 'helm' is not installed -- not required for this install method but recommended"

# ---------------------------------------------------------------------------
# Step 2: Verify oc login
# ---------------------------------------------------------------------------
log "Verifying OpenShift login..."
OC_USER=$(oc whoami 2>/dev/null) || die "Not logged in to OpenShift. Run 'oc login' first."
OC_SERVER=$(oc whoami --show-server 2>/dev/null) || true
log "  Logged in as '${OC_USER}' on ${OC_SERVER:-unknown}"

# ---------------------------------------------------------------------------
# Step 3: Create namespace
# ---------------------------------------------------------------------------
log "Ensuring namespace '${NAMESPACE}' exists..."
if oc get namespace "${NAMESPACE}" >/dev/null 2>&1; then
  log "  Namespace '${NAMESPACE}' already exists"
else
  oc create namespace "${NAMESPACE}"
  oc label namespace "${NAMESPACE}" "${LABEL_KEY}=${LABEL_VAL}" --overwrite
  log "  Created namespace '${NAMESPACE}'"
fi

# ---------------------------------------------------------------------------
# Step 4: Install CRDs
# ---------------------------------------------------------------------------
log "Installing CRDs..."
for crd in "${ROOT_DIR}"/config/crd/bases/*.yaml; do
  crd_name=$(basename "${crd}")
  oc apply -f "${crd}"
  log "  Applied ${crd_name}"
done

log "Waiting for CRDs to be established..."
oc wait --for=condition=Established crd/mcpservers.mcp.mcp-gateway.io --timeout=60s
oc wait --for=condition=Established crd/mcpagents.mcp.mcp-gateway.io --timeout=60s
oc wait --for=condition=Established crd/mcppolicies.mcp.mcp-gateway.io --timeout=60s
oc wait --for=condition=Established crd/mcpmarketplaceentries.mcp.mcp-gateway.io --timeout=60s
log "  All CRDs established"

# ---------------------------------------------------------------------------
# Step 5: Build Go binaries (cross-compile for linux/amd64)
# ---------------------------------------------------------------------------
log "Building operator binary..."
cd "${ROOT_DIR}"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/manager-linux ./cmd/operator/
log "  Built bin/manager-linux"

log "Building API server binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/apiserver-linux ./cmd/apiserver/
log "  Built bin/apiserver-linux"

# ---------------------------------------------------------------------------
# Step 6: Create ImageStreams
# ---------------------------------------------------------------------------
log "Creating ImageStreams..."

for name in mcp-gateway-operator mcp-gateway-apiserver mcp-gateway-ui; do
  if oc get imagestream "${name}" -n "${NAMESPACE}" >/dev/null 2>&1; then
    log "  ImageStream '${name}' already exists"
  else
    oc create imagestream "${name}" -n "${NAMESPACE}"
    oc label imagestream "${name}" -n "${NAMESPACE}" "${LABEL_KEY}=${LABEL_VAL}" --overwrite
    log "  Created ImageStream '${name}'"
  fi
done

# ---------------------------------------------------------------------------
# Step 7: Create BuildConfigs
# ---------------------------------------------------------------------------
log "Creating BuildConfigs..."

# Operator BuildConfig
if ! oc get buildconfig mcp-gateway-operator -n "${NAMESPACE}" >/dev/null 2>&1; then
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: mcp-gateway-operator
  labels:
    app.kubernetes.io/part-of: mcp-gateway
spec:
  output:
    to:
      kind: ImageStreamTag
      name: mcp-gateway-operator:latest
  source:
    type: Binary
    binary: {}
  strategy:
    type: Docker
    dockerStrategy:
      dockerfilePath: Dockerfile.openshift
EOF
  log "  Created BuildConfig 'mcp-gateway-operator'"
else
  log "  BuildConfig 'mcp-gateway-operator' already exists"
fi

# API Server BuildConfig
if ! oc get buildconfig mcp-gateway-apiserver -n "${NAMESPACE}" >/dev/null 2>&1; then
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: mcp-gateway-apiserver
  labels:
    app.kubernetes.io/part-of: mcp-gateway
spec:
  output:
    to:
      kind: ImageStreamTag
      name: mcp-gateway-apiserver:latest
  source:
    type: Binary
    binary: {}
  strategy:
    type: Docker
    dockerStrategy:
      dockerfilePath: Dockerfile.apiserver
EOF
  log "  Created BuildConfig 'mcp-gateway-apiserver'"
else
  log "  BuildConfig 'mcp-gateway-apiserver' already exists"
fi

# UI BuildConfig
if ! oc get buildconfig mcp-gateway-ui -n "${NAMESPACE}" >/dev/null 2>&1; then
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: build.openshift.io/v1
kind: BuildConfig
metadata:
  name: mcp-gateway-ui
  labels:
    app.kubernetes.io/part-of: mcp-gateway
spec:
  output:
    to:
      kind: ImageStreamTag
      name: mcp-gateway-ui:latest
  source:
    type: Binary
    binary: {}
  strategy:
    type: Docker
    dockerStrategy:
      dockerfilePath: Dockerfile.ui
EOF
  log "  Created BuildConfig 'mcp-gateway-ui'"
else
  log "  BuildConfig 'mcp-gateway-ui' already exists"
fi

# ---------------------------------------------------------------------------
# Step 8: Run builds
# ---------------------------------------------------------------------------
log "Starting operator build..."
oc start-build mcp-gateway-operator -n "${NAMESPACE}" --from-dir="${ROOT_DIR}" --follow
log "  Operator image built successfully"

log "Starting API server build..."
oc start-build mcp-gateway-apiserver -n "${NAMESPACE}" --from-dir="${ROOT_DIR}" --follow
log "  API server image built successfully"

log "Starting UI build..."
oc start-build mcp-gateway-ui -n "${NAMESPACE}" --from-dir="${ROOT_DIR}" --follow
log "  UI image built successfully"

# ---------------------------------------------------------------------------
# Step 9: Deploy operator
# ---------------------------------------------------------------------------
log "Deploying operator..."

# ServiceAccount
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mcp-gateway-operator
  labels:
    app.kubernetes.io/part-of: mcp-gateway
EOF

# ClusterRoleBinding for the operator
cat <<EOF | oc apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mcp-gateway-operator-admin
  labels:
    app.kubernetes.io/part-of: mcp-gateway
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: mcp-gateway-operator
    namespace: ${NAMESPACE}
EOF

# Operator Deployment
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-gateway-operator
  labels:
    app: mcp-gateway-operator
    app.kubernetes.io/part-of: mcp-gateway
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mcp-gateway-operator
  template:
    metadata:
      labels:
        app: mcp-gateway-operator
        app.kubernetes.io/part-of: mcp-gateway
    spec:
      serviceAccountName: mcp-gateway-operator
      containers:
        - name: manager
          image: ${IMAGE_OPERATOR}
          ports:
            - containerPort: 8080
              name: metrics
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8081
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 10
EOF
log "  Operator deployment created"

# ---------------------------------------------------------------------------
# Step 10: Deploy API server
# ---------------------------------------------------------------------------
log "Deploying API server..."

# ServiceAccount for the API server
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: mcp-gateway-apiserver
  labels:
    app.kubernetes.io/part-of: mcp-gateway
EOF

# ClusterRoleBinding for the API server
cat <<EOF | oc apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: mcp-gateway-apiserver-admin
  labels:
    app.kubernetes.io/part-of: mcp-gateway
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: mcp-gateway-apiserver
    namespace: ${NAMESPACE}
EOF

# API server Deployment
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-gateway-apiserver
  labels:
    app: mcp-gateway-apiserver
    app.kubernetes.io/part-of: mcp-gateway
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mcp-gateway-apiserver
  template:
    metadata:
      labels:
        app: mcp-gateway-apiserver
        app.kubernetes.io/part-of: mcp-gateway
    spec:
      serviceAccountName: mcp-gateway-apiserver
      containers:
        - name: apiserver
          image: ${IMAGE_APISERVER}
          ports:
            - containerPort: 8090
              name: http
          env:
            - name: CORS_ENABLED
              value: "true"
            - name: CORS_ALLOW_ORIGINS
              value: "*"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8090
            initialDelaySeconds: 10
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8090
            initialDelaySeconds: 5
            periodSeconds: 10
EOF

# API server Service
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: v1
kind: Service
metadata:
  name: mcp-gateway-apiserver
  labels:
    app: mcp-gateway-apiserver
    app.kubernetes.io/part-of: mcp-gateway
spec:
  selector:
    app: mcp-gateway-apiserver
  ports:
    - port: 8090
      targetPort: 8090
      protocol: TCP
      name: http
EOF
log "  API server deployment and service created"

# ---------------------------------------------------------------------------
# Step 11: Deploy UI
# ---------------------------------------------------------------------------
log "Deploying UI..."

API_SERVICE_URL="http://mcp-gateway-apiserver.${NAMESPACE}.svc.cluster.local:8090"

cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mcp-gateway-ui
  labels:
    app: mcp-gateway-ui
    app.kubernetes.io/part-of: mcp-gateway
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mcp-gateway-ui
  template:
    metadata:
      labels:
        app: mcp-gateway-ui
        app.kubernetes.io/part-of: mcp-gateway
    spec:
      containers:
        - name: ui
          image: ${IMAGE_UI}
          ports:
            - containerPort: 3000
              name: http
          env:
            - name: NEXT_PUBLIC_API_URL
              value: "${API_SERVICE_URL}"
          resources:
            requests:
              cpu: 50m
              memory: 128Mi
            limits:
              cpu: 200m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /
              port: 3000
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 10
EOF

# UI Service
cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: v1
kind: Service
metadata:
  name: mcp-gateway-ui
  labels:
    app: mcp-gateway-ui
    app.kubernetes.io/part-of: mcp-gateway
spec:
  selector:
    app: mcp-gateway-ui
  ports:
    - port: 3000
      targetPort: 3000
      protocol: TCP
      name: http
EOF
log "  UI deployment and service created"

# ---------------------------------------------------------------------------
# Step 12: Create OpenShift Route for UI
# ---------------------------------------------------------------------------
log "Creating Route for UI..."

cat <<EOF | oc apply -n "${NAMESPACE}" -f -
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: mcp-gateway-ui
  labels:
    app: mcp-gateway-ui
    app.kubernetes.io/part-of: mcp-gateway
spec:
  to:
    kind: Service
    name: mcp-gateway-ui
    weight: 100
  port:
    targetPort: http
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
  wildcardPolicy: None
EOF
log "  Route created with edge TLS termination"

# ---------------------------------------------------------------------------
# Step 13: Wait for deployments
# ---------------------------------------------------------------------------
log "Waiting for deployments to become ready..."

oc rollout status deployment/mcp-gateway-operator  -n "${NAMESPACE}" --timeout=180s
log "  Operator is ready"

oc rollout status deployment/mcp-gateway-apiserver -n "${NAMESPACE}" --timeout=180s
log "  API server is ready"

oc rollout status deployment/mcp-gateway-ui        -n "${NAMESPACE}" --timeout=180s
log "  UI is ready"

# ---------------------------------------------------------------------------
# Step 14: Print summary
# ---------------------------------------------------------------------------
UI_ROUTE=$(oc get route mcp-gateway-ui -n "${NAMESPACE}" -o jsonpath='{.spec.host}' 2>/dev/null || echo "unknown")

echo ""
echo "============================================================"
echo "  MCP Gateway installation complete"
echo "============================================================"
echo ""
echo "  Namespace:       ${NAMESPACE}"
echo "  Cluster:         ${OC_SERVER:-unknown}"
echo "  User:            ${OC_USER}"
echo ""
echo "  Components:"
echo "    Operator       $(oc get deployment mcp-gateway-operator  -n "${NAMESPACE}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo 0)/1 ready"
echo "    API Server     $(oc get deployment mcp-gateway-apiserver -n "${NAMESPACE}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo 0)/1 ready"
echo "    UI             $(oc get deployment mcp-gateway-ui        -n "${NAMESPACE}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo 0)/1 ready"
echo ""
echo "  CRDs installed:"
echo "    - mcpservers.mcp.mcp-gateway.io"
echo "    - mcpagents.mcp.mcp-gateway.io"
echo "    - mcppolicies.mcp.mcp-gateway.io"
echo "    - mcpmarketplaceentries.mcp.mcp-gateway.io"
echo ""
echo "  URLs:"
echo "    UI:            https://${UI_ROUTE}"
echo "    API (internal):${API_SERVICE_URL}"
echo ""
echo "  Next steps:"
echo "    1. Open the UI at https://${UI_ROUTE}"
echo "    2. Deploy an MCP server:  oc apply -f examples/echo-server.yaml"
echo "    3. Register an agent:     oc apply -f examples/agent.yaml"
echo ""
echo "============================================================"
