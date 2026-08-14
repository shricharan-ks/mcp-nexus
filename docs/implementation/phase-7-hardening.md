# Phase 7: Production Hardening (Weeks 25-28)

Make the platform production-ready with high availability, secret rotation, scale-to-zero, load testing, security hardening, and disaster recovery.

---

## Step 1: High Availability

Configure all components for HA: multi-replica operator with leader election, 3-replica API server, PodDisruptionBudgets, Redis Sentinel for shared state, and a failover test.

### Files

```
deploy/helm/templates/operator-deployment.yaml       (update replicas, leader election)
deploy/helm/templates/api-server-deployment.yaml      (update replicas)
deploy/helm/templates/pdb-operator.yaml
deploy/helm/templates/pdb-api-server.yaml
deploy/helm/templates/pdb-envoy.yaml
deploy/helm/templates/redis-sentinel.yaml
deploy/helm/values.yaml                               (add HA section)
internal/controller/leader_election.go
test/e2e/ha_failover_test.go
```

### Key Code

**deploy/helm/templates/operator-deployment.yaml** (HA additions)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-operator
  namespace: {{ .Release.Namespace }}
spec:
  replicas: {{ .Values.ha.operator.replicas | default 2 }}
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: operator
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app.kubernetes.io/name: operator
              topologyKey: kubernetes.io/hostname
      containers:
        - name: operator
          args:
            - --leader-elect=true
            - --leader-election-id={{ .Release.Name }}-operator
            - --leader-election-namespace={{ .Release.Namespace }}
            - --leader-election-lease-duration=15s
            - --leader-election-renew-deadline=10s
            - --leader-election-retry-period=2s
          env:
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
```

**deploy/helm/templates/pdb-operator.yaml**

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ .Release.Name }}-operator
  namespace: {{ .Release.Namespace }}
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: operator
```

**deploy/helm/templates/redis-sentinel.yaml**

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Release.Name }}-redis
  namespace: {{ .Release.Namespace }}
spec:
  serviceName: {{ .Release.Name }}-redis
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/name: redis
  template:
    metadata:
      labels:
        app.kubernetes.io/name: redis
        app.kubernetes.io/part-of: mcp-gateway
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app.kubernetes.io/name: redis
                topologyKey: kubernetes.io/hostname
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
              name: redis
            - containerPort: 26379
              name: sentinel
          command: ["redis-server"]
          args:
            - /etc/redis/redis.conf
            - --requirepass
            - $(REDIS_PASSWORD)
          env:
            - name: REDIS_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ .Release.Name }}-redis
                  key: password
          volumeMounts:
            - name: data
              mountPath: /data
            - name: config
              mountPath: /etc/redis
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          readinessProbe:
            exec:
              command: ["redis-cli", "-a", "$(REDIS_PASSWORD)", "ping"]
            initialDelaySeconds: 5
      volumes:
        - name: config
          configMap:
            name: {{ .Release.Name }}-redis-config
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
```

**test/e2e/ha_failover_test.go**

```go
//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorFailover(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Step 1: Identify the current leader
	leaderPod := getLeaderPod(t, ctx, "mcp-system", "mcp-gateway-operator")
	t.Logf("Current leader: %s", leaderPod)

	// Step 2: Create an MCPServer to verify the operator is working
	kubectl(t, "apply", "-f", "testdata/ha-test-mcpserver.yaml")
	t.Cleanup(func() {
		kubectl(t, "delete", "-f", "testdata/ha-test-mcpserver.yaml", "--ignore-not-found")
	})
	waitForMCPServer(t, ctx, "ha-test-server", "mcp-system")

	// Step 3: Kill the leader pod
	t.Logf("Deleting leader pod: %s", leaderPod)
	kubectl(t, "delete", "pod", leaderPod, "-n", "mcp-system", "--grace-period=0")

	// Step 4: Wait for new leader election
	assertEventually(t, ctx, 30*time.Second, func() bool {
		newLeader := getLeaderPod(t, ctx, "mcp-system", "mcp-gateway-operator")
		return newLeader != "" && newLeader != leaderPod
	}, "new leader should be elected after old leader is deleted")

	newLeader := getLeaderPod(t, ctx, "mcp-system", "mcp-gateway-operator")
	t.Logf("New leader: %s", newLeader)

	// Step 5: Verify the new leader reconciles
	// Modify the MCPServer to trigger reconciliation
	kubectl(t, "annotate", "mcpserver", "ha-test-server", "-n", "mcp-system",
		"test/failover-check="+time.Now().Format(time.RFC3339), "--overwrite")

	// Step 6: Verify the MCPServer is still healthy
	waitForMCPServer(t, ctx, "ha-test-server", "mcp-system")
}

func TestAPIServerHA(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Verify 3 replicas are running
	pods := getPods(t, ctx, "mcp-system", "app.kubernetes.io/name=api-server")
	assert.GreaterOrEqual(t, len(pods), 3)

	// Kill one pod and verify requests still succeed
	kubectl(t, "delete", "pod", pods[0], "-n", "mcp-system", "--grace-period=0")

	// Immediate request should succeed (other replicas handle it)
	resp := httpGet(t, "http://mcp-gateway-api.mcp-system:8080/healthz")
	assert.Equal(t, 200, resp.StatusCode)

	// Wait for replacement pod
	assertEventually(t, ctx, 60*time.Second, func() bool {
		currentPods := getPods(t, ctx, "mcp-system", "app.kubernetes.io/name=api-server")
		return len(currentPods) >= 3
	}, "API server should return to 3 replicas")
}

