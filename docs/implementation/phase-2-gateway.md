# Phase 2: Gateway Integration (Weeks 6-9)

**Goal:** MCP traffic flows through Envoy AI Gateway with JWT authentication.

By the end of this phase every MCP request entering the cluster is TLS-terminated,
JWT-validated, and rate-limited by Envoy Gateway. The operator automatically
generates HTTPRoutes so that adding a new `MCPServer` CR is all an admin needs to
do.

---

## 2.1 Add Envoy Gateway

### Helm Dependency

Add Envoy Gateway as a subchart in `deploy/charts/mcp-gateway/Chart.yaml`:

```yaml
dependencies:
  - name: gateway-helm
    version: "1.2.1"
    repository: "oci://docker.io/envoyproxy"
    alias: envoy-gateway
    condition: envoy-gateway.enabled
```

### Values

`deploy/charts/mcp-gateway/values.yaml` additions:

```yaml
envoy-gateway:
  enabled: true

  config:
    envoyGateway:
      gateway:
        controllerName: gateway.envoyproxy.io/mcp-gateway-controller
      logging:
        level:
          default: info
      provider:
        type: Kubernetes

  certManager:
    enabled: true            # install cert-manager CRDs & controller
    issuer:
      name: mcp-selfsigned   # switch to letsencrypt-prod for prod
      kind: ClusterIssuer

gateway:
  name: mcp-gateway
  namespace: mcp-system
  className: mcp-gateway
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: mcp-gateway-tls
    - name: http-redirect
      protocol: HTTP
      port: 80
      # EnvoyPatchPolicy redirects all HTTP -> HTTPS
```

### GatewayClass & Gateway Manifests

Applied by the Helm chart via `templates/`:

```yaml
# templates/gatewayclass.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: mcp-gateway
spec:
  controllerName: gateway.envoyproxy.io/mcp-gateway-controller

---
# templates/gateway.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: {{ .Values.gateway.name }}
  namespace: {{ .Values.gateway.namespace }}
  annotations:
    cert-manager.io/cluster-issuer: {{ .Values.envoy-gateway.certManager.issuer.name }}
spec:
  gatewayClassName: {{ .Values.gateway.className }}
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: mcp-gateway-tls
    - name: http-redirect
      protocol: HTTP
      port: 80
```

### cert-manager ClusterIssuer

```yaml
# templates/cluster-issuer.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: mcp-selfsigned
spec:
  selfSigned: {}
```

For production, replace with an ACME issuer pointing at Let's Encrypt.

---

## 2.2 HTTPRoute Generation

The operator watches `MCPServer` CRs and creates one `HTTPRoute` per server.
Route logic lives in a dedicated package so it can be unit-tested independently.

### Route Pattern

Every MCP server is exposed at `/<server-name>/mcp`. Envoy matches on path
prefix and optionally on MCP-specific headers (`Mcp-Method`, `Mcp-Name`) to
enable per-tool routing in later phases.

### Go Code: `internal/envoy/route.go`

