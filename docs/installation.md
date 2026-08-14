# MCP Gateway -- OpenShift Installation Guide

This guide covers installing MCP Gateway on an OpenShift cluster. MCP Gateway is a Kubernetes-native control plane for managing Model Context Protocol (MCP) servers, providing an operator, REST API, and web dashboard.

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| OpenShift cluster | 4.14+ | Cluster-admin access required |
| `oc` CLI | Matching cluster version | Must be logged in (`oc login`) |
| Go | 1.22+ | For cross-compiling the operator and API server |
| Node.js | 20+ | For the UI build (handled inside the container build) |
| Git | Any recent version | To clone the repository |

Verify your prerequisites:

```bash
oc version --client
oc whoami
go version
```

## Quick Install

```bash
git clone https://github.com/mcp-gateway/mcp-gateway.git
cd mcp-gateway
./scripts/install.sh --namespace scharan-test
```

The install script will:

1. Verify prerequisites and OpenShift login
2. Create the target namespace
3. Install all four CRDs
4. Cross-compile the Go binaries for linux/amd64
5. Build container images using OpenShift binary builds
6. Deploy the operator, API server, and UI
7. Create an OpenShift Route with edge TLS for the UI
8. Wait for all deployments to become ready
9. Print the UI URL and summary

## What Gets Installed

| Component | Type | Description |
|---|---|---|
| Operator | Deployment | Watches MCPServer/MCPAgent/MCPPolicy CRs and reconciles child resources |
| API Server | Deployment + Service | REST API on port 8090 for the UI and external integrations |
| UI | Deployment + Service + Route | Next.js dashboard on port 3000, exposed via edge-TLS Route |
| MCPServer CRD | CustomResourceDefinition | Defines MCP server instances |
| MCPAgent CRD | CustomResourceDefinition | Defines agents with server access, rate limits, and quotas |
| MCPPolicy CRD | CustomResourceDefinition | Defines authorization policies for MCP tool access |
| MCPMarketplaceEntry CRD | CustomResourceDefinition | Defines marketplace catalog entries for shared MCP servers |

All resources are labeled with `app.kubernetes.io/part-of=mcp-gateway`.

## Manual Installation

If you prefer to install step-by-step instead of using the script:

### 1. Create the namespace

```bash
NAMESPACE=mcp-system
oc create namespace $NAMESPACE
```

### 2. Install CRDs

```bash
oc apply -f config/crd/bases/mcp.mcp-gateway.io_mcpservers.yaml
oc apply -f config/crd/bases/mcp.mcp-gateway.io_mcpagents.yaml
oc apply -f config/crd/bases/mcp.mcp-gateway.io_mcppolicies.yaml
oc apply -f config/crd/bases/mcp.mcp-gateway.io_mcpmarketplaceentries.yaml
```

Verify CRDs are established:

```bash
oc get crd | grep mcp-gateway
```

### 3. Build the binaries

Cross-compile for linux/amd64:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/manager-linux ./cmd/operator/
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/apiserver-linux ./cmd/apiserver/
```

### 4. Build container images

Create ImageStreams:

```bash
oc create imagestream mcp-gateway-operator -n $NAMESPACE
oc create imagestream mcp-gateway-apiserver -n $NAMESPACE
oc create imagestream mcp-gateway-ui -n $NAMESPACE
```

Create BuildConfigs for binary builds:

```bash
# Operator
oc new-build --name=mcp-gateway-operator \
  --binary \
  --strategy=docker \
  --docker-image="registry.access.redhat.com/ubi9-minimal:latest" \
  --to=mcp-gateway-operator:latest \
  -n $NAMESPACE

# API server
oc new-build --name=mcp-gateway-apiserver \
  --binary \
  --strategy=docker \
  --to=mcp-gateway-apiserver:latest \
  -n $NAMESPACE

# UI
oc new-build --name=mcp-gateway-ui \
  --binary \
  --strategy=docker \
  --to=mcp-gateway-ui:latest \
  -n $NAMESPACE
```

Run the builds:

```bash
oc start-build mcp-gateway-operator --from-dir=. --follow -n $NAMESPACE
oc start-build mcp-gateway-apiserver --from-dir=. --follow -n $NAMESPACE
oc start-build mcp-gateway-ui --from-dir=. --follow -n $NAMESPACE
```

### 5. Set up RBAC

```bash
REGISTRY=image-registry.openshift-image-registry.svc:5000

# Operator ServiceAccount and RBAC
oc create serviceaccount mcp-gateway-operator -n $NAMESPACE
oc create clusterrolebinding mcp-gateway-operator-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=$NAMESPACE:mcp-gateway-operator

# API server ServiceAccount and RBAC
oc create serviceaccount mcp-gateway-apiserver -n $NAMESPACE
oc create clusterrolebinding mcp-gateway-apiserver-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=$NAMESPACE:mcp-gateway-apiserver
```

### 6. Deploy the operator

```bash
IMAGE_OPERATOR=$REGISTRY/$NAMESPACE/mcp-gateway-operator:latest

oc create deployment mcp-gateway-operator \
  --image=$IMAGE_OPERATOR \
  -n $NAMESPACE

oc set serviceaccount deployment/mcp-gateway-operator mcp-gateway-operator -n $NAMESPACE
```

### 7. Deploy the API server

```bash
IMAGE_APISERVER=$REGISTRY/$NAMESPACE/mcp-gateway-apiserver:latest

oc create deployment mcp-gateway-apiserver \
  --image=$IMAGE_APISERVER \
  --port=8090 \
  -n $NAMESPACE