func getLeaderPod(t *testing.T, ctx context.Context, namespace, leaseName string) string {
	t.Helper()
	output := kubectlOutput(t, "get", "lease", leaseName, "-n", namespace,
		"-o", "jsonpath={.spec.holderIdentity}")
	return output
}
```

### Quality Gate

- Operator Deployment has 2+ replicas with anti-affinity.
- Only one operator pod holds the leader lease at any time.
- Deleting the leader pod triggers a new election within 30 seconds.
- API server continues serving requests when one of three replicas is killed.
- PDBs prevent evicting the last replica during node drains.

### Testing Command

```bash
# Deploy HA configuration
helm upgrade --install mcp-gateway deploy/helm/ \
  --set ha.operator.replicas=2 \
  --set ha.apiServer.replicas=3 \
  -n mcp-system

# Verify replicas
kubectl get pods -n mcp-system -l app.kubernetes.io/name=operator
kubectl get pods -n mcp-system -l app.kubernetes.io/name=api-server

# Check leader lease
kubectl get lease -n mcp-system

# Run failover test
go test -tags=e2e ./test/e2e/ -run TestOperatorFailover -v -timeout 10m
```

### Pitfalls

- **Leader election lease duration:** Setting `leaseDuration` too low (< 10s) causes false leader transitions during brief network partitions. The 15s/10s/2s defaults are safe for most clusters.
- **Anti-affinity on single-node clusters:** The `requiredDuringSchedulingIgnoredDuringExecution` anti-affinity will leave pods unschedulable on single-node dev clusters. Use `preferredDuringSchedulingIgnoredDuringExecution` for development, `required` for production.
- **Redis Sentinel partition handling:** During a network partition, Sentinel may promote a replica that has stale data. Configure `min-replicas-to-write: 1` to prevent writes to isolated masters.

### Progress Marker

- [ ] Operator runs with 2+ replicas and leader election
- [ ] API server runs with 3 replicas
- [ ] PDBs created for all critical components
- [ ] Redis Sentinel cluster operational
- [ ] Failover test passes

---

## Step 2: Secret Rotation

Integrate External Secrets Operator for managing MCP server credentials and implement hash-based annotation triggers for rolling updates on secret changes.

### Files

```
api/v1alpha1/mcpserver_types.go              (add externalSecretRef field)
internal/controller/mcpserver_controller.go   (hash annotation logic)
internal/controller/secret_rotation.go
deploy/helm/templates/external-secret.yaml
deploy/helm/templates/secret-store.yaml
```

### Key Code

**MCPServer spec extension**

```go
// In MCPServerSpec, add:
type MCPServerSpec struct {
	// ... existing fields ...

	// ExternalSecretRef references an ExternalSecret that provides credentials.
	// When the ExternalSecret rotates, the MCPServer pods are restarted.
	// +optional
	ExternalSecretRef *ExternalSecretReference `json:"externalSecretRef,omitempty"`
}

type ExternalSecretReference struct {
	// Name is the name of the ExternalSecret resource.
	Name string `json:"name"`

	// RefreshInterval is how often to check for rotation (e.g., "1h").
	// +kubebuilder:default="1h"
	RefreshInterval string `json:"refreshInterval,omitempty"`
}
```

**internal/controller/secret_rotation.go**

```go
package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// SecretHashAnnotation stores the hash of secret data. When the hash
	// changes, the controller triggers a rollout of the MCP server pods.
	SecretHashAnnotation = "mcp.gateway.io/secret-hash"
)

// computeSecretHash computes a deterministic SHA-256 hash of a Secret's data.
func computeSecretHash(secret *corev1.Secret) string {
	h := sha256.New()

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write(secret.Data[k])
		h.Write([]byte("\n"))
	}

	return hex.EncodeToString(h.Sum(nil))[:16] // first 16 hex chars
}

// checkSecretRotation compares the current secret hash with the stored hash
// annotation. Returns true if the secret has been rotated and a pod restart
// is needed.
func (r *MCPServerReconciler) checkSecretRotation(
	ctx context.Context,
	secretName string,
	namespace string,
	currentHash string,
) (bool, string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, secret); err != nil {
		return false, "", fmt.Errorf("getting secret %s: %w", secretName, err)
	}

	newHash := computeSecretHash(secret)
	rotated := currentHash != "" && currentHash != newHash
	return rotated, newHash, nil
}
```

**deploy/helm/templates/external-secret.yaml**

```yaml
{{- range .Values.externalSecrets }}
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: {{ .name }}
  namespace: {{ $.Release.Namespace }}
spec:
  refreshInterval: {{ .refreshInterval | default "1h" }}
  secretStoreRef:
    name: {{ $.Release.Name }}-secret-store
    kind: ClusterSecretStore
  target:
    name: {{ .name }}
    creationPolicy: Owner
    template:
      type: Opaque
  data:
    {{- range .keys }}
    - secretKey: {{ .key }}
      remoteRef:
        key: {{ .remoteKey }}
        property: {{ .property | default "" }}
    {{- end }}
---
{{- end }}
```

**deploy/helm/templates/secret-store.yaml**

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: {{ .Release.Name }}-secret-store
spec:
  provider:
    {{- if eq .Values.secretStore.provider "vault" }}
    vault:
      server: {{ .Values.secretStore.vault.address }}
      path: {{ .Values.secretStore.vault.path | default "secret" }}
      version: v2
      auth:
        kubernetes:
          mountPath: {{ .Values.secretStore.vault.authPath | default "kubernetes" }}
          role: {{ .Values.secretStore.vault.role }}
    {{- else if eq .Values.secretStore.provider "aws" }}
    aws:
      service: SecretsManager
      region: {{ .Values.secretStore.aws.region }}
      auth:
        jwt:
          serviceAccountRef:
            name: {{ .Release.Name }}-external-secrets
    {{- end }}
```

### Quality Gate