```go
package envoy

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	mcpv1alpha1 "github.com/mcp-gateway/api/v1alpha1"
)

const (
	GatewayName = "mcp-gateway"
	GatewayNS   = "mcp-system"
	ListenerHTTPS = "https"
)

// BuildHTTPRoute creates a Gateway API HTTPRoute for the given MCPServer.
// The route is owned by the MCPServer so garbage collection works automatically.
func BuildHTTPRoute(server *mcpv1alpha1.MCPServer) *gatewayv1.HTTPRoute {
	pathPrefix := fmt.Sprintf("/%s/mcp", server.Name)
	pathType := gatewayv1.PathMatchPathPrefix

	gwNS := gatewayv1.Namespace(GatewayNS)
	listenerName := gatewayv1.SectionName(ListenerHTTPS)

	port := gatewayv1.PortNumber(server.Spec.Port)

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("mcp-%s", server.Name),
			Namespace: server.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "mcp-gateway-operator",
				"mcp-gateway.io/server":        server.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(server, mcpv1alpha1.GroupVersion.WithKind("MCPServer")),
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:        gatewayv1.ObjectName(GatewayName),
						Namespace:   &gwNS,
						SectionName: &listenerName,
					},
				},
			},
			Rules: buildRules(server, pathPrefix, pathType, port),
		},
	}
	return route
}

func buildRules(
	server *mcpv1alpha1.MCPServer,
	pathPrefix string,
	pathType gatewayv1.PathMatchType,
	port gatewayv1.PortNumber,
) []gatewayv1.HTTPRouteRule {
	rules := []gatewayv1.HTTPRouteRule{
		// Catch-all rule for the server prefix
		{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &pathType,
						Value: &pathPrefix,
					},
				},
			},
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(server.Spec.ServiceName),
							Port: &port,
						},
					},
				},
			},
		},
	}

	// Per-tool header-match rules (enables fine-grained policy later)
	for _, tool := range server.Spec.Tools {
		toolPath := pathPrefix
		headerType := gatewayv1.HeaderMatchExact
		rules = append(rules, gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &pathType,
						Value: &toolPath,
					},
					Headers: []gatewayv1.HTTPHeaderMatch{
						{
							Type:  &headerType,
							Name:  "Mcp-Method",
							Value: "tools/call",
						},
						{
							Type:  &headerType,
							Name:  "Mcp-Name",
							Value: tool.Name,
						},
					},
				},
			},
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(server.Spec.ServiceName),
							Port: &port,
						},
					},
				},
			},
		})
	}

	return rules
}
```

### Reconciler Integration

In `MCPServerReconciler.Reconcile`, after ensuring the backing Deployment and
Service exist, call the route builder:

```go
desired := envoy.BuildHTTPRoute(mcpServer)
found := &gatewayv1.HTTPRoute{}
err := r.Get(ctx, client.ObjectKeyFromObject(desired), found)
if apierrors.IsNotFound(err) {
    return ctrl.Result{}, r.Create(ctx, desired)
}
// update if spec drift detected
if !equality.Semantic.DeepEqual(found.Spec, desired.Spec) {
    found.Spec = desired.Spec
    return ctrl.Result{}, r.Update(ctx, found)
}
```

---

## 2.3 Keycloak Deployment

### Helm Subchart

Add Bitnami Keycloak as a dependency:

```yaml
# Chart.yaml
dependencies:
  - name: keycloak
    version: "24.0.5"
    repository: "oci://registry-1.docker.io/bitnamicharts"
    condition: keycloak.enabled
```

### Values

```yaml
keycloak:
  enabled: true
  auth:
    adminUser: admin
    adminPassword: ""          # set via --set or sealed-secret
  postgresql:
    enabled: true
  extraVolumes:
    - name: realm-config
      configMap:
        name: mcp-keycloak-realm
  extraVolumeMounts:
    - name: realm-config
      mountPath: /opt/bitnami/keycloak/data/import
  extraStartupArgs: "--import-realm"
```

### Realm Configuration

```yaml
# templates/keycloak-realm-cm.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-keycloak-realm
  namespace: mcp-system
data:
  mcp-gateway-realm.json: |
    {
      "realm": "mcp-gateway",
      "enabled": true,
      "sslRequired": "external",
      "roles": {
        "realm": [
          { "name": "mcp-agent",   "description": "Can invoke MCP tools" },
          { "name": "mcp-admin",   "description": "Can manage MCP resources" }
        ]
      },
      "clients": [
        {
          "clientId": "mcp-gateway-api",
          "enabled": true,
          "protocol": "openid-connect",
          "publicClient": false,
          "serviceAccountsEnabled": true,
          "directAccessGrantsEnabled": true,
          "standardFlowEnabled": false,
          "protocolMappers": [
            {
              "name": "agent-id-mapper",
              "protocol": "openid-connect",
              "protocolMapper": "oidc-usermodel-attribute-mapper",
              "config": {
                "user.attribute": "agent_id",
                "claim.name": "agent_id",
                "id.token.claim": "true",
                "access.token.claim": "true",
                "jsonType.label": "String"
              }
            }
          ]
        }
      ],
      "defaultDefaultClientScopes": ["openid", "profile"],
      "components": {
        "org.keycloak.keys.KeyProviders": [
          {
            "name": "rsa-generated",
            "providerId": "rsa-generated",
            "config": { "keySize": ["2048"], "priority": ["100"] }
          }
        ]
      }
    }
```