oc set serviceaccount deployment/mcp-gateway-apiserver mcp-gateway-apiserver -n $NAMESPACE
oc set env deployment/mcp-gateway-apiserver CORS_ENABLED=true CORS_ALLOW_ORIGINS="*" -n $NAMESPACE
oc expose deployment/mcp-gateway-apiserver --port=8090 -n $NAMESPACE
```

### 8. Deploy the UI

```bash
IMAGE_UI=$REGISTRY/$NAMESPACE/mcp-gateway-ui:latest
API_URL=http://mcp-gateway-apiserver.$NAMESPACE.svc.cluster.local:8090

oc create deployment mcp-gateway-ui \
  --image=$IMAGE_UI \
  --port=3000 \
  -n $NAMESPACE

oc set env deployment/mcp-gateway-ui NEXT_PUBLIC_API_URL=$API_URL -n $NAMESPACE
oc expose deployment/mcp-gateway-ui --port=3000 -n $NAMESPACE
```

### 9. Create the Route

```bash
oc create route edge mcp-gateway-ui \
  --service=mcp-gateway-ui \
  --port=3000 \
  --insecure-policy=Redirect \
  -n $NAMESPACE
```

## Accessing the UI

Get the Route URL:

```bash
oc get route mcp-gateway-ui -n $NAMESPACE
```

The UI will be available at `https://<route-host>`. The Route uses edge TLS termination, so the connection from the browser to the OpenShift router is encrypted.

## Deploying Your First MCP Server

Once the gateway is running, deploy an MCP server by creating an MCPServer custom resource.

### Example: Echo Server

```bash
oc apply -f - <<EOF
apiVersion: mcp.mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: echo-server
  namespace: mcp-servers
spec:
  source:
    image: ghcr.io/mcp-gateway/echo-mcp-server:latest
    port: 8080
    healthCheck:
      path: /health
      periodSeconds: 10
  protocol:
    transport: streamable-http
    endpoint: /mcp
  resources:
    requests:
      cpu: "50m"
      memory: "64Mi"
    limits:
      cpu: "200m"
      memory: "128Mi"
EOF
```

Or apply the included example:

```bash
oc apply -f examples/echo-server.yaml
```

Verify it is running:

```bash
oc get mcpservers -A
```

The operator will create the underlying Deployment and Service, and the server will appear in the UI dashboard.

## Creating an Agent

Agents are registered consumers of MCP servers. Create one with an MCPAgent custom resource:

```bash
oc apply -f - <<EOF
apiVersion: mcp.mcp-gateway.io/v1alpha1
kind: MCPAgent
metadata:
  name: my-agent
  namespace: mcp-servers
spec:
  identity:
    oidcClientId: agent-my-agent
  serverAccess:
    - serverRef:
        name: echo-server
  rateLimits:
    global:
      requestsPerMinute: 60
  quota:
    maxConcurrentConnections: 5
    maxMonthlyToolCalls: 10000
EOF
```

Check the agent status:

```bash
oc get mcpagents -A
```

The agent will progress through `Pending` -> `Registering` -> `Active` phases. Once active, the agent's OIDC client ID can be used to authenticate against MCP servers through the gateway.

## Uninstalling

```bash
./scripts/uninstall.sh --namespace scharan-test
```

The script will:

1. Ask for confirmation
2. Delete all gateway components (Route, Deployments, Services, ServiceAccounts)
3. Remove ClusterRoleBindings
4. Delete all MCPServer, MCPAgent, MCPPolicy, and MCPMarketplaceEntry CRs across all namespaces
5. Remove the CRDs
6. Clean up BuildConfigs and ImageStreams
7. Optionally delete the namespace

## Troubleshooting

### ImagePullBackOff

The most common cause is that the internal registry image path does not match the namespace. Verify the image reference:

```bash
oc get deployment mcp-gateway-operator -n $NAMESPACE -o jsonpath='{.spec.template.spec.containers[0].image}'
```

It should follow the pattern `image-registry.openshift-image-registry.svc:5000/<namespace>/mcp-gateway-operator:latest`. Check that the ImageStream exists and has a tag:

```bash
oc get imagestream mcp-gateway-operator -n $NAMESPACE
oc get istag mcp-gateway-operator:latest -n $NAMESPACE
```

### Build failures

If `oc start-build` fails, check the build logs:

```bash
oc logs -f buildconfig/mcp-gateway-operator -n $NAMESPACE
```

Common causes:
- Missing binary (`bin/manager-linux`): Re-run the `go build` step.
- Dockerfile not found: Ensure you are running the build from the repository root.

### CRD conflicts

If CRDs already exist from a previous installation:

```bash
oc get crd | grep mcp-gateway
```

Either delete the old CRDs first (`oc delete crd <name>`) or use `oc apply` which will update them in place.

### RBAC / permission errors

The operator and API server both need cluster-wide access to watch and manage resources. Verify the ClusterRoleBindings:

```bash
oc get clusterrolebinding | grep mcp-gateway
```

If they are missing, recreate them:

```bash
oc create clusterrolebinding mcp-gateway-operator-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=$NAMESPACE:mcp-gateway-operator
```

### Operator not reconciling

Check the operator logs:

```bash
oc logs deployment/mcp-gateway-operator -n $NAMESPACE
```

Look for errors related to:
- Missing RBAC permissions
- CRDs not registered
- Failed health checks

### UI shows "connection refused"

The UI communicates with the API server via the internal service URL. Verify the API server is running and the service resolves:

```bash
oc get service mcp-gateway-apiserver -n $NAMESPACE
oc get endpoints mcp-gateway-apiserver -n $NAMESPACE
```

Check the environment variable on the UI deployment:

```bash
oc set env deployment/mcp-gateway-ui --list -n $NAMESPACE
```

The `NEXT_PUBLIC_API_URL` should point to `http://mcp-gateway-apiserver.<namespace>.svc.cluster.local:8090`.