- ExternalSecret resources sync successfully from the configured secret store.
- Changing a secret value in Vault/AWS triggers ExternalSecret refresh.
- The operator detects the hash change and triggers a rolling restart.
- No downtime during secret rotation (rolling update strategy).

### Testing Command

```bash
# Deploy with external secrets
helm upgrade --install mcp-gateway deploy/helm/ \
  --set secretStore.provider=vault \
  --set secretStore.vault.address=http://vault:8200 \
  -n mcp-system

# Verify ExternalSecret sync
kubectl get externalsecrets -n mcp-system
kubectl describe externalsecret <name> -n mcp-system

# Simulate secret rotation
vault kv put secret/mcp/github-token GITHUB_TOKEN=new-token-value

# Watch for pod restart
kubectl get pods -n mcp-system -w

# Run rotation test
go test -tags=e2e ./test/e2e/ -run TestSecretRotation -v
```

### Pitfalls

- **Hash annotation race condition:** If the secret changes twice before the controller processes the first change, only the latest hash is stored. This is acceptable behavior since the latest secret values are always what we want.
- **External Secrets Operator prerequisite:** The ESO CRDs and controller must be installed before deploying MCPGateway with `externalSecretRef`. Add a Helm dependency or document the prerequisite clearly.
- **Secret store authentication bootstrap:** The ClusterSecretStore needs credentials to talk to Vault/AWS. This creates a chicken-and-egg problem. Use Kubernetes-native auth (ServiceAccount tokens) to avoid needing a pre-existing secret.

### Progress Marker

- [ ] ExternalSecretReference field added to MCPServer spec
- [ ] Secret hash computation is deterministic
- [ ] Hash annotation triggers pod rolling restart
- [ ] ExternalSecret Helm templates work with Vault and AWS
- [ ] ClusterSecretStore configured

---

## Step 3: Scale-to-Zero with KEDA

Configure KEDA ScaledObjects for MCP servers that should scale to zero when idle, with an HTTP interceptor proxy to hold requests during cold-start.

### Files

```
deploy/helm/templates/keda-scaledobject.yaml
deploy/helm/templates/keda-interceptor.yaml
internal/controller/keda_integration.go
internal/controller/keda_integration_test.go
```

### Key Code

**deploy/helm/templates/keda-scaledobject.yaml**

```yaml
{{- range .Values.mcpServers }}
{{- if .scaleToZero.enabled }}
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{ .name }}-scaledobject
  namespace: {{ $.Release.Namespace }}
  labels:
    app.kubernetes.io/part-of: mcp-gateway
    mcp.gateway.io/server: {{ .name }}
spec:
  scaleTargetRef:
    name: {{ .name }}
  pollingInterval: {{ .scaleToZero.pollingInterval | default 15 }}
  cooldownPeriod: {{ .scaleToZero.cooldownPeriod | default 300 }}
  minReplicaCount: 0
  maxReplicaCount: {{ .scaleToZero.maxReplicas | default 10 }}
  triggers:
    - type: prometheus
      metadata:
        serverAddress: http://prometheus-operated.monitoring:9090
        metricName: mcp_server_active_connections
        query: |
          sum(envoy_http_downstream_cx_active{envoy_http_conn_manager_prefix=~"mcp.{{ $.Release.Namespace }}.{{ .name }}.*"})
        threshold: "1"
        activationThreshold: "0"
    - type: prometheus
      metadata:
        serverAddress: http://prometheus-operated.monitoring:9090
        metricName: mcp_server_request_rate
        query: |
          sum(rate(envoy_http_downstream_rq_total{envoy_http_conn_manager_prefix=~"mcp.{{ $.Release.Namespace }}.{{ .name }}.*"}[2m]))
        threshold: "0.1"
        activationThreshold: "0"
  advanced:
    restoreToOriginalReplicaCount: false
    horizontalPodAutoscalerConfig:
      behavior:
        scaleDown:
          stabilizationWindowSeconds: {{ .scaleToZero.scaleDownStabilization | default 300 }}
          policies:
            - type: Pods
              value: 1
              periodSeconds: 60
        scaleUp:
          stabilizationWindowSeconds: 0
          policies:
            - type: Pods
              value: 4
              periodSeconds: 15
---
{{- end }}
{{- end }}
```

**deploy/helm/templates/keda-interceptor.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-cold-start-proxy
  namespace: {{ .Release.Namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: cold-start-proxy
  template:
    spec:
      containers:
        - name: proxy
          image: "{{ .Values.keda.interceptor.image }}:{{ .Values.keda.interceptor.tag }}"
          env:
            - name: UPSTREAM_TIMEOUT
              value: "{{ .Values.keda.interceptor.upstreamTimeout | default "30s" }}"
            - name: HEALTH_CHECK_INTERVAL
              value: "{{ .Values.keda.interceptor.healthCheckInterval | default "500ms" }}"
          ports:
            - containerPort: 8080
              name: http
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
```

**internal/controller/keda_integration.go**

```go
package controller

import (
	"context"
	"fmt"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileScaledObject creates or updates a KEDA ScaledObject for an MCPServer
// that has scale-to-zero enabled.
func (r *MCPServerReconciler) reconcileScaledObject(
	ctx context.Context,
	server *mcpv1alpha1.MCPServer,
) error {
	if server.Spec.ScaleToZero == nil || !server.Spec.ScaleToZero.Enabled {
		// Delete ScaledObject if it exists
		existing := &kedav1alpha1.ScaledObject{}
		key := client.ObjectKey{
			Name:      server.Name + "-scaledobject",
			Namespace: server.Namespace,
		}
		if err := r.Get(ctx, key, existing); err == nil {
			return r.Delete(ctx, existing)
		}
		return nil
	}

	so := &kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name + "-scaledobject",
			Namespace: server.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, so, func() error {
		so.Spec = kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: server.Name,
			},
			PollingInterval: int32Ptr(server.Spec.ScaleToZero.PollingInterval),
			CooldownPeriod:  int32Ptr(server.Spec.ScaleToZero.CooldownPeriod),
			MinReplicaCount: int32Ptr(0),
			MaxReplicaCount: int32Ptr(server.Spec.ScaleToZero.MaxReplicas),
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "prometheus",
					Metadata: map[string]string{
						"serverAddress":       "http://prometheus-operated.monitoring:9090",
						"metricName":          "mcp_server_active_connections",
						"query":               fmt.Sprintf(`sum(envoy_http_downstream_cx_active{envoy_http_conn_manager_prefix=~"mcp.%s.%s.*"})`, server.Namespace, server.Name),
						"threshold":           "1",
						"activationThreshold": "0",
					},
				},
			},
		}
		return controllerutil.SetControllerReference(server, so, r.Scheme)
	})

	return err
}