### Bootstrap Script

`scripts/bootstrap-keycloak.sh` -- run once after first install to create the
initial admin agent and retrieve its credentials:

```bash
#!/usr/bin/env bash
set -euo pipefail

KEYCLOAK_URL="${KEYCLOAK_URL:-https://keycloak.mcp-system.svc.cluster.local}"
REALM="mcp-gateway"
ADMIN_USER="${KC_ADMIN_USER:-admin}"
ADMIN_PASS="${KC_ADMIN_PASS:?KC_ADMIN_PASS must be set}"

# Obtain admin token
ADMIN_TOKEN=$(curl -sf -X POST \
  "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
  -d "grant_type=password" \
  -d "client_id=admin-cli" \
  -d "username=${ADMIN_USER}" \
  -d "password=${ADMIN_PASS}" | jq -r '.access_token')

echo "[+] Admin token acquired"

# Create a service-account agent for E2E testing
CLIENT_PAYLOAD=$(cat <<'PAYLOAD'
{
  "clientId": "test-agent-001",
  "enabled": true,
  "protocol": "openid-connect",
  "publicClient": false,
  "serviceAccountsEnabled": true,
  "directAccessGrantsEnabled": false,
  "standardFlowEnabled": false,
  "attributes": {
    "agent_id": "test-agent-001"
  }
}
PAYLOAD
)

# Idempotent: check if client exists
EXISTING=$(curl -sf -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KEYCLOAK_URL}/admin/realms/${REALM}/clients?clientId=test-agent-001")

if [ "$EXISTING" = "200" ]; then
  CLIENTS_JSON=$(curl -sf \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    "${KEYCLOAK_URL}/admin/realms/${REALM}/clients?clientId=test-agent-001")
  COUNT=$(echo "$CLIENTS_JSON" | jq length)
  if [ "$COUNT" -eq 0 ]; then
    curl -sf -X POST \
      -H "Authorization: Bearer ${ADMIN_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "${CLIENT_PAYLOAD}" \
      "${KEYCLOAK_URL}/admin/realms/${REALM}/clients"
    echo "[+] Client test-agent-001 created"
  else
    echo "[=] Client test-agent-001 already exists"
  fi
fi

# Retrieve client secret
CLIENT_ID=$(curl -sf \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KEYCLOAK_URL}/admin/realms/${REALM}/clients?clientId=test-agent-001" \
  | jq -r '.[0].id')

SECRET=$(curl -sf \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  "${KEYCLOAK_URL}/admin/realms/${REALM}/clients/${CLIENT_ID}/client-secret" \
  | jq -r '.value')

echo "[+] client_id=test-agent-001  client_secret=${SECRET}"

# Store in k8s secret for tests
kubectl create secret generic test-agent-credentials \
  --namespace=mcp-system \
  --from-literal=client_id=test-agent-001 \
  --from-literal=client_secret="${SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "[+] Secret test-agent-credentials written to mcp-system"
```

---

## 2.4 JWT Validation

Envoy Gateway's `SecurityPolicy` CRD attaches JWT validation to the Gateway.
Every request to an MCP route must carry a valid Bearer token issued by the
Keycloak `mcp-gateway` realm.

### SecurityPolicy YAML

