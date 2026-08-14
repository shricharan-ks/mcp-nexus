# Getting Started with MCP Gateway

This guide walks you through building, deploying, and using MCP Gateway on a
local Kind cluster.

## 1. Prerequisites

Install the following tools before continuing:

```bash
# Go 1.26+
# Download from https://go.dev/dl/ or use your package manager
go version

# Docker 24+
# https://docs.docker.com/get-docker/
docker version

# Kind (Kubernetes in Docker)
go install sigs.k8s.io/kind@latest
kind version

# Helm 3.12+
# https://helm.sh/docs/intro/install/
helm version

# kubectl 1.28+
# https://kubernetes.io/docs/tasks/tools/
kubectl version --client
```

## 2. Clone and build

```bash
git clone https://github.com/mcp-gateway/mcp-gateway.git
cd mcp-gateway

make build
```

Verify the binary was produced:

```bash
ls -l bin/operator
```

## 3. Start a Kind cluster

```bash
make kind-up
```

This creates a Kind cluster named `mcp-gateway` with port mappings for the
gateway and dashboard.

## 4. Deploy the operator

```bash
make dev-deploy
```

This builds the container image, loads it into Kind, installs the CRDs, and
starts the operator in the `mcp-system` namespace.

## 5. Verify the operator is running

```bash
kubectl get pods -n mcp-system
```

Expected output:

```
NAME                              READY   STATUS    RESTARTS   AGE
mcp-gateway-operator-xxx-yyy      1/1     Running   0          30s
```

## 6. Deploy your first MCP server

```bash
kubectl apply -f examples/echo-server.yaml
```

This creates a namespace `mcp-servers` and deploys a simple echo MCP server.

## 7. Check server status

```bash
kubectl get mcps -n mcp-servers
```

Expected output:

```
NAME          PHASE     READY   IMAGE                                         TRANSPORT         AGE
echo-server   Running   1       ghcr.io/mcp-gateway/echo-mcp-server:latest   streamable-http   45s
```

You can inspect the full status with:

```bash
kubectl describe mcps echo-server -n mcp-servers
```

## 8. Create an agent

Apply an MCPAgent to register an AI agent that can access the echo server:

```bash
cat <<EOF | kubectl apply -f -
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
EOF
```

Verify:

```bash
kubectl get mcpa -n mcp-servers
```

## 9. Create a policy

Apply an MCPPolicy to restrict which tools the agent can call:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: mcp.mcp-gateway.io/v1alpha1
kind: MCPPolicy
metadata:
  name: echo-readonly
  namespace: mcp-servers
spec:
  rules:
    - effect: ALLOW
      principals:
        agentRefs:
          - name: my-agent
      actions:
        - tools/call
      resources:
        serverRef:
          name: echo-server
        tools:
          - echo
          - ping
    - effect: DENY
      principals:
        agentRefs:
          - name: my-agent
      actions:
        - tools/call
      resources:
        serverRef:
          name: echo-server
        tools:
          - "*"
EOF
```

Verify the policy synced to Cerbos:

```bash
kubectl get mcpp -n mcp-servers
```

## 10. Clean up

```bash
# Delete the example resources
kubectl delete -f examples/echo-server.yaml

# Tear down the Kind cluster
make kind-down
```

## Next steps

- Read the [User Guide](user-guide.md) for full coverage of all CRDs and features.
- Review the [Architecture](architecture.md) doc for system design details.
- See the [Security](security.md) doc for authentication, authorization, and hardening.
- Browse the `examples/` directory for more sample manifests.