func int32Ptr(val int32) *int32 {
	return &val
}
```

### Quality Gate

- ScaledObject is created when MCPServer has `scaleToZero.enabled: true`.
- MCP server deployment scales to 0 replicas after cooldown period with no traffic.
- First request after scale-to-zero is held by the interceptor proxy until the server starts.
- Cold-start time is under 30 seconds.
- ScaledObject is deleted when `scaleToZero` is disabled or the MCPServer is deleted.

### Testing Command

```bash
# Deploy with scale-to-zero
helm upgrade --install mcp-gateway deploy/helm/ \
  --set keda.enabled=true \
  -n mcp-system

# Verify ScaledObject
kubectl get scaledobject -n mcp-system

# Wait for scale-to-zero (after cooldown)
kubectl get deploy <server-name> -n mcp-system -w

# Trigger cold start
curl http://mcp-gateway-envoy.mcp-system:10000/mcp/<server>/sse

# Run tests
go test ./internal/controller/ -run TestKEDA -v -count=1
```

### Pitfalls

- **Prometheus query lag:** KEDA polls Prometheus at the configured interval. If the Prometheus scrape interval is longer than KEDA's polling interval, stale metrics may cause premature scale-down. Ensure the Prometheus scrape interval is less than half of KEDA's polling interval.
- **Cold-start timeout:** SSE connections from agents may time out during cold start. The interceptor proxy must respond with HTTP 202 or use chunked transfer encoding to keep the connection alive while the server starts.
- **KEDA CRD version mismatch:** KEDA v2.x and v1.x have different CRD APIs. Pin the KEDA version in `go.mod` and Helm chart dependencies.

### Progress Marker

- [ ] ScaledObject created for scale-to-zero MCPServers
- [ ] Deployment scales to 0 with no traffic
- [ ] Cold-start proxy holds requests during startup
- [ ] Cold-start completes in under 30 seconds
- [ ] ScaledObject cleanup on delete/disable

---

## Step 4: Load Testing

Create comprehensive k6 load test scripts covering tool calls, discovery, and rate limiting scenarios with strict performance thresholds.

### Files

```
test/load/k6-mcp-load.js
test/load/k6-config.json
test/load/run-load-test.sh
test/load/Makefile
```

### Key Code

**test/load/k6-mcp-load.js**

```javascript
import http from "k6/http";
import { check, sleep, group } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";

// Custom metrics
const toolCallDuration = new Trend("mcp_tool_call_duration", true);
const discoveryDuration = new Trend("mcp_discovery_duration", true);
const rateLimitedRequests = new Counter("mcp_rate_limited_total");
const toolCallErrors = new Rate("mcp_tool_call_error_rate");

// Configuration
const BASE_URL = __ENV.MCP_GATEWAY_URL || "http://localhost:10000";
const MCP_SERVER = __ENV.MCP_SERVER || "load-test-server";

export const options = {
  scenarios: {
    // Scenario 1: Steady tool_call load
    tool_calls: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "2m", target: 100 },   // ramp up
        { duration: "5m", target: 1000 },  // peak load
        { duration: "5m", target: 1000 },  // sustain
        { duration: "2m", target: 0 },     // ramp down
      ],
      gracefulRampDown: "30s",
      exec: "toolCallScenario",
    },

    // Scenario 2: Discovery (tools/list) burst
    discovery: {
      executor: "constant-arrival-rate",
      rate: 500,
      timeUnit: "1s",
      duration: "3m",
      preAllocatedVUs: 200,
      maxVUs: 500,
      exec: "discoveryScenario",
      startTime: "2m",
    },

    // Scenario 3: Rate-limited agent
    rate_limited: {
      executor: "constant-vus",
      vus: 50,
      duration: "5m",
      exec: "rateLimitedScenario",
      startTime: "4m",
    },
  },

  thresholds: {
    // Global thresholds
    http_req_duration: ["p(95)<300", "p(99)<500"],
    http_req_failed: ["rate<0.01"],

    // Tool call thresholds
    mcp_tool_call_duration: ["p(95)<200", "p(99)<500"],
    mcp_tool_call_error_rate: ["rate<0.005"],

    // Discovery thresholds
    mcp_discovery_duration: ["p(95)<100", "p(99)<250"],

    // Rate limiting should work correctly
    mcp_rate_limited_total: ["count>0"],
  },
};

// JSON-RPC helper
function jsonrpc(method, params = {}, id = 1) {
  return JSON.stringify({
    jsonrpc: "2.0",
    method: method,
    params: params,
    id: id,
  });
}