```yaml
# templates/security-policy.yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: mcp-jwt-auth
  namespace: mcp-system
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: mcp-gateway
  jwt:
    providers:
      - name: keycloak-mcp
        issuer: "https://keycloak.mcp-system.svc.cluster.local/realms/mcp-gateway"
        audiences:
          - "mcp-gateway-api"
        remoteJWKS:
          uri: "https://keycloak.mcp-system.svc.cluster.local/realms/mcp-gateway/protocol/openid-connect/certs"
          cacheDuration: 300s
        claimToHeaders:
          - claim: agent_id
            header: x-mcp-agent-id
          - claim: sub
            header: x-mcp-subject
          - claim: realm_access.roles
            header: x-mcp-roles
        extractFrom:
          headers:
            - name: Authorization
              valuePrefix: "Bearer "
```

### How It Works

1. Envoy receives the request on the HTTPS listener.
2. The `SecurityPolicy` triggers JWT validation before routing.
3. The JWKS endpoint is fetched (and cached for 5 minutes) from Keycloak.
4. On success, `agent_id`, `sub`, and roles are injected as
   `x-mcp-agent-id`, `x-mcp-subject`, and `x-mcp-roles` headers for
   downstream consumption by the ext_authz filter (Phase 3).
5. On failure (missing, expired, wrong audience), Envoy returns **401
   Unauthorized** with a JSON body.

---

## 2.5 Rate Limiting

### Architecture

```
Client -> Envoy (route match) -> rate-limit check -> backend
                                      |
                              Envoy Rate Limit Service
                                      |
                                    Redis
```

### Redis + Rate Limit Service Deployment

Add as Helm dependencies or raw manifests. A minimal setup:

```yaml
# values.yaml
rateLimiting:
  enabled: true
  redis:
    host: redis.mcp-system.svc.cluster.local
    port: 6379
  service:
    replicas: 2
    image: envoyproxy/ratelimit:v1.4.0
```

### Rate Limit Config

```yaml
# templates/ratelimit-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-ratelimit-config
  namespace: mcp-system
data:
  config.yaml: |
    domain: mcp-gateway
    descriptors:
      # Global per-agent limit: 100 req/min
      - key: agent_id
        rate_limit:
          unit: minute
          requests_per_unit: 100

      # Per-server limit: 60 req/min per agent per server
      - key: agent_id
        descriptors:
          - key: mcp_server
            rate_limit:
              unit: minute
              requests_per_unit: 60

      # Per-tool limit: 20 req/min per agent per tool
      - key: agent_id
        descriptors:
          - key: mcp_server
            descriptors:
              - key: mcp_tool
                rate_limit:
                  unit: minute
                  requests_per_unit: 20

      # Unauthenticated fallback (should not happen; defense in depth)
      - key: remote_address
        rate_limit:
          unit: minute
          requests_per_unit: 10
```

### BackendTrafficPolicy

```yaml
# templates/backend-traffic-policy.yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: BackendTrafficPolicy
metadata:
  name: mcp-rate-limit
  namespace: mcp-system
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: mcp-gateway
  rateLimit:
    type: Global
    global:
      rules:
        - clientSelectors:
            - headers:
                - name: x-mcp-agent-id
                  type: Distinct
          limit:
            requests: 100
            unit: Minute
        - clientSelectors:
            - headers:
                - name: x-mcp-agent-id
                  type: Distinct
                - name: x-mcp-server
                  type: Distinct
          limit:
            requests: 60
            unit: Minute
        - clientSelectors:
            - headers:
                - name: x-mcp-agent-id
                  type: Distinct
                - name: x-mcp-server
                  type: Distinct
                - name: Mcp-Name
                  type: Distinct
          limit:
            requests: 20
            unit: Minute
```

---

## 2.6 End-to-End Tests

Tests live in `test/e2e/gateway_test.go` and run against a kind cluster with
the full stack deployed.