export function toolCallScenario() {
  group("tool_call", () => {
    const payload = jsonrpc("tools/call", {
      name: "echo",
      arguments: { message: `load-test-${__VU}-${__ITER}` },
    });

    const res = http.post(
      `${BASE_URL}/mcp/${MCP_SERVER}/message`,
      payload,
      {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${__ENV.MCP_TOKEN || "load-test-token"}`,
        },
        tags: { name: "tool_call" },
      }
    );

    toolCallDuration.add(res.timings.duration);

    const success = check(res, {
      "status is 200": (r) => r.status === 200,
      "response is valid JSON-RPC": (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.jsonrpc === "2.0" && body.result !== undefined;
        } catch {
          return false;
        }
      },
    });

    toolCallErrors.add(!success);

    sleep(Math.random() * 0.5); // jitter
  });
}

export function discoveryScenario() {
  group("discovery", () => {
    const payload = jsonrpc("tools/list");

    const res = http.post(
      `${BASE_URL}/mcp/${MCP_SERVER}/message`,
      payload,
      {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${__ENV.MCP_TOKEN || "load-test-token"}`,
        },
        tags: { name: "discovery" },
      }
    );

    discoveryDuration.add(res.timings.duration);

    check(res, {
      "status is 200": (r) => r.status === 200,
      "returns tool list": (r) => {
        try {
          const body = JSON.parse(r.body);
          return Array.isArray(body.result?.tools);
        } catch {
          return false;
        }
      },
    });
  });
}

export function rateLimitedScenario() {
  group("rate_limited", () => {
    // Send requests faster than the rate limit
    for (let i = 0; i < 20; i++) {
      const payload = jsonrpc("tools/call", {
        name: "echo",
        arguments: { message: "rate-limit-test" },
      });

      const res = http.post(
        `${BASE_URL}/mcp/${MCP_SERVER}/message`,
        payload,
        {
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${__ENV.RATE_LIMITED_TOKEN || "rate-limited-token"}`,
          },
          tags: { name: "rate_limited" },
        }
      );

      if (res.status === 429) {
        rateLimitedRequests.add(1);
        check(res, {
          "rate limit response has retry-after": (r) =>
            r.headers["Retry-After"] !== undefined,
        });
        break;
      }
    }

    sleep(1);
  });
}

export function handleSummary(data) {
  return {
    "test/load/results/summary.json": JSON.stringify(data, null, 2),
    stdout: textSummary(data, { indent: " ", enableColors: true }),
  };
}

function textSummary(data, opts) {
  // k6 built-in summary
  return "";
}
```

**test/load/run-load-test.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/results"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

mkdir -p "${RESULTS_DIR}"

echo "=== MCP Gateway Load Test ==="
echo "Target: ${MCP_GATEWAY_URL:-http://localhost:10000}"
echo "Server: ${MCP_SERVER:-load-test-server}"
echo "Results: ${RESULTS_DIR}/run-${TIMESTAMP}"
echo ""

# Run k6
k6 run \
  --out json="${RESULTS_DIR}/run-${TIMESTAMP}/metrics.json" \
  --summary-export="${RESULTS_DIR}/run-${TIMESTAMP}/summary.json" \
  "${SCRIPT_DIR}/k6-mcp-load.js"

EXIT_CODE=$?

echo ""
echo "=== Results ==="
echo "Exit code: ${EXIT_CODE}"
echo "Results directory: ${RESULTS_DIR}/run-${TIMESTAMP}"

if [ ${EXIT_CODE} -ne 0 ]; then
  echo ""
  echo "THRESHOLD VIOLATIONS DETECTED"
  echo "Check the summary for details."
fi

exit ${EXIT_CODE}
```

### Quality Gate

- All thresholds pass under load: p99 < 500ms, error rate < 1%, tool call p95 < 200ms.
- Rate limiting correctly returns 429 with Retry-After header.
- No pod OOMs or restarts during the test.
- Results are exportable for trending.

### Testing Command

```bash
# Install k6
brew install k6  # or: go install go.k6.io/k6@latest

# Run load test locally
chmod +x test/load/run-load-test.sh
MCP_GATEWAY_URL=http://localhost:10000 ./test/load/run-load-test.sh

# Run with custom settings
k6 run --vus 100 --duration 2m test/load/k6-mcp-load.js

# Run in CI (nightly)
make load-test
```

### Pitfalls

- **k6 VU memory:** Each VU uses ~1-5MB of memory. 1000 VUs requires at least 5GB RAM on the load generation machine. Run load tests from a dedicated machine, not from the cluster itself.
- **Connection pooling:** k6 reuses HTTP connections by default. This is realistic for persistent MCP clients but may hide connection setup overhead. Add a scenario with `noConnectionReuse: true` to test cold connections.
- **Clock skew in results:** If the load generator and target cluster have different clocks, response time measurements may be skewed. Use NTP on both machines.

### Progress Marker

- [ ] k6 script covers all three scenarios
- [ ] Thresholds defined for p95, p99, and error rate
- [ ] Rate limiting scenario validates 429 responses
- [ ] Results exported to JSON for trending
- [ ] Load test runs in nightly CI

---

## Step 5: Security Hardening

Apply defense-in-depth: default-deny NetworkPolicies, per-component allow policies, PodSecurity restricted profile, RBAC audit, and CI vulnerability scanning.

### Files

```
deploy/helm/templates/networkpolicy-default-deny.yaml
deploy/helm/templates/networkpolicy-operator.yaml
deploy/helm/templates/networkpolicy-envoy.yaml
deploy/helm/templates/networkpolicy-api-server.yaml
deploy/helm/templates/podsecurity.yaml
deploy/helm/templates/rbac-audit.yaml
.github/workflows/security-scan.yaml
hack/audit-rbac.sh
```

### Key Code

**deploy/helm/templates/networkpolicy-default-deny.yaml**

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .Release.Name }}-default-deny
  namespace: {{ .Release.Namespace }}
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
```

**deploy/helm/templates/networkpolicy-operator.yaml**

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .Release.Name }}-operator
  namespace: {{ .Release.Namespace }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: operator
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Prometheus scraping
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
          podSelector:
            matchLabels:
              app.kubernetes.io/name: prometheus
      ports:
        - port: 8080
          protocol: TCP
    # Webhook server (for admission webhooks)
    - from: []
      ports:
        - port: 9443
          protocol: TCP
  egress:
    # Kubernetes API server
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - port: 443
          protocol: TCP
        - port: 6443
          protocol: TCP
    # OTel Collector
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: otel-collector
      ports:
        - port: 4317
          protocol: TCP
    # DNS
    - to: []
      ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
```

**deploy/helm/templates/networkpolicy-envoy.yaml**

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .Release.Name }}-envoy
  namespace: {{ .Release.Namespace }}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: envoy
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Client traffic (from any namespace or external)
    - ports:
        - port: 10000
          protocol: TCP
    # Prometheus scraping
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
      ports:
        - port: 9901
          protocol: TCP
  egress:
    # MCP server backends (in same namespace)
    - to:
        - podSelector: {}
      ports:
        - port: 8080
          protocol: TCP
        - port: 8443
          protocol: TCP
    # xDS control plane (operator)
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: operator
      ports:
        - port: 18000
          protocol: TCP
    # OTel Collector
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: otel-collector
      ports:
        - port: 4317
          protocol: TCP
    # DNS
    - to: []
      ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
```

**PodSecurity configuration (namespace label)**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Release.Namespace }}
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/enforce-version: latest
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

**.github/workflows/security-scan.yaml**

```yaml
name: Security Scan

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: "0 6 * * 1"  # Monday 6am UTC

jobs:
  govulncheck:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Run govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

  trivy-config:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Trivy config scan
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: config
          scan-ref: deploy/
          severity: CRITICAL,HIGH
          exit-code: 1

  trivy-image:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        image:
          - operator
          - api-server
          - mlflow-converter
          - marketplace-indexer
    steps:
      - uses: actions/checkout@v4
      - name: Build image
        run: docker build -t mcp-gateway-${{ matrix.image }}:scan -f cmd/${{ matrix.image }}/Dockerfile .
      - name: Trivy image scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: mcp-gateway-${{ matrix.image }}:scan
          format: sarif
          output: trivy-${{ matrix.image }}.sarif
          severity: CRITICAL,HIGH
          exit-code: 1
      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: trivy-${{ matrix.image }}.sarif

  rbac-audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: RBAC audit
        run: |
          chmod +x hack/audit-rbac.sh
          ./hack/audit-rbac.sh
```

**hack/audit-rbac.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== RBAC Audit ==="

RBAC_FILES=$(find deploy/ -name "*.yaml" | xargs grep -l "ClusterRole\|Role" 2>/dev/null || true)

ISSUES=0

for file in $RBAC_FILES; do
  echo ""
  echo "Checking: $file"

  # Check for wildcard verbs
  if grep -q '"*"' "$file" || grep -q "'*'" "$file"; then
    echo "  WARNING: Wildcard verb (*) found - use explicit verb list"
    ISSUES=$((ISSUES + 1))
  fi

  # Check for wildcard resources
  if grep -q 'resources:.*"*"' "$file"; then
    echo "  WARNING: Wildcard resource (*) found - use explicit resource list"
    ISSUES=$((ISSUES + 1))
  fi

  # Check for cluster-admin binding
  if grep -q "cluster-admin" "$file"; then
    echo "  CRITICAL: cluster-admin role binding found"
    ISSUES=$((ISSUES + 1))
  fi

  # Check for secrets access
  if grep -q "secrets" "$file"; then
    echo "  INFO: Secrets access declared - verify necessity"
  fi
done

echo ""
echo "=== Summary ==="
echo "Files checked: $(echo "$RBAC_FILES" | wc -w | tr -d ' ')"
echo "Issues found: $ISSUES"

if [ $ISSUES -gt 0 ]; then
  echo ""
  echo "RBAC audit failed with $ISSUES issues."
  exit 1
fi

echo "RBAC audit passed."
```

### Quality Gate

- Default-deny NetworkPolicy blocks all traffic not explicitly allowed.
- Each component can only communicate with its required dependencies.
- All pods run under the `restricted` PodSecurity profile.
- No wildcard RBAC verbs or resources in production RBAC roles.
- `govulncheck` reports zero known vulnerabilities in Go dependencies.
- Trivy reports zero CRITICAL/HIGH vulnerabilities in container images.

### Testing Command

```bash
# Apply NetworkPolicies
helm upgrade --install mcp-gateway deploy/helm/ \
  --set networkPolicy.enabled=true \
  -n mcp-system

# Verify default deny
kubectl run nettest --rm -it --restart=Never --image=busybox -n mcp-system -- \
  wget -qO- --timeout=5 http://mcp-gateway-operator:8080/metrics 2>&1 || echo "BLOCKED (expected)"

# Run RBAC audit
chmod +x hack/audit-rbac.sh
./hack/audit-rbac.sh

# Run govulncheck
govulncheck ./...

# Run Trivy config scan
trivy config deploy/
```

### Pitfalls

- **Default-deny blocks DNS:** The default-deny policy blocks egress, including DNS resolution. Every component's allow policy must include egress to port 53 (UDP and TCP) or pods cannot resolve service names.
- **PodSecurity restricted profile:** The `restricted` profile requires `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, and specific seccomp profiles. Envoy and Redis containers may need custom SecurityContexts to comply.
- **NetworkPolicy CNI dependency:** NetworkPolicies require a CNI that supports them (Calico, Cilium, Weave). The default Kind CNI (kindnet) does not enforce NetworkPolicies. Use Calico for testing.

### Progress Marker

- [ ] Default-deny NetworkPolicy deployed
- [ ] Per-component allow policies tested
- [ ] PodSecurity restricted enforced on namespace
- [ ] RBAC audit passes with zero issues
- [ ] govulncheck passes in CI
- [ ] Trivy scans pass for all images

---

## Step 6: Disaster Recovery

Implement backup and restore with Velero, including automated backup schedules, restore scripts, and a DR test that validates RTO < 30 minutes.

### Files

```
deploy/helm/templates/velero-schedule.yaml
hack/backup.sh
hack/restore.sh
hack/dr-test.sh
deploy/helm/values.yaml   (add backup section)
```

### Key Code

**deploy/helm/templates/velero-schedule.yaml**

```yaml
apiVersion: velero.io/v1
kind: Schedule
metadata:
  name: {{ .Release.Name }}-daily-backup
  namespace: velero
spec:
  schedule: "{{ .Values.backup.schedule | default "0 2 * * *" }}"
  template:
    includedNamespaces:
      - {{ .Release.Namespace }}
    includedResources:
      - mcpservers.mcp.gateway.io
      - mcppolicies.mcp.gateway.io
      - mcpmarketplaceentries.mcp.gateway.io
      - secrets
      - configmaps
      - persistentvolumeclaims
    labelSelector:
      matchLabels:
        app.kubernetes.io/part-of: mcp-gateway
    storageLocation: {{ .Values.backup.storageLocation | default "default" }}
    volumeSnapshotLocations:
      - {{ .Values.backup.volumeSnapshotLocation | default "default" }}
    ttl: {{ .Values.backup.ttl | default "720h" }}
    snapshotVolumes: true
    hooks:
      resources:
        - name: postgresql-backup
          includedResources: [pods]
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: postgresql
          pre:
            - exec:
                container: postgresql
                command:
                  - /bin/bash
                  - -c
                  - pg_dump -U $POSTGRES_USER -d $POSTGRES_DB > /var/lib/postgresql/backup/pre-velero.sql
                onError: Fail
                timeout: 120s
```

**hack/backup.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${MCP_NAMESPACE:-mcp-system}"
BACKUP_NAME="mcp-backup-$(date +%Y%m%d-%H%M%S)"

echo "=== MCP Gateway Backup ==="
echo "Namespace: ${NAMESPACE}"
echo "Backup name: ${BACKUP_NAME}"
echo ""

# Pre-flight checks
echo "Pre-flight checks..."
velero version --client-only >/dev/null 2>&1 || { echo "ERROR: velero CLI not found"; exit 1; }
kubectl get ns velero >/dev/null 2>&1 || { echo "ERROR: velero namespace not found"; exit 1; }

# Check Velero server health
VELERO_STATUS=$(velero backup-location get -o json | jq -r '.items[0].status.phase')
if [ "$VELERO_STATUS" != "Available" ]; then
  echo "ERROR: Velero backup location is not available (status: ${VELERO_STATUS})"
  exit 1
fi

# Create backup
echo "Creating backup..."
velero backup create "${BACKUP_NAME}" \
  --include-namespaces="${NAMESPACE}" \
  --include-resources="mcpservers.mcp.gateway.io,mcppolicies.mcp.gateway.io,mcpmarketplaceentries.mcp.gateway.io,secrets,configmaps,persistentvolumeclaims" \
  --selector="app.kubernetes.io/part-of=mcp-gateway" \
  --snapshot-volumes \
  --wait

# Verify backup
echo ""
echo "Verifying backup..."
PHASE=$(velero backup get "${BACKUP_NAME}" -o json | jq -r '.status.phase')
if [ "$PHASE" != "Completed" ]; then
  echo "ERROR: Backup phase is ${PHASE}, expected Completed"
  velero backup logs "${BACKUP_NAME}"
  exit 1
fi

ITEMS=$(velero backup get "${BACKUP_NAME}" -o json | jq '.status.progress.itemsBackedUp')
echo "Backup completed: ${ITEMS} items backed up"
echo "Backup name: ${BACKUP_NAME}"
```

**hack/restore.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_NAME="${1:?Usage: restore.sh <backup-name> [--dry-run]}"
DRY_RUN="${2:-}"
NAMESPACE="${MCP_NAMESPACE:-mcp-system}"

echo "=== MCP Gateway Restore ==="
echo "Backup: ${BACKUP_NAME}"
echo "Target namespace: ${NAMESPACE}"

# Verify backup exists
velero backup get "${BACKUP_NAME}" >/dev/null 2>&1 || {
  echo "ERROR: Backup ${BACKUP_NAME} not found"
  echo "Available backups:"
  velero backup get
  exit 1
}

if [ "${DRY_RUN}" == "--dry-run" ]; then
  echo ""
  echo "DRY RUN: Would restore the following:"
  velero backup describe "${BACKUP_NAME}" --details
  exit 0
fi

# Confirm
echo ""
echo "WARNING: This will restore resources into namespace ${NAMESPACE}."
echo "Existing resources with the same names will be updated."
read -p "Continue? (y/N) " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted."
  exit 0
fi

# Run restore
RESTORE_NAME="mcp-restore-$(date +%Y%m%d-%H%M%S)"
echo "Starting restore: ${RESTORE_NAME}"
velero restore create "${RESTORE_NAME}" \
  --from-backup="${BACKUP_NAME}" \
  --include-namespaces="${NAMESPACE}" \
  --wait

# Verify restore
PHASE=$(velero restore get "${RESTORE_NAME}" -o json | jq -r '.status.phase')
if [ "$PHASE" != "Completed" ]; then
  echo "ERROR: Restore phase is ${PHASE}"
  velero restore logs "${RESTORE_NAME}"
  exit 1
fi

echo ""
echo "Restore completed."

# Post-restore verification
echo "Verifying restored resources..."
echo "MCPServers:"
kubectl get mcpservers -n "${NAMESPACE}"
echo ""
echo "MCPPolicies:"
kubectl get mcppolicies -n "${NAMESPACE}"
echo ""

# Restart operator to reconcile
echo "Restarting operator to reconcile restored resources..."
kubectl rollout restart deployment -n "${NAMESPACE}" -l app.kubernetes.io/name=operator
kubectl rollout status deployment -n "${NAMESPACE}" -l app.kubernetes.io/name=operator --timeout=120s

echo ""
echo "Restore complete. Verify all servers are Ready:"
kubectl get mcpservers -n "${NAMESPACE}" -o wide
```

**hack/dr-test.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${MCP_NAMESPACE:-mcp-system}"
DR_NAMESPACE="mcp-dr-test"
START_TIME=$(date +%s)

echo "=== MCP Gateway Disaster Recovery Test ==="
echo "Source namespace: ${NAMESPACE}"
echo "DR test namespace: ${DR_NAMESPACE}"
echo ""

# Step 1: Create a known state
echo "Step 1: Creating test resources..."
kubectl create namespace "${DR_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f test/e2e/testdata/dr-test-resources.yaml -n "${DR_NAMESPACE}"
sleep 5

# Step 2: Take a backup
echo ""
echo "Step 2: Taking backup..."
BACKUP_NAME="dr-test-$(date +%Y%m%d-%H%M%S)"
velero backup create "${BACKUP_NAME}" \
  --include-namespaces="${DR_NAMESPACE}" \
  --selector="app.kubernetes.io/part-of=mcp-gateway" \
  --wait

# Step 3: Simulate disaster
echo ""
echo "Step 3: Simulating disaster (deleting namespace)..."
kubectl delete namespace "${DR_NAMESPACE}" --wait=true

# Step 4: Restore
echo ""
echo "Step 4: Restoring from backup..."
kubectl create namespace "${DR_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
velero restore create "dr-test-restore" \
  --from-backup="${BACKUP_NAME}" \
  --include-namespaces="${DR_NAMESPACE}" \
  --wait

# Step 5: Verify
echo ""
echo "Step 5: Verifying restored resources..."
SERVERS=$(kubectl get mcpservers -n "${DR_NAMESPACE}" -o name 2>/dev/null | wc -l | tr -d ' ')
POLICIES=$(kubectl get mcppolicies -n "${DR_NAMESPACE}" -o name 2>/dev/null | wc -l | tr -d ' ')

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo ""
echo "=== DR Test Results ==="
echo "Duration: ${DURATION} seconds ($(( DURATION / 60 )) minutes)"
echo "MCPServers restored: ${SERVERS}"
echo "MCPPolicies restored: ${POLICIES}"

# Step 6: Cleanup
echo ""
echo "Cleaning up..."
kubectl delete namespace "${DR_NAMESPACE}" --ignore-not-found
velero backup delete "${BACKUP_NAME}" --confirm

# Check RTO
MAX_RTO_SECONDS=1800  # 30 minutes
if [ $DURATION -gt $MAX_RTO_SECONDS ]; then
  echo ""
  echo "FAIL: DR took ${DURATION}s, exceeds RTO target of ${MAX_RTO_SECONDS}s (30 minutes)"
  exit 1
fi

if [ "$SERVERS" -lt 1 ]; then
  echo "FAIL: No MCPServers were restored"
  exit 1
fi

echo ""
echo "PASS: DR test completed in ${DURATION}s (RTO target: ${MAX_RTO_SECONDS}s)"
```

### Quality Gate

- Daily backup schedule runs at 2 AM and completes successfully.
- Backup includes all CRDs, Secrets, ConfigMaps, and PVCs with the mcp-gateway label.
- PostgreSQL pre-backup hook dumps the database before volume snapshot.
- Restore script recovers all resources and triggers operator reconciliation.
- DR test completes in under 30 minutes (RTO target).
- DR test verifies resource counts match pre-disaster state.

### Testing Command

```bash
# Install Velero (with MinIO for testing)
velero install --provider aws --plugins velero/velero-plugin-for-aws:v1.9.0 \
  --bucket mcp-backups --secret-file ./credentials-velero \
  --backup-location-config region=us-east-1,s3ForcePathStyle=true,s3Url=http://minio:9000

# Run manual backup
chmod +x hack/backup.sh
./hack/backup.sh

# List backups
velero backup get

# Dry-run restore
chmod +x hack/restore.sh
./hack/restore.sh <backup-name> --dry-run

# Run DR test
chmod +x hack/dr-test.sh
./hack/dr-test.sh
```

### Pitfalls

- **Velero CRD restore ordering:** Velero restores resources in a specific order, but CRDs must exist before CRs. If the CRDs themselves were lost (full cluster disaster), install the Helm chart CRDs first, then run the Velero restore for CRs only.
- **PVC snapshot driver:** Volume snapshots require a CSI driver that supports snapshots. Check `kubectl get volumesnapshotclass` before relying on volume snapshots. Fall back to Restic/Kopia file-level backup if snapshots are not available.
- **Secret restoration security:** Velero backups contain secrets in plaintext (base64-encoded in the backup tarball). Ensure the backup storage (S3 bucket) is encrypted at rest with KMS and has strict access controls.
- **PostgreSQL consistency:** The pre-backup hook runs `pg_dump` but the volume snapshot may capture a slightly different state. For strict consistency, use `pg_dump` only and skip volume snapshots for the PostgreSQL PVC, or use PostgreSQL's native backup tools (pg_basebackup).

### Progress Marker

- [ ] Velero Schedule created for daily backups
- [ ] Backup script runs successfully
- [ ] Restore script recovers all resources
- [ ] DR test completes under 30 minutes
- [ ] PostgreSQL pre-backup hook works
- [ ] Backup storage encrypted at rest