```go
package e2e

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	gatewayURL   = "https://localhost:8443"
	keycloakURL  = "https://localhost:8444"
	realm        = "mcp-gateway"
	testClientID = "test-agent-001"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // self-signed in CI
	},
}

// getToken obtains a JWT from Keycloak using client_credentials grant.
func getToken(t *testing.T, clientID, clientSecret string) string {
	t.Helper()
	body := fmt.Sprintf(
		"grant_type=client_credentials&client_id=%s&client_secret=%s",
		clientID, clientSecret,
	)
	resp, err := httpClient.Post(
		fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", keycloakURL, realm),
		"application/x-www-form-urlencoded",
		strings.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "token request failed")

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tokenResp))
	require.NotEmpty(t, tokenResp.AccessToken)
	return tokenResp.AccessToken
}

// getClientSecret reads the test-agent secret from the cluster.
func getClientSecret(t *testing.T) string {
	t.Helper()
	// In CI this is set as an env var by the test harness
	secret := mustEnv(t, "TEST_AGENT_CLIENT_SECRET")
	return secret
}

func mustEnv(t *testing.T, key string) string {
	t.Helper()
	val, ok := lookupEnv(key)
	require.True(t, ok, "env var %s must be set", key)
	return val
}

// --- Test Cases ---

func TestUnauthenticatedRequest_Returns401(t *testing.T) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		gatewayURL+"/test-server/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"unauthenticated requests must be rejected with 401")
}

func TestValidJWT_Returns200(t *testing.T) {
	secret := getClientSecret(t)
	token := getToken(t, testClientID, secret)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		gatewayURL+"/test-server/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"authenticated request must succeed")

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "jsonrpc",
		"response must be valid JSON-RPC")
}

func TestExpiredJWT_Returns401(t *testing.T) {
	// This token was generated with exp=0 (epoch); always expired.
	expiredToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJpc3MiOiJ0ZXN0IiwiZXhwIjowLCJhdWQiOiJtY3AtZ2F0ZXdheS1hcGkifQ." +
		"invalid-signature"

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		gatewayURL+"/test-server/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+expiredToken)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"expired JWT must be rejected with 401")
}

func TestRateLimit_Returns429(t *testing.T) {
	secret := getClientSecret(t)
	token := getToken(t, testClientID, secret)

	var lastStatus int
	// Send 110 requests to exceed the 100/min per-agent limit
	for i := 0; i < 110; i++ {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			gatewayURL+"/test-server/mcp",
			strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list","id":1}`),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		lastStatus = resp.StatusCode

		if resp.StatusCode == http.StatusTooManyRequests {
			break
		}
	}

	assert.Equal(t, http.StatusTooManyRequests, lastStatus,
		"rate-limited requests must return 429")
}
```

### Running E2E Tests

```bash
# From repo root
make e2e-setup           # spins up kind + installs chart
make e2e-bootstrap       # runs bootstrap-keycloak.sh
go test -v -tags=e2e -count=1 ./test/e2e/...
make e2e-teardown        # destroys kind cluster
```

---

## Deliverables Checklist

| Item | Path | Status |
|------|------|--------|
| Envoy Gateway Helm dep | `deploy/charts/mcp-gateway/Chart.yaml` | |
| GatewayClass + Gateway | `deploy/charts/mcp-gateway/templates/` | |
| cert-manager issuer | `deploy/charts/mcp-gateway/templates/cluster-issuer.yaml` | |
| Route builder | `internal/envoy/route.go` | |
| Reconciler integration | `internal/controller/mcpserver_controller.go` | |
| Keycloak subchart | `deploy/charts/mcp-gateway/Chart.yaml` | |
| Realm ConfigMap | `deploy/charts/mcp-gateway/templates/keycloak-realm-cm.yaml` | |
| Bootstrap script | `scripts/bootstrap-keycloak.sh` | |
| SecurityPolicy | `deploy/charts/mcp-gateway/templates/security-policy.yaml` | |
| Rate limit ConfigMap | `deploy/charts/mcp-gateway/templates/ratelimit-configmap.yaml` | |
| BackendTrafficPolicy | `deploy/charts/mcp-gateway/templates/backend-traffic-policy.yaml` | |
| E2E tests | `test/e2e/gateway_test.go` | |
