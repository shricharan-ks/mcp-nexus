# Phase 1: Core Operator

**Goal:** `kubectl apply -f examples/github-server.yaml` creates a running pod that responds to MCP protocol requests.

**Definition of Done:** envtest passes with 80%+ coverage, 3 example MCP servers deploy successfully, E2E test passes in Kind.

**Prerequisites:** Phase 0 complete -- operator pod runs in Kind with `make kind-up && make dev-deploy`.

---

## Step 1.1: MCPServer CRD Types

### Overview

Define the MCPServer Custom Resource Definition (CRD) types in Go. These types define the contract between users and the operator: what users can declare, and what the operator reports back.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `api/v1alpha1/mcpserver_types.go` | Core MCPServer type definitions |
| `api/v1alpha1/groupversion_info.go` | API group/version registration |
| `api/v1alpha1/zz_generated.deepcopy.go` | Auto-generated DeepCopy methods |

### Key Code/Config

**api/v1alpha1/groupversion_info.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1alpha1 contains API Schema definitions for the gateway v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=gateway.mcp-gateway.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "gateway.mcp-gateway.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionResource scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
```

**api/v1alpha1/mcpserver_types.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// =============================================================================
// Enums
// =============================================================================

// MCPServerPhase describes the current lifecycle phase of an MCPServer.
// +kubebuilder:validation:Enum=Pending;Deploying;Running;Updating;Failed;Terminating
type MCPServerPhase string

const (
	// MCPServerPhasePending indicates the MCPServer has been created but not yet processed.
	MCPServerPhasePending MCPServerPhase = "Pending"

	// MCPServerPhaseDeploying indicates the operator is creating the Deployment and Service.
	MCPServerPhaseDeploying MCPServerPhase = "Deploying"

	// MCPServerPhaseRunning indicates the MCPServer pod is running and healthy.
	MCPServerPhaseRunning MCPServerPhase = "Running"

	// MCPServerPhaseUpdating indicates the MCPServer is being updated (rolling update).
	MCPServerPhaseUpdating MCPServerPhase = "Updating"

	// MCPServerPhaseFailed indicates the MCPServer failed to deploy or run.
	MCPServerPhaseFailed MCPServerPhase = "Failed"

	// MCPServerPhaseTerminating indicates the MCPServer is being deleted.
	MCPServerPhaseTerminating MCPServerPhase = "Terminating"
)

// TransportType describes how clients communicate with the MCP server.
// +kubebuilder:validation:Enum=stdio;sse;streamable-http
type TransportType string

const (
	// TransportStdio indicates the MCP server uses stdin/stdout for communication.
	// The operator wraps it with a sidecar proxy that exposes HTTP.
	TransportStdio TransportType = "stdio"

	// TransportSSE indicates the MCP server exposes a Server-Sent Events endpoint.
	TransportSSE TransportType = "sse"

	// TransportStreamableHTTP indicates the MCP server uses the streamable HTTP transport.
	TransportStreamableHTTP TransportType = "streamable-http"
)

// =============================================================================
// Spec Types
// =============================================================================

// MCPServerSpec defines the desired state of an MCPServer.
type MCPServerSpec struct {
	// Image is the container image for the MCP server.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Transport specifies the MCP transport protocol.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=stdio
	Transport TransportType `json:"transport"`

	// Protocol defines MCP protocol settings (e.g., endpoint paths).
	// +optional
	Protocol *MCPServerProtocol `json:"protocol,omitempty"`

	// Source specifies where the MCP server definition comes from (e.g., a registry).
	// +optional
	Source *MCPServerSource `json:"source,omitempty"`

	// Replicas is the desired number of MCP server pod replicas.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Scaling defines autoscaling configuration.
	// +optional
	Scaling *MCPServerScaling `json:"scaling,omitempty"`

	// Port is the port the MCP server listens on (for sse/streamable-http transports).
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=3000
	// +optional
	Port *int32 `json:"port,omitempty"`

	// Command overrides the container entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args are arguments to the entrypoint.
	// +optional
	Args []string `json:"args,omitempty"`

	// Env is a list of environment variables to set in the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources defines compute resource requirements.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Secrets defines references to Kubernetes Secrets to mount or inject.
	// +optional
	Secrets []MCPServerSecret `json:"secrets,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use for the MCP server pods.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// NodeSelector is a selector for node assignment.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod scheduling.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Labels are additional labels to add to the MCP server pods.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are additional annotations to add to the MCP server pods.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// HealthCheck configures the health check for the MCP server.
	// If not specified, the operator uses the MCP initialize call as a health check.
	// +optional
	HealthCheck *MCPServerHealthCheck `json:"healthCheck,omitempty"`
}

// MCPServerProtocol defines MCP protocol-specific configuration.
type MCPServerProtocol struct {
	// SSEEndpoint is the path for the SSE endpoint (for sse transport).
	// +kubebuilder:default="/sse"
	// +optional
	SSEEndpoint string `json:"sseEndpoint,omitempty"`

	// MessageEndpoint is the path for sending messages (for sse transport).
	// +kubebuilder:default="/message"
	// +optional
	MessageEndpoint string `json:"messageEndpoint,omitempty"`

	// MCPEndpoint is the path for the streamable HTTP endpoint.
	// +kubebuilder:default="/mcp"
	// +optional
	MCPEndpoint string `json:"mcpEndpoint,omitempty"`

	// Version is the MCP protocol version the server supports.
	// +kubebuilder:default="2025-03-26"
	// +optional
	Version string `json:"version,omitempty"`
}

// MCPServerSource describes where an MCP server definition originates.
type MCPServerSource struct {
	// Registry is the name of the MCP server registry (e.g., "smithery", "mcp-get").
	// +optional
	Registry string `json:"registry,omitempty"`

	// Name is the server name in the registry.
	// +optional
	Name string `json:"name,omitempty"`

	// Version is the server version from the registry.
	// +optional
	Version string `json:"version,omitempty"`
}

// MCPServerScaling defines autoscaling behavior.
type MCPServerScaling struct {
	// MinReplicas is the minimum number of replicas.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	// +optional
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`

	// TargetCPUUtilization is the target CPU utilization percentage for scaling.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=80
	// +optional
	TargetCPUUtilization *int32 `json:"targetCPUUtilization,omitempty"`
}

// MCPServerSecret defines a reference to a Kubernetes Secret.
type MCPServerSecret struct {
	// Name is the name of the Kubernetes Secret.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// MountPath, if specified, mounts the Secret as a volume at this path.
	// Mutually exclusive with EnvPrefix.
	// +optional
	MountPath string `json:"mountPath,omitempty"`

	// EnvPrefix, if specified, injects the Secret's keys as environment variables
	// with this prefix. For example, prefix "MCP_" and key "TOKEN" becomes "MCP_TOKEN".
	// Mutually exclusive with MountPath.
	// +optional
	EnvPrefix string `json:"envPrefix,omitempty"`

	// Keys is an optional list of specific keys to use from the Secret.
	// If empty, all keys are used.
	// +optional
	Keys []string `json:"keys,omitempty"`
}

// MCPServerHealthCheck defines health check configuration.
type MCPServerHealthCheck struct {
	// Enabled controls whether health checking is active.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// IntervalSeconds is the interval between health checks.
	// +kubebuilder:validation:Minimum=5
	// +kubebuilder:default=30
	// +optional
	IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`

	// TimeoutSeconds is the timeout for each health check.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// FailureThreshold is the number of consecutive failures before marking unhealthy.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`
}

// =============================================================================
// Status Types
// =============================================================================

// MCPServerStatus defines the observed state of an MCPServer.
type MCPServerStatus struct {
	// Phase is the current lifecycle phase of the MCPServer.
	// +optional
	Phase MCPServerPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the MCPServer's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ReadyReplicas is the number of ready MCP server pod replicas.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available MCP server pod replicas.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Endpoint is the internal service URL where the MCP server can be reached.
	// Format: http://<service-name>.<namespace>.svc.cluster.local:<port>
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Capabilities contains the discovered MCP server capabilities after initialization.
	// +optional
	Capabilities *DiscoveredCapabilities `json:"capabilities,omitempty"`

	// LastDiscoveryTime is the timestamp of the last successful capability discovery.
	// +optional
	LastDiscoveryTime *metav1.Time `json:"lastDiscoveryTime,omitempty"`

	// ObservedGeneration is the most recently observed generation of the MCPServer.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Message provides additional details about the current phase (e.g., error messages).
	// +optional
	Message string `json:"message,omitempty"`
}

// DiscoveredCapabilities contains the MCP capabilities discovered from a running server.
type DiscoveredCapabilities struct {
	// ServerName is the name the MCP server reports for itself.
	// +optional
	ServerName string `json:"serverName,omitempty"`

	// ServerVersion is the version the MCP server reports.
	// +optional
	ServerVersion string `json:"serverVersion,omitempty"`

	// ProtocolVersion is the MCP protocol version the server supports.
	// +optional
	ProtocolVersion string `json:"protocolVersion,omitempty"`

	// Tools is the list of tools the MCP server provides.
	// +optional
	Tools []MCPTool `json:"tools,omitempty"`

	// Resources is the list of resources the MCP server provides.
	// +optional
	Resources []MCPResource `json:"resources,omitempty"`

	// Prompts is the list of prompts the MCP server provides.
	// +optional
	Prompts []MCPPrompt `json:"prompts,omitempty"`

	// ToolCount is the total number of tools discovered.
	// +optional
	ToolCount int `json:"toolCount,omitempty"`

	// ResourceCount is the total number of resources discovered.
	// +optional
	ResourceCount int `json:"resourceCount,omitempty"`

	// PromptCount is the total number of prompts discovered.
	// +optional
	PromptCount int `json:"promptCount,omitempty"`
}

// MCPTool represents a tool provided by an MCP server.
type MCPTool struct {
	// Name is the tool name.
	Name string `json:"name"`

	// Description is a human-readable description of the tool.
	// +optional
	Description string `json:"description,omitempty"`

	// InputSchema is the JSON Schema for the tool's input parameters.
	// Stored as a raw JSON string.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	InputSchema *runtime.RawExtension `json:"inputSchema,omitempty"`
}

// MCPResource represents a resource provided by an MCP server.
type MCPResource struct {
	// URI is the resource URI.
	URI string `json:"uri"`

	// Name is the resource name.
	Name string `json:"name"`

	// Description is a human-readable description.
	// +optional
	Description string `json:"description,omitempty"`

	// MimeType is the MIME type of the resource content.
	// +optional
	MimeType string `json:"mimeType,omitempty"`
}

// MCPPrompt represents a prompt template provided by an MCP server.
type MCPPrompt struct {
	// Name is the prompt name.
	Name string `json:"name"`

	// Description is a human-readable description.
	// +optional
	Description string `json:"description,omitempty"`

	// Arguments is the list of arguments the prompt accepts.
	// +optional
	Arguments []MCPPromptArgument `json:"arguments,omitempty"`
}

// MCPPromptArgument represents an argument to an MCP prompt.
type MCPPromptArgument struct {
	// Name is the argument name.
	Name string `json:"name"`

	// Description is a human-readable description.
	// +optional
	Description string `json:"description,omitempty"`

	// Required indicates whether this argument must be provided.
	// +optional
	Required bool `json:"required,omitempty"`
}

// =============================================================================
// Root Types
// =============================================================================

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcp;mcps
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`,description="Current phase"
// +kubebuilder:printcolumn:name="Transport",type=string,JSONPath=`.spec.transport`,description="Transport type"
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`,description="Ready replicas"
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`,description="Service endpoint",priority=1
// +kubebuilder:printcolumn:name="Tools",type=integer,JSONPath=`.status.capabilities.toolCount`,description="Discovered tools",priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MCPServer is the Schema for the mcpservers API.
// It represents a single MCP (Model Context Protocol) server that the operator
// manages. The operator handles deployment, service creation, health checking,
// capability discovery, and lifecycle management.
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPServerSpec   `json:"spec,omitempty"`
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPServerList contains a list of MCPServer resources.
type MCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPServer{}, &MCPServerList{})
}

// =============================================================================
// Helper Functions
// =============================================================================

// GetReplicas returns the desired replica count, defaulting to 1.
func (s *MCPServerSpec) GetReplicas() int32 {
	if s.Replicas != nil {
		return *s.Replicas
	}
	return 1
}

// GetPort returns the server port, defaulting to 3000.
func (s *MCPServerSpec) GetPort() int32 {
	if s.Port != nil {
		return *s.Port
	}
	return 3000
}

// IsStdioTransport returns true if the server uses stdio transport.
func (s *MCPServerSpec) IsStdioTransport() bool {
	return s.Transport == TransportStdio
}

// GetResourceRequirements returns the resource requirements with defaults.
func (s *MCPServerSpec) GetResourceRequirements() corev1.ResourceRequirements {
	if s.Resources != nil {
		return *s.Resources
	}
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
}

// SetPhase updates the phase and message in status.
func (s *MCPServerStatus) SetPhase(phase MCPServerPhase, message string) {
	s.Phase = phase
	s.Message = message
}
```

**Important:** The `MCPTool.InputSchema` field uses `runtime.RawExtension` from `k8s.io/apimachinery/pkg/runtime`. Add this import:

```go
import (
	"k8s.io/apimachinery/pkg/runtime"
)
```

This must be added alongside the other imports. The full import block should be:

```go
import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)
```

After creating the types, generate DeepCopy methods and CRD manifests:

```bash
make generate
make manifests
```

### Quality Gate

- `make generate` succeeds (creates `zz_generated.deepcopy.go`)
- `make manifests` succeeds (creates CRD YAML in `config/crd/bases/`)
- `go build ./api/...` compiles without errors
- CRD YAML contains all custom print columns

### Testing Command

```bash
# Generate code
make generate

# Verify DeepCopy was generated
test -f api/v1alpha1/zz_generated.deepcopy.go && echo "PASS" || echo "FAIL"

# Generate manifests
make manifests

# Verify CRD was generated
test -f config/crd/bases/gateway.mcp-gateway.io_mcpservers.yaml && echo "PASS" || echo "FAIL"

# Compile
go build ./api/...
echo "Build: $?"

# Check CRD has expected fields
grep -q "MCPServer" config/crd/bases/gateway.mcp-gateway.io_mcpservers.yaml && echo "PASS" || echo "FAIL"
grep -q "phase" config/crd/bases/gateway.mcp-gateway.io_mcpservers.yaml && echo "PASS" || echo "FAIL"
grep -q "transport" config/crd/bases/gateway.mcp-gateway.io_mcpservers.yaml && echo "PASS" || echo "FAIL"
```

### Common Pitfalls

- Forgetting to run `make generate` after modifying types. The DeepCopy methods are required for the types to be used as Kubernetes objects.
- Using `map[string]interface{}` instead of `runtime.RawExtension` for JSON Schema fields. `RawExtension` is the correct way to store arbitrary JSON in CRD status.
- Not including `+kubebuilder:object:root=true` on the root types. Without this, the code generator skips them.
- Missing `+kubebuilder:subresource:status` causes status updates to fail at runtime (the API server rejects status updates without the subresource enabled).
- Pointer vs. value types: use pointers (`*int32`, `*bool`) for optional fields with meaningful zero values. Use values for required fields.
- `+listType=map` and `+listMapKey=type` on Conditions are required for server-side apply compatibility.
- Circular imports: do not import `internal/` packages from `api/`. The `api/` package must be self-contained.

### Progress Marker

```
[x] 1.1 MCPServer CRD types defined
```

---

## Step 1.2: MCPServerReconciler

### Overview

Implement the reconciliation loop that manages the full lifecycle of MCPServer resources. The reconciler implements a state machine with transitions: Pending -> Deploying -> Running -> Updating -> Failed -> Terminating.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `internal/controller/mcpserver_controller.go` | Main reconciler logic |
| `internal/controller/mcpserver_resources.go` | Deployment and Service builders |
| `internal/controller/mcpserver_labels.go` | Label and selector helpers |
| `internal/controller/suite_test.go` | Test suite setup (envtest) |
| `cmd/operator/main.go` | Register the controller (update) |

### Key Code/Config

**internal/controller/mcpserver_labels.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"fmt"

	gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

const (
	// LabelManagedBy is the label key for the managing controller.
	LabelManagedBy = "app.kubernetes.io/managed-by"

	// LabelPartOf is the label key for the parent application.
	LabelPartOf = "app.kubernetes.io/part-of"

	// LabelComponent is the label key for the component type.
	LabelComponent = "app.kubernetes.io/component"

	// LabelName is the label key for the application name.
	LabelName = "app.kubernetes.io/name"

	// LabelInstance is the label key for the instance identifier.
	LabelInstance = "app.kubernetes.io/instance"

	// LabelVersion is the label key for the application version.
	LabelVersion = "app.kubernetes.io/version"

	// LabelMCPTransport is a custom label for the MCP transport type.
	LabelMCPTransport = "mcp-gateway.io/transport"

	// ManagerName is the value used for the managed-by label.
	ManagerName = "mcp-gateway-operator"

	// FinalizerName is the finalizer added to MCPServer resources.
	FinalizerName = "gateway.mcp-gateway.io/finalizer"
)

// labelsForMCPServer returns the standard labels for resources managed by the operator.
func labelsForMCPServer(server *gatewayv1alpha1.MCPServer) map[string]string {
	labels := map[string]string{
		LabelManagedBy:    ManagerName,
		LabelPartOf:       "mcp-gateway",
		LabelComponent:    "mcp-server",
		LabelName:         server.Name,
		LabelInstance:     server.Name,
		LabelMCPTransport: string(server.Spec.Transport),
	}

	// Add user-specified labels
	for k, v := range server.Spec.Labels {
		labels[k] = v
	}

	return labels
}

// selectorLabelsForMCPServer returns the minimal set of labels used for pod selection.
// These must be immutable after Deployment creation.
func selectorLabelsForMCPServer(server *gatewayv1alpha1.MCPServer) map[string]string {
	return map[string]string{
		LabelName:     server.Name,
		LabelInstance: server.Name,
		LabelPartOf:   "mcp-gateway",
	}
}

// deploymentName returns the name for the Deployment backing an MCPServer.
func deploymentName(server *gatewayv1alpha1.MCPServer) string {
	return fmt.Sprintf("mcp-%s", server.Name)
}

// serviceName returns the name for the Service backing an MCPServer.
func serviceName(server *gatewayv1alpha1.MCPServer) string {
	return fmt.Sprintf("mcp-%s", server.Name)
}

// serviceEndpoint returns the in-cluster endpoint URL for an MCPServer's Service.
func serviceEndpoint(server *gatewayv1alpha1.MCPServer) string {
	port := server.Spec.GetPort()
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		serviceName(server), server.Namespace, port)
}
```

**internal/controller/mcpserver_resources.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// buildDeployment creates the Deployment spec for an MCPServer.
func buildDeployment(server *gatewayv1alpha1.MCPServer) *appsv1.Deployment {
	labels := labelsForMCPServer(server)
	selectorLabels := selectorLabelsForMCPServer(server)
	replicas := server.Spec.GetReplicas()
	port := server.Spec.GetPort()
	resources := server.Spec.GetResourceRequirements()

	// Build container
	container := corev1.Container{
		Name:      "mcp-server",
		Image:     server.Spec.Image,
		Resources: resources,
		Ports: []corev1.ContainerPort{
			{
				Name:          "mcp",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}

	// Set command/args if specified
	if len(server.Spec.Command) > 0 {
		container.Command = server.Spec.Command
	}
	if len(server.Spec.Args) > 0 {
		container.Args = server.Spec.Args
	}

	// Set environment variables
	container.Env = append(container.Env, server.Spec.Env...)

	// Add health checks for non-stdio transports
	if !server.Spec.IsStdioTransport() {
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/",
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			TimeoutSeconds:      3,
			FailureThreshold:    3,
		}
		container.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/",
					Port: intstr.FromInt32(port),
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       20,
			TimeoutSeconds:      3,
			FailureThreshold:    3,
		}
	}

	// Build volumes and volume mounts from secrets
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	for _, secret := range server.Spec.Secrets {
		if secret.MountPath != "" {
			volName := fmt.Sprintf("secret-%s", secret.Name)
			volumes = append(volumes, corev1.Volume{
				Name: volName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secret.Name,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      volName,
				MountPath: secret.MountPath,
				ReadOnly:  true,
			})
		} else if secret.EnvPrefix != "" {
			// Inject as env vars with prefix
			container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
				Prefix: secret.EnvPrefix,
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
				},
			})
		}
	}

	container.VolumeMounts = volumeMounts

	// Build pod annotations
	podAnnotations := make(map[string]string)
	for k, v := range server.Spec.Annotations {
		podAnnotations[k] = v
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(server),
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					Containers:                    []corev1.Container{container},
					Volumes:                       volumes,
					TerminationGracePeriodSeconds: int64Ptr(30),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}

	// Set service account
	if server.Spec.ServiceAccountName != "" {
		dep.Spec.Template.Spec.ServiceAccountName = server.Spec.ServiceAccountName
	}

	// Set node selector
	if len(server.Spec.NodeSelector) > 0 {
		dep.Spec.Template.Spec.NodeSelector = server.Spec.NodeSelector
	}

	// Set tolerations
	if len(server.Spec.Tolerations) > 0 {
		dep.Spec.Template.Spec.Tolerations = server.Spec.Tolerations
	}

	return dep
}

// buildService creates the Service spec for an MCPServer.
func buildService(server *gatewayv1alpha1.MCPServer) *corev1.Service {
	labels := labelsForMCPServer(server)
	selectorLabels := selectorLabelsForMCPServer(server)
	port := server.Spec.GetPort()

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName(server),
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "mcp",
					Port:       port,
					TargetPort: intstr.FromString("mcp"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}
```

**internal/controller/mcpserver_controller.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

const (
	// requeueDelay is the default delay before requeuing a reconciliation.
	requeueDelay = 30 * time.Second

	// requeueDelayShort is used when the operator expects a quick state change.
	requeueDelayShort = 5 * time.Second

	// Condition types
	ConditionTypeReady       = "Ready"
	ConditionTypeDeployed    = "Deployed"
	ConditionTypeDiscovered  = "Discovered"
	ConditionTypeDegraded    = "Degraded"
)

// MCPServerReconciler reconciles MCPServer objects.
type MCPServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gateway.mcp-gateway.io,resources=mcpservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.mcp-gateway.io,resources=mcpservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.mcp-gateway.io,resources=mcpservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop for MCPServer resources.
func (r *MCPServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the MCPServer instance
	var server gatewayv1alpha1.MCPServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("MCPServer resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch MCPServer")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !server.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &server)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&server, FinalizerName) {
		controllerutil.AddFinalizer(&server, FinalizerName)
		if err := r.Update(ctx, &server); err != nil {
			logger.Error(err, "unable to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize phase if empty
	if server.Status.Phase == "" {
		return r.transitionTo(ctx, &server, gatewayv1alpha1.MCPServerPhasePending, "MCPServer created, starting reconciliation")
	}

	// State machine dispatch
	switch server.Status.Phase {
	case gatewayv1alpha1.MCPServerPhasePending:
		return r.reconcilePending(ctx, &server)
	case gatewayv1alpha1.MCPServerPhaseDeploying:
		return r.reconcileDeploying(ctx, &server)
	case gatewayv1alpha1.MCPServerPhaseRunning:
		return r.reconcileRunning(ctx, &server)
	case gatewayv1alpha1.MCPServerPhaseUpdating:
		return r.reconcileUpdating(ctx, &server)
	case gatewayv1alpha1.MCPServerPhaseFailed:
		return r.reconcileFailed(ctx, &server)
	case gatewayv1alpha1.MCPServerPhaseTerminating:
		return r.reconcileTerminating(ctx, &server)
	default:
		logger.Info("unknown phase, resetting to Pending", "phase", server.Status.Phase)
		return r.transitionTo(ctx, &server, gatewayv1alpha1.MCPServerPhasePending, "Unknown phase, resetting")
	}
}

// ---------------------------------------------------------------------------
// State Machine Handlers
// ---------------------------------------------------------------------------

// reconcilePending validates the spec and transitions to Deploying.
func (r *MCPServerReconciler) reconcilePending(ctx context.Context, server *gatewayv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling Pending state")

	// Validate spec
	if server.Spec.Image == "" {
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseFailed, "spec.image is required")
	}

	// Transition to Deploying
	return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseDeploying, "Spec validated, deploying")
}

// reconcileDeploying creates or updates the Deployment and Service.
func (r *MCPServerReconciler) reconcileDeploying(ctx context.Context, server *gatewayv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling Deploying state")

	// Ensure Deployment exists
	if err := r.ensureDeployment(ctx, server); err != nil {
		logger.Error(err, "failed to ensure Deployment")
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseFailed,
			fmt.Sprintf("Failed to create Deployment: %v", err))
	}

	// Ensure Service exists
	if err := r.ensureService(ctx, server); err != nil {
		logger.Error(err, "failed to ensure Service")
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseFailed,
			fmt.Sprintf("Failed to create Service: %v", err))
	}

	// Set Deployed condition
	meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeDeployed,
		Status:             metav1.ConditionTrue,
		Reason:             "DeploymentCreated",
		Message:            "Deployment and Service created successfully",
		ObservedGeneration: server.Generation,
	})

	// Set endpoint
	server.Status.Endpoint = serviceEndpoint(server)

	// Check if deployment is ready
	ready, err := r.isDeploymentReady(ctx, server)
	if err != nil {
		return ctrl.Result{}, err
	}

	if ready {
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseRunning, "Deployment is ready")
	}

	// Not ready yet, requeue
	if err := r.updateStatus(ctx, server); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueDelayShort}, nil
}

// reconcileRunning checks health and detects spec changes.
func (r *MCPServerReconciler) reconcileRunning(ctx context.Context, server *gatewayv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling Running state")

	// Check if spec changed (generation mismatch)
	if server.Status.ObservedGeneration != server.Generation {
		logger.Info("spec changed, transitioning to Updating",
			"observedGeneration", server.Status.ObservedGeneration,
			"generation", server.Generation)
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseUpdating, "Spec changed, updating")
	}

	// Verify Deployment still exists and is healthy
	dep := &appsv1.Deployment{}
	depName := types.NamespacedName{
		Name:      deploymentName(server),
		Namespace: server.Namespace,
	}
	if err := r.Get(ctx, depName, dep); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Deployment disappeared, redeploying")
			return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseDeploying, "Deployment not found, redeploying")
		}
		return ctrl.Result{}, err
	}

	// Update replica counts from Deployment status
	server.Status.ReadyReplicas = dep.Status.ReadyReplicas
	server.Status.AvailableReplicas = dep.Status.AvailableReplicas

	// Check if deployment became unhealthy
	if dep.Status.ReadyReplicas == 0 && server.Spec.GetReplicas() > 0 {
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeDegraded,
			Status:             metav1.ConditionTrue,
			Reason:             "NoReadyReplicas",
			Message:            "No ready replicas available",
			ObservedGeneration: server.Generation,
		})
	} else {
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               ConditionTypeDegraded,
			Status:             metav1.ConditionFalse,
			Reason:             "ReplicasAvailable",
			Message:            fmt.Sprintf("%d/%d replicas ready", dep.Status.ReadyReplicas, server.Spec.GetReplicas()),
			ObservedGeneration: server.Generation,
		})
	}

	// Set Ready condition
	meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Running",
		Message:            "MCPServer is running",
		ObservedGeneration: server.Generation,
	})

	if err := r.updateStatus(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	// Periodic reconciliation to detect drift
	return ctrl.Result{RequeueAfter: requeueDelay}, nil
}

// reconcileUpdating performs a rolling update of the Deployment.
func (r *MCPServerReconciler) reconcileUpdating(ctx context.Context, server *gatewayv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling Updating state")

	// Update the Deployment
	if err := r.ensureDeployment(ctx, server); err != nil {
		logger.Error(err, "failed to update Deployment")
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseFailed,
			fmt.Sprintf("Failed to update Deployment: %v", err))
	}

	// Update the Service
	if err := r.ensureService(ctx, server); err != nil {
		logger.Error(err, "failed to update Service")
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseFailed,
			fmt.Sprintf("Failed to update Service: %v", err))
	}

	// Check if the updated deployment is ready
	ready, err := r.isDeploymentReady(ctx, server)
	if err != nil {
		return ctrl.Result{}, err
	}

	if ready {
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhaseRunning, "Update complete, running")
	}

	// Not ready yet
	if err := r.updateStatus(ctx, server); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueDelayShort}, nil
}

// reconcileFailed handles the Failed state. It retries after a delay.
func (r *MCPServerReconciler) reconcileFailed(ctx context.Context, server *gatewayv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling Failed state", "message", server.Status.Message)

	// If spec was updated (generation changed), retry by going back to Pending
	if server.Status.ObservedGeneration != server.Generation {
		logger.Info("spec changed since failure, retrying")
		return r.transitionTo(ctx, server, gatewayv1alpha1.MCPServerPhasePending, "Spec updated, retrying")
	}

	// Otherwise, requeue with backoff to retry
	return ctrl.Result{RequeueAfter: requeueDelay * 2}, nil
}

// reconcileTerminating handles cleanup during deletion.
func (r *MCPServerReconciler) reconcileTerminating(ctx context.Context, server *gatewayv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling Terminating state")

	// Perform any external cleanup here (e.g., deregister from service mesh,
	// remove external DNS entries, etc.)
	// For now, Kubernetes garbage collection handles child resources via ownerReferences.

	logger.Info("cleanup complete")
	return ctrl.Result{}, nil
}

// reconcileDelete handles the deletion of an MCPServer.
func (r *MCPServerReconciler) reconcileDelete(ctx context.Context, server *gatewayv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("handling MCPServer deletion")

	if controllerutil.ContainsFinalizer(server, FinalizerName) {
		// Set phase to Terminating
		server.Status.SetPhase(gatewayv1alpha1.MCPServerPhaseTerminating, "Cleaning up resources")
		if err := r.Status().Update(ctx, server); err != nil {
			// If status update fails, we can still proceed with cleanup
			logger.Error(err, "failed to update status to Terminating")
		}

		// Perform cleanup
		// ownerReferences handle Deployment/Service deletion automatically.
		// Add any external cleanup here.

		// Remove finalizer
		controllerutil.RemoveFinalizer(server, FinalizerName)
		if err := r.Update(ctx, server); err != nil {
			logger.Error(err, "failed to remove finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// ---------------------------------------------------------------------------
// Resource Management
// ---------------------------------------------------------------------------

// ensureDeployment creates or updates the Deployment for an MCPServer.
func (r *MCPServerReconciler) ensureDeployment(ctx context.Context, server *gatewayv1alpha1.MCPServer) error {
	logger := log.FromContext(ctx)

	desired := buildDeployment(server)

	// Set owner reference for garbage collection
	if err := ctrl.SetControllerReference(server, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	// Check if Deployment already exists
	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if apierrors.IsNotFound(err) {
		// Create
		logger.Info("creating Deployment", "name", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating deployment: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting deployment: %w", err)
	}

	// Update existing deployment
	// Preserve the selector (immutable) but update everything else
	existing.Spec.Replicas = desired.Spec.Replicas
	existing.Spec.Template = desired.Spec.Template
	existing.Labels = desired.Labels

	logger.Info("updating Deployment", "name", existing.Name)
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating deployment: %w", err)
	}

	return nil
}

// ensureService creates or updates the Service for an MCPServer.
func (r *MCPServerReconciler) ensureService(ctx context.Context, server *gatewayv1alpha1.MCPServer) error {
	logger := log.FromContext(ctx)

	desired := buildService(server)

	// Set owner reference
	if err := ctrl.SetControllerReference(server, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference: %w", err)
	}

	// Check if Service already exists
	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if apierrors.IsNotFound(err) {
		// Create
		logger.Info("creating Service", "name", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("creating service: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting service: %w", err)
	}

	// Update existing service
	existing.Spec.Ports = desired.Spec.Ports
	existing.Spec.Selector = desired.Spec.Selector
	existing.Labels = desired.Labels

	logger.Info("updating Service", "name", existing.Name)
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("updating service: %w", err)
	}

	return nil
}

// isDeploymentReady checks if the Deployment has the desired number of ready replicas.
func (r *MCPServerReconciler) isDeploymentReady(ctx context.Context, server *gatewayv1alpha1.MCPServer) (bool, error) {
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      deploymentName(server),
		Namespace: server.Namespace,
	}, dep)
	if err != nil {
		return false, err
	}

	desired := server.Spec.GetReplicas()

	// Zero replicas is a valid "ready" state (scale-to-zero)
	if desired == 0 {
		return true, nil
	}

	return dep.Status.ReadyReplicas >= desired, nil
}

// ---------------------------------------------------------------------------
// Status Helpers
// ---------------------------------------------------------------------------

// transitionTo updates the MCPServer phase and status.
func (r *MCPServerReconciler) transitionTo(
	ctx context.Context,
	server *gatewayv1alpha1.MCPServer,
	phase gatewayv1alpha1.MCPServerPhase,
	message string,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("transitioning phase",
		"from", server.Status.Phase,
		"to", phase,
		"message", message)

	server.Status.SetPhase(phase, message)
	server.Status.ObservedGeneration = server.Generation

	if err := r.updateStatus(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	// Determine requeue behavior based on target phase
	switch phase {
	case gatewayv1alpha1.MCPServerPhaseFailed:
		return ctrl.Result{RequeueAfter: requeueDelay * 2}, nil
	case gatewayv1alpha1.MCPServerPhaseRunning:
		return ctrl.Result{RequeueAfter: requeueDelay}, nil
	default:
		return ctrl.Result{Requeue: true}, nil
	}
}

// updateStatus updates the status subresource.
func (r *MCPServerReconciler) updateStatus(ctx context.Context, server *gatewayv1alpha1.MCPServer) error {
	if err := r.Status().Update(ctx, server); err != nil {
		log.FromContext(ctx).Error(err, "unable to update MCPServer status")
		return fmt.Errorf("updating status: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Controller Setup
// ---------------------------------------------------------------------------

// SetupWithManager sets up the controller with the Manager.
func (r *MCPServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1alpha1.MCPServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
```

**Update cmd/operator/main.go** to register the controller and the CRD scheme. Add these changes:

In the import block, add:

```go
gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
"github.com/mcp-gateway/mcp-gateway/internal/controller"
```

In the `init()` function, add:

```go
utilruntime.Must(gatewayv1alpha1.AddToScheme(scheme))
```

After the manager creation (after `// +kubebuilder:scaffold:builder`), add:

```go
if err = (&controller.MCPServerReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "MCPServer")
    os.Exit(1)
}
```

### Quality Gate

- `go build ./cmd/operator/` compiles
- `go build ./internal/controller/` compiles
- `go vet ./...` passes
- Controller registers with the manager without errors

### Testing Command

```bash
# Build everything
go build ./...
echo "Build: $?"

# Vet
go vet ./...
echo "Vet: $?"

# Verify the controller compiles
go build ./internal/controller/
echo "Controller: $?"

# Verify the operator binary compiles
go build ./cmd/operator/
echo "Operator: $?"
```

### Common Pitfalls

- **Status().Update() vs Update():** Always use `r.Status().Update(ctx, server)` for status changes. Using `r.Update()` for status fields silently does nothing because the status subresource has its own endpoint.
- **ownerReferences:** Always set via `ctrl.SetControllerReference()` before creating child resources. Without this, deleting the MCPServer leaves orphaned Deployments/Services.
- **Finalizers:** Add the finalizer BEFORE doing any external work. Remove it AFTER cleanup is complete. If you remove it first and cleanup fails, the resource is deleted and cleanup never finishes.
- **Selector immutability:** Deployment `.spec.selector` is immutable after creation. Never change `selectorLabelsForMCPServer()` once deployed. If you must change it, delete and recreate the Deployment.
- **Requeue vs RequeueAfter:** Use `Requeue: true` for immediate reprocessing (state transitions). Use `RequeueAfter: duration` for polling/periodic checks. Never use `Requeue: true` in a steady state (Running) or you create a tight loop.
- **Generation vs ObservedGeneration:** `server.Generation` is incremented by the API server on spec changes. Compare with `status.observedGeneration` to detect changes. Do NOT use `resourceVersion` for this purpose.
- **Concurrent updates:** The reconciler may be called concurrently for different MCPServers. Never share mutable state between reconcile calls without synchronization.

### Progress Marker

```
[x] 1.2 MCPServerReconciler implemented
```

---

## Step 1.3: MCP Discovery Client

### Overview

Implement a client that communicates with running MCP servers to discover their capabilities (tools, resources, prompts) via the MCP protocol's JSON-RPC interface.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `internal/discovery/client.go` | MCP discovery client |
| `internal/discovery/types.go` | JSON-RPC types for MCP protocol |
| `internal/discovery/client_test.go` | Client tests |

### Key Code/Config

**internal/discovery/types.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package discovery

import "encoding/json"

// =============================================================================
// JSON-RPC Types
// =============================================================================

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int              `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *JSONRPCError    `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// =============================================================================
// MCP Protocol Types
// =============================================================================

// InitializeParams represents the parameters for the initialize request.
type InitializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation `json:"clientInfo"`
}

// ClientCapabilities represents the capabilities the client supports.
type ClientCapabilities struct {
	// Empty for now - the gateway client doesn't need to advertise capabilities
}

// Implementation identifies the client or server implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents the response from initialize.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

// ServerCapabilities represents what the MCP server supports.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// ToolsCapability indicates the server supports tools.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability indicates the server supports resources.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability indicates the server supports prompts.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability indicates the server supports logging.
type LoggingCapability struct{}

// ToolsListResult represents the response from tools/list.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ResourcesListResult represents the response from resources/list.
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// Resource represents an MCP resource definition.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// PromptsListResult represents the response from prompts/list.
type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

// Prompt represents an MCP prompt definition.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument to a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// DiscoveryResult aggregates all discovered capabilities from an MCP server.
type DiscoveryResult struct {
	ServerInfo      Implementation
	ProtocolVersion string
	Tools           []Tool
	Resources       []Resource
	Prompts         []Prompt
}
```

**internal/discovery/client.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	// DefaultTimeout is the default timeout for MCP requests.
	DefaultTimeout = 10 * time.Second

	// DefaultProtocolVersion is the MCP protocol version to request.
	DefaultProtocolVersion = "2025-03-26"

	// ClientName identifies this discovery client.
	ClientName = "mcp-gateway-operator"

	// ClientVersion is the version of the discovery client.
	ClientVersion = "0.1.0"
)

// Client is an MCP discovery client that communicates with MCP servers
// to discover their capabilities via JSON-RPC.
type Client struct {
	httpClient *http.Client
	idCounter  atomic.Int64
}

// NewClient creates a new MCP discovery client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ClientOption configures the discovery client.
type ClientOption func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// Discover performs full capability discovery against an MCP server.
// It calls initialize, then tools/list, resources/list, and prompts/list
// based on the server's advertised capabilities.
func (c *Client) Discover(ctx context.Context, endpoint string) (*DiscoveryResult, error) {
	// Step 1: Initialize
	initResult, err := c.Initialize(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}

	result := &DiscoveryResult{
		ServerInfo:      initResult.ServerInfo,
		ProtocolVersion: initResult.ProtocolVersion,
	}

	// Step 2: List tools (if supported)
	if initResult.Capabilities.Tools != nil {
		tools, err := c.ListTools(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("tools/list: %w", err)
		}
		result.Tools = tools
	}

	// Step 3: List resources (if supported)
	if initResult.Capabilities.Resources != nil {
		resources, err := c.ListResources(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("resources/list: %w", err)
		}
		result.Resources = resources
	}

	// Step 4: List prompts (if supported)
	if initResult.Capabilities.Prompts != nil {
		prompts, err := c.ListPrompts(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("prompts/list: %w", err)
		}
		result.Prompts = prompts
	}

	return result, nil
}

// Initialize sends the initialize request to an MCP server.
func (c *Client) Initialize(ctx context.Context, endpoint string) (*InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: DefaultProtocolVersion,
		Capabilities:    ClientCapabilities{},
		ClientInfo: Implementation{
			Name:    ClientName,
			Version: ClientVersion,
		},
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling params: %w", err)
	}

	resp, err := c.call(ctx, endpoint, "initialize", paramsJSON)
	if err != nil {
		return nil, err
	}

	var result InitializeResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling initialize result: %w", err)
	}

	return &result, nil
}

// ListTools calls tools/list on the MCP server.
func (c *Client) ListTools(ctx context.Context, endpoint string) ([]Tool, error) {
	resp, err := c.call(ctx, endpoint, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result ToolsListResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling tools/list result: %w", err)
	}

	return result.Tools, nil
}

// ListResources calls resources/list on the MCP server.
func (c *Client) ListResources(ctx context.Context, endpoint string) ([]Resource, error) {
	resp, err := c.call(ctx, endpoint, "resources/list", nil)
	if err != nil {
		return nil, err
	}

	var result ResourcesListResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling resources/list result: %w", err)
	}

	return result.Resources, nil
}

// ListPrompts calls prompts/list on the MCP server.
func (c *Client) ListPrompts(ctx context.Context, endpoint string) ([]Prompt, error) {
	resp, err := c.call(ctx, endpoint, "prompts/list", nil)
	if err != nil {
		return nil, err
	}

	var result PromptsListResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling prompts/list result: %w", err)
	}

	return result.Prompts, nil
}

// call sends a JSON-RPC request to the MCP server and returns the result.
func (c *Client) call(ctx context.Context, endpoint, method string, params json.RawMessage) (json.RawMessage, error) {
	id := int(c.idCounter.Add(1))

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", httpResp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON-RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}
```

**internal/discovery/client_test.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMCPServer creates a test HTTP server that responds to MCP JSON-RPC requests.
func mockMCPServer(t *testing.T, capabilities ServerCapabilities, tools []Tool, resources []Resource, prompts []Prompt) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		var result interface{}
		switch req.Method {
		case "initialize":
			result = InitializeResult{
				ProtocolVersion: DefaultProtocolVersion,
				Capabilities:    capabilities,
				ServerInfo: Implementation{
					Name:    "test-server",
					Version: "1.0.0",
				},
			}
		case "tools/list":
			result = ToolsListResult{Tools: tools}
		case "resources/list":
			result = ResourcesListResult{Resources: resources}
		case "prompts/list":
			result = PromptsListResult{Prompts: prompts}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resultJSON, _ := json.Marshal(result)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestClient_Initialize(t *testing.T) {
	server := mockMCPServer(t,
		ServerCapabilities{
			Tools: &ToolsCapability{},
		},
		nil, nil, nil,
	)
	defer server.Close()

	client := NewClient()
	result, err := client.Initialize(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Equal(t, "test-server", result.ServerInfo.Name)
	assert.Equal(t, "1.0.0", result.ServerInfo.Version)
	assert.Equal(t, DefaultProtocolVersion, result.ProtocolVersion)
	assert.NotNil(t, result.Capabilities.Tools)
}

func TestClient_ListTools(t *testing.T) {
	tools := []Tool{
		{
			Name:        "echo",
			Description: "Echoes the input",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
		},
		{
			Name:        "add",
			Description: "Adds two numbers",
		},
	}

	server := mockMCPServer(t,
		ServerCapabilities{Tools: &ToolsCapability{}},
		tools, nil, nil,
	)
	defer server.Close()

	client := NewClient()
	result, err := client.ListTools(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "echo", result[0].Name)
	assert.Equal(t, "add", result[1].Name)
}

func TestClient_ListResources(t *testing.T) {
	resources := []Resource{
		{
			URI:         "file:///etc/config",
			Name:        "config",
			Description: "Configuration file",
			MimeType:    "application/json",
		},
	}

	server := mockMCPServer(t,
		ServerCapabilities{Resources: &ResourcesCapability{}},
		nil, resources, nil,
	)
	defer server.Close()

	client := NewClient()
	result, err := client.ListResources(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "file:///etc/config", result[0].URI)
}

func TestClient_ListPrompts(t *testing.T) {
	prompts := []Prompt{
		{
			Name:        "summarize",
			Description: "Summarize text",
			Arguments: []PromptArgument{
				{Name: "text", Description: "Text to summarize", Required: true},
			},
		},
	}

	server := mockMCPServer(t,
		ServerCapabilities{Prompts: &PromptsCapability{}},
		nil, nil, prompts,
	)
	defer server.Close()

	client := NewClient()
	result, err := client.ListPrompts(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "summarize", result[0].Name)
	assert.Len(t, result[0].Arguments, 1)
	assert.True(t, result[0].Arguments[0].Required)
}

func TestClient_Discover(t *testing.T) {
	tools := []Tool{{Name: "echo", Description: "Echoes input"}}
	resources := []Resource{{URI: "file:///data", Name: "data"}}
	prompts := []Prompt{{Name: "greet", Description: "Greeting prompt"}}

	server := mockMCPServer(t,
		ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
			Prompts:   &PromptsCapability{},
		},
		tools, resources, prompts,
	)
	defer server.Close()

	client := NewClient()
	result, err := client.Discover(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Equal(t, "test-server", result.ServerInfo.Name)
	assert.Len(t, result.Tools, 1)
	assert.Len(t, result.Resources, 1)
	assert.Len(t, result.Prompts, 1)
}

func TestClient_Discover_ToolsOnly(t *testing.T) {
	tools := []Tool{{Name: "tool1"}}

	server := mockMCPServer(t,
		ServerCapabilities{
			Tools: &ToolsCapability{},
			// No Resources or Prompts capability
		},
		tools, nil, nil,
	)
	defer server.Close()

	client := NewClient()
	result, err := client.Discover(context.Background(), server.URL)

	require.NoError(t, err)
	assert.Len(t, result.Tools, 1)
	assert.Empty(t, result.Resources)
	assert.Empty(t, result.Prompts)
}

func TestClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient()
	_, err := client.Initialize(context.Background(), server.URL)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON-RPC error")
	assert.Contains(t, err.Error(), "Invalid Request")
}

func TestClient_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewClient()
	_, err := client.Initialize(context.Background(), server.URL)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestClient_ConnectionRefused(t *testing.T) {
	client := NewClient()
	_, err := client.Initialize(context.Background(), "http://localhost:1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sending request")
}
```

### Quality Gate

- `go build ./internal/discovery/` compiles
- `go test ./internal/discovery/` passes all tests
- All MCP protocol methods are covered (initialize, tools/list, resources/list, prompts/list)

### Testing Command

```bash
# Build
go build ./internal/discovery/
echo "Build: $?"

# Run tests
go test ./internal/discovery/ -v -count=1
echo "Tests: $?"

# Coverage
go test ./internal/discovery/ -coverprofile=cover-discovery.out
go tool cover -func=cover-discovery.out | tail -1
```

### Common Pitfalls

- Not using `json.RawMessage` for the `params` and `result` fields. These must be raw JSON to avoid double-marshaling.
- Not setting `Content-Type: application/json` on requests. Some MCP servers reject requests without it.
- Not handling the case where capabilities are nil (e.g., server does not support tools). Always check `initResult.Capabilities.Tools != nil` before calling `tools/list`.
- Using a global HTTP client. Always create a per-client instance for proper timeout control.
- Not incrementing the JSON-RPC `id` field. Some servers require unique IDs.
- Forgetting to call `resp.Body.Close()`. Use `defer resp.Body.Close()` immediately after checking for errors.

### Progress Marker

```
[x] 1.3 MCP discovery client implemented
```

---

## Step 1.4: Unit Tests

### Overview

Write comprehensive table-driven unit tests for the resource builders (`buildDeployment`, `buildService`) and label helpers.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `internal/controller/mcpserver_resources_test.go` | Tests for Deployment/Service builders |
| `internal/controller/mcpserver_labels_test.go` | Tests for label and naming helpers |

### Key Code/Config

**internal/controller/mcpserver_labels_test.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

func TestLabelsForMCPServer(t *testing.T) {
	tests := []struct {
		name     string
		server   *gatewayv1alpha1.MCPServer
		expected map[string]string
	}{
		{
			name: "basic labels",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Transport: gatewayv1alpha1.TransportStdio,
				},
			},
			expected: map[string]string{
				LabelManagedBy:    ManagerName,
				LabelPartOf:       "mcp-gateway",
				LabelComponent:    "mcp-server",
				LabelName:         "test-server",
				LabelInstance:     "test-server",
				LabelMCPTransport: "stdio",
			},
		},
		{
			name: "with user labels",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-server",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Transport: gatewayv1alpha1.TransportSSE,
					Labels: map[string]string{
						"team": "platform",
						"env":  "dev",
					},
				},
			},
			expected: map[string]string{
				LabelManagedBy:    ManagerName,
				LabelPartOf:       "mcp-gateway",
				LabelComponent:    "mcp-server",
				LabelName:         "my-server",
				LabelInstance:     "my-server",
				LabelMCPTransport: "sse",
				"team":            "platform",
				"env":             "dev",
			},
		},
		{
			name: "streamable-http transport",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http-server",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Transport: gatewayv1alpha1.TransportStreamableHTTP,
				},
			},
			expected: map[string]string{
				LabelMCPTransport: "streamable-http",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := labelsForMCPServer(tt.server)
			for k, v := range tt.expected {
				assert.Equal(t, v, labels[k], "label %s mismatch", k)
			}
		})
	}
}

func TestSelectorLabelsForMCPServer(t *testing.T) {
	server := &gatewayv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	labels := selectorLabelsForMCPServer(server)

	assert.Equal(t, "test", labels[LabelName])
	assert.Equal(t, "test", labels[LabelInstance])
	assert.Equal(t, "mcp-gateway", labels[LabelPartOf])
	assert.Len(t, labels, 3, "selector labels should be minimal and stable")
}

func TestDeploymentName(t *testing.T) {
	tests := []struct {
		serverName string
		expected   string
	}{
		{"echo", "mcp-echo"},
		{"github-server", "mcp-github-server"},
		{"my-mcp-server", "mcp-my-mcp-server"},
	}

	for _, tt := range tests {
		t.Run(tt.serverName, func(t *testing.T) {
			server := &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{Name: tt.serverName},
			}
			assert.Equal(t, tt.expected, deploymentName(server))
		})
	}
}

func TestServiceName(t *testing.T) {
	server := &gatewayv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{Name: "echo"},
	}
	assert.Equal(t, "mcp-echo", serviceName(server))
}

func TestServiceEndpoint(t *testing.T) {
	port := int32(3000)
	server := &gatewayv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "echo",
			Namespace: "default",
		},
		Spec: gatewayv1alpha1.MCPServerSpec{
			Port: &port,
		},
	}

	expected := "http://mcp-echo.default.svc.cluster.local:3000"
	assert.Equal(t, expected, serviceEndpoint(server))
}

func TestServiceEndpointDefaultPort(t *testing.T) {
	server := &gatewayv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "echo",
			Namespace: "test-ns",
		},
		Spec: gatewayv1alpha1.MCPServerSpec{},
	}

	expected := "http://mcp-echo.test-ns.svc.cluster.local:3000"
	assert.Equal(t, expected, serviceEndpoint(server))
}
```

**internal/controller/mcpserver_resources_test.go:**

```go
/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

func TestBuildDeployment(t *testing.T) {
	tests := []struct {
		name   string
		server *gatewayv1alpha1.MCPServer
		check  func(t *testing.T, dep *appsv1.Deployment)
	}{
		{
			name: "basic deployment with defaults",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "echo",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "ghcr.io/modelcontextprotocol/echo-server:latest",
					Transport: gatewayv1alpha1.TransportStdio,
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				assert.Equal(t, "mcp-echo", dep.Name)
				assert.Equal(t, "default", dep.Namespace)
				assert.Equal(t, int32(1), *dep.Spec.Replicas)

				require.Len(t, dep.Spec.Template.Spec.Containers, 1)
				container := dep.Spec.Template.Spec.Containers[0]
				assert.Equal(t, "mcp-server", container.Name)
				assert.Equal(t, "ghcr.io/modelcontextprotocol/echo-server:latest", container.Image)
				assert.Equal(t, int32(3000), container.Ports[0].ContainerPort)

				// Stdio transport should NOT have HTTP health checks
				assert.Nil(t, container.ReadinessProbe)
				assert.Nil(t, container.LivenessProbe)

				// Security context
				assert.NotNil(t, dep.Spec.Template.Spec.SecurityContext)
				assert.True(t, *dep.Spec.Template.Spec.SecurityContext.RunAsNonRoot)
			},
		},
		{
			name: "SSE transport with health checks",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sse-server",
					Namespace: "test",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-mcp-server:v1",
					Transport: gatewayv1alpha1.TransportSSE,
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				container := dep.Spec.Template.Spec.Containers[0]
				assert.NotNil(t, container.ReadinessProbe, "SSE transport should have readiness probe")
				assert.NotNil(t, container.LivenessProbe, "SSE transport should have liveness probe")
				assert.Equal(t, int32(3000), container.ReadinessProbe.HTTPGet.Port.IntVal)
			},
		},
		{
			name: "custom replicas and port",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-server:v2",
					Transport: gatewayv1alpha1.TransportStreamableHTTP,
					Replicas:  int32Ptr(3),
					Port:      int32Ptr(8080),
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				assert.Equal(t, int32(3), *dep.Spec.Replicas)
				container := dep.Spec.Template.Spec.Containers[0]
				assert.Equal(t, int32(8080), container.Ports[0].ContainerPort)
			},
		},
		{
			name: "custom command and args",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-cmd",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-server:v1",
					Transport: gatewayv1alpha1.TransportStdio,
					Command:   []string{"/bin/server"},
					Args:      []string{"--verbose", "--port=3000"},
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				container := dep.Spec.Template.Spec.Containers[0]
				assert.Equal(t, []string{"/bin/server"}, container.Command)
				assert.Equal(t, []string{"--verbose", "--port=3000"}, container.Args)
			},
		},
		{
			name: "environment variables",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "env-server",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-server:v1",
					Transport: gatewayv1alpha1.TransportStdio,
					Env: []corev1.EnvVar{
						{Name: "API_KEY", Value: "test-key"},
						{Name: "DEBUG", Value: "true"},
					},
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				container := dep.Spec.Template.Spec.Containers[0]
				assert.Len(t, container.Env, 2)
				assert.Equal(t, "API_KEY", container.Env[0].Name)
				assert.Equal(t, "test-key", container.Env[0].Value)
			},
		},
		{
			name: "custom resources",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-server",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-server:v1",
					Transport: gatewayv1alpha1.TransportStdio,
					Resources: &corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				container := dep.Spec.Template.Spec.Containers[0]
				assert.Equal(t, resource.MustParse("250m"), container.Resources.Requests[corev1.ResourceCPU])
				assert.Equal(t, resource.MustParse("1Gi"), container.Resources.Limits[corev1.ResourceMemory])
			},
		},
		{
			name: "secret mount",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-server",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-server:v1",
					Transport: gatewayv1alpha1.TransportStdio,
					Secrets: []gatewayv1alpha1.MCPServerSecret{
						{
							Name:      "api-credentials",
							MountPath: "/etc/secrets",
						},
					},
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				// Check volume
				require.Len(t, dep.Spec.Template.Spec.Volumes, 1)
				assert.Equal(t, "secret-api-credentials", dep.Spec.Template.Spec.Volumes[0].Name)

				// Check mount
				container := dep.Spec.Template.Spec.Containers[0]
				require.Len(t, container.VolumeMounts, 1)
				assert.Equal(t, "/etc/secrets", container.VolumeMounts[0].MountPath)
				assert.True(t, container.VolumeMounts[0].ReadOnly)
			},
		},
		{
			name: "secret as env vars",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "env-secret-server",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-server:v1",
					Transport: gatewayv1alpha1.TransportStdio,
					Secrets: []gatewayv1alpha1.MCPServerSecret{
						{
							Name:      "api-key",
							EnvPrefix: "MCP_",
						},
					},
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				container := dep.Spec.Template.Spec.Containers[0]
				require.Len(t, container.EnvFrom, 1)
				assert.Equal(t, "MCP_", container.EnvFrom[0].Prefix)
				assert.Equal(t, "api-key", container.EnvFrom[0].SecretRef.Name)
			},
		},
		{
			name: "with labels and annotations on selector",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "my-server:v1",
					Transport: gatewayv1alpha1.TransportStdio,
					Labels: map[string]string{
						"custom-label": "custom-value",
					},
					Annotations: map[string]string{
						"custom-annotation": "value",
					},
				},
			},
			check: func(t *testing.T, dep *appsv1.Deployment) {
				// Selector should NOT include custom labels (immutable)
				_, hasCustLabel := dep.Spec.Selector.MatchLabels["custom-label"]
				assert.False(t, hasCustLabel, "custom labels must not be in selector")

				// Pod template should include custom labels
				assert.Equal(t, "custom-value", dep.Spec.Template.Labels["custom-label"])

				// Pod template should include custom annotations
				assert.Equal(t, "value", dep.Spec.Template.Annotations["custom-annotation"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dep := buildDeployment(tt.server)
			require.NotNil(t, dep)
			tt.check(t, dep)
		})
	}
}

func TestBuildService(t *testing.T) {
	tests := []struct {
		name   string
		server *gatewayv1alpha1.MCPServer
		check  func(t *testing.T, svc *corev1.Service)
	}{
		{
			name: "basic service",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "echo",
					Namespace: "default",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "echo:latest",
					Transport: gatewayv1alpha1.TransportStdio,
				},
			},
			check: func(t *testing.T, svc *corev1.Service) {
				assert.Equal(t, "mcp-echo", svc.Name)
				assert.Equal(t, "default", svc.Namespace)
				assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)

				require.Len(t, svc.Spec.Ports, 1)
				assert.Equal(t, "mcp", svc.Spec.Ports[0].Name)
				assert.Equal(t, int32(3000), svc.Spec.Ports[0].Port)
				assert.Equal(t, "mcp", svc.Spec.Ports[0].TargetPort.String())

				// Selector should match deployment selector
				assert.Equal(t, "echo", svc.Spec.Selector[LabelName])
			},
		},
		{
			name: "custom port",
			server: &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom",
					Namespace: "test",
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "server:v1",
					Transport: gatewayv1alpha1.TransportSSE,
					Port:      int32Ptr(8080),
				},
			},
			check: func(t *testing.T, svc *corev1.Service) {
				assert.Equal(t, int32(8080), svc.Spec.Ports[0].Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := buildService(tt.server)
			require.NotNil(t, svc)
			tt.check(t, svc)
		})
	}
}

// TestSelectorLabelsImmutability ensures selector labels remain consistent
// across calls. If selector labels change, existing Deployments break.
func TestSelectorLabelsImmutability(t *testing.T) {
	server := &gatewayv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stable",
			Namespace: "default",
		},
		Spec: gatewayv1alpha1.MCPServerSpec{
			Image:     "server:v1",
			Transport: gatewayv1alpha1.TransportStdio,
		},
	}

	labels1 := selectorLabelsForMCPServer(server)

	// Change user labels -- selector should NOT change
	server.Spec.Labels = map[string]string{"new-label": "new-value"}
	labels2 := selectorLabelsForMCPServer(server)

	assert.Equal(t, labels1, labels2, "selector labels must be stable across spec changes")
}

// TestDeploymentSelectorMatchesPodLabels verifies that the Deployment's selector
// is a subset of the pod template labels.
func TestDeploymentSelectorMatchesPodLabels(t *testing.T) {
	server := &gatewayv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "verify",
			Namespace: "default",
		},
		Spec: gatewayv1alpha1.MCPServerSpec{
			Image:     "server:v1",
			Transport: gatewayv1alpha1.TransportStdio,
			Labels: map[string]string{
				"extra": "label",
			},
		},
	}

	dep := buildDeployment(server)

	for k, v := range dep.Spec.Selector.MatchLabels {
		podVal, exists := dep.Spec.Template.Labels[k]
		assert.True(t, exists, "selector label %s not in pod template", k)
		assert.Equal(t, v, podVal, "selector label %s value mismatch", k)
	}
}

// Import needed for the test file
import appsv1 "k8s.io/api/apps/v1"
```

**Note:** The import for `appsv1` at the bottom is shown for clarity. In the actual file, place it in the import block at the top.

### Quality Gate

- `go test ./internal/controller/ -run TestBuild -v` passes all tests
- `go test ./internal/controller/ -run TestLabel -v` passes all tests
- No test relies on external services or cluster access
- All tests use table-driven pattern

### Testing Command

```bash
# Run unit tests
go test ./internal/controller/ -v -count=1 -run "TestBuild|TestLabel|TestSelector|TestDeployment|TestService"

# Coverage
go test ./internal/controller/ -coverprofile=cover-controller.out -count=1
go tool cover -func=cover-controller.out
```

### Common Pitfalls

- Importing `appsv1` incorrectly. Use `appsv1 "k8s.io/api/apps/v1"` not `"k8s.io/api/apps/v1"`.
- Not testing that selector labels are a subset of pod template labels. If they diverge, the Deployment will never match its pods.
- Testing implementation details instead of behavior. Test what the Deployment looks like, not how it was built.
- Not testing the zero-value/default paths (nil `Replicas`, nil `Port`, etc.).
- Using `assert` where `require` is needed. Use `require` when the test cannot meaningfully continue after a failure (e.g., `require.Len` before indexing).

### Progress Marker

```
[x] 1.4 Unit tests complete
```

---

## Step 1.5: envtest Integration Tests

### Overview

Write integration tests using controller-runtime's envtest framework. These tests run against a real (embedded) API server to verify the full reconciliation loop.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `internal/controller/suite_test.go` | envtest suite setup and teardown |
| `internal/controller/mcpserver_controller_test.go` | Integration test cases |

### Key Code/Config

**internal/controller/suite_test.go:**

```go
//go:build integration
// +build integration

/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = gatewayv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Start the controller manager
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&MCPServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
```

**internal/controller/mcpserver_controller_test.go:**

```go
//go:build integration
// +build integration

/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gatewayv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

const (
	timeout  = 30 * time.Second
	interval = 250 * time.Millisecond
)

var _ = Describe("MCPServer Controller", func() {

	Context("When creating an MCPServer", func() {
		It("should create a Deployment and Service", func() {
			serverName := "test-create"
			namespace := "default"

			server := &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serverName,
					Namespace: namespace,
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "ghcr.io/modelcontextprotocol/echo-server:latest",
					Transport: gatewayv1alpha1.TransportStdio,
				},
			}

			// Create MCPServer
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			// Verify Deployment is created
			depKey := types.NamespacedName{
				Name:      fmt.Sprintf("mcp-%s", serverName),
				Namespace: namespace,
			}
			createdDep := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, depKey, createdDep)
			}, timeout, interval).Should(Succeed())

			Expect(createdDep.Spec.Template.Spec.Containers[0].Image).To(
				Equal("ghcr.io/modelcontextprotocol/echo-server:latest"))
			Expect(*createdDep.Spec.Replicas).To(Equal(int32(1)))

			// Verify Service is created
			svcKey := types.NamespacedName{
				Name:      fmt.Sprintf("mcp-%s", serverName),
				Namespace: namespace,
			}
			createdSvc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, svcKey, createdSvc)
			}, timeout, interval).Should(Succeed())

			Expect(createdSvc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))

			// Verify ownerReference is set on Deployment
			Expect(createdDep.OwnerReferences).To(HaveLen(1))
			Expect(createdDep.OwnerReferences[0].Name).To(Equal(serverName))
			Expect(createdDep.OwnerReferences[0].Kind).To(Equal("MCPServer"))

			// Verify status transitions to Deploying
			updatedServer := &gatewayv1alpha1.MCPServer{}
			Eventually(func() gatewayv1alpha1.MCPServerPhase {
				k8sClient.Get(ctx, types.NamespacedName{
					Name: serverName, Namespace: namespace,
				}, updatedServer)
				return updatedServer.Status.Phase
			}, timeout, interval).Should(Equal(gatewayv1alpha1.MCPServerPhaseDeploying))

			// Verify endpoint is set
			Eventually(func() string {
				k8sClient.Get(ctx, types.NamespacedName{
					Name: serverName, Namespace: namespace,
				}, updatedServer)
				return updatedServer.Status.Endpoint
			}, timeout, interval).ShouldNot(BeEmpty())

			// Cleanup
			Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		})
	})

	Context("When deleting an MCPServer", func() {
		It("should clean up Deployment and Service via ownerReferences", func() {
			serverName := "test-delete"
			namespace := "default"

			server := &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serverName,
					Namespace: namespace,
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "echo:latest",
					Transport: gatewayv1alpha1.TransportStdio,
				},
			}

			// Create
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			// Wait for Deployment
			depKey := types.NamespacedName{
				Name:      fmt.Sprintf("mcp-%s", serverName),
				Namespace: namespace,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, depKey, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			// Delete MCPServer
			Expect(k8sClient.Delete(ctx, server)).To(Succeed())

			// Verify MCPServer is deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: serverName, Namespace: namespace,
				}, &gatewayv1alpha1.MCPServer{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			// Verify Deployment is garbage collected
			// Note: In envtest, garbage collection may not work automatically.
			// In a real cluster, ownerReferences trigger GC.
		})
	})

	Context("When updating an MCPServer spec", func() {
		It("should update the Deployment (rolling update)", func() {
			serverName := "test-update"
			namespace := "default"

			server := &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serverName,
					Namespace: namespace,
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "echo:v1",
					Transport: gatewayv1alpha1.TransportStdio,
				},
			}

			// Create
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			// Wait for Deployment
			depKey := types.NamespacedName{
				Name:      fmt.Sprintf("mcp-%s", serverName),
				Namespace: namespace,
			}
			Eventually(func() error {
				return k8sClient.Get(ctx, depKey, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			// Update image
			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: serverName, Namespace: namespace,
				}, server); err != nil {
					return err
				}
				server.Spec.Image = "echo:v2"
				return k8sClient.Update(ctx, server)
			}, timeout, interval).Should(Succeed())

			// Verify Deployment image is updated
			Eventually(func() string {
				dep := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, depKey, dep); err != nil {
					return ""
				}
				if len(dep.Spec.Template.Spec.Containers) == 0 {
					return ""
				}
				return dep.Spec.Template.Spec.Containers[0].Image
			}, timeout, interval).Should(Equal("echo:v2"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		})
	})

	Context("When an MCPServer has an invalid spec", func() {
		It("should transition to Failed phase", func() {
			serverName := "test-invalid"
			namespace := "default"

			server := &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serverName,
					Namespace: namespace,
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "", // Invalid: empty image
					Transport: gatewayv1alpha1.TransportStdio,
				},
			}

			// Create - this may fail at admission (CRD validation) or during reconciliation
			err := k8sClient.Create(ctx, server)
			if err != nil {
				// CRD validation caught it - that's fine
				return
			}

			// If it got past validation, the controller should set Failed phase
			Eventually(func() gatewayv1alpha1.MCPServerPhase {
				updated := &gatewayv1alpha1.MCPServer{}
				k8sClient.Get(ctx, types.NamespacedName{
					Name: serverName, Namespace: namespace,
				}, updated)
				return updated.Status.Phase
			}, timeout, interval).Should(Equal(gatewayv1alpha1.MCPServerPhaseFailed))

			// Cleanup
			k8sClient.Delete(ctx, server)
		})
	})

	Context("When scaling replicas", func() {
		It("should update the Deployment replicas", func() {
			serverName := "test-scale"
			namespace := "default"

			replicas := int32(1)
			server := &gatewayv1alpha1.MCPServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serverName,
					Namespace: namespace,
				},
				Spec: gatewayv1alpha1.MCPServerSpec{
					Image:     "echo:latest",
					Transport: gatewayv1alpha1.TransportStdio,
					Replicas:  &replicas,
				},
			}

			// Create
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			depKey := types.NamespacedName{
				Name:      fmt.Sprintf("mcp-%s", serverName),
				Namespace: namespace,
			}

			// Wait for Deployment
			Eventually(func() error {
				return k8sClient.Get(ctx, depKey, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			// Scale up
			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{
					Name: serverName, Namespace: namespace,
				}, server); err != nil {
					return err
				}
				newReplicas := int32(3)
				server.Spec.Replicas = &newReplicas
				return k8sClient.Update(ctx, server)
			}, timeout, interval).Should(Succeed())

			// Verify
			Eventually(func() int32 {
				dep := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, depKey, dep); err != nil {
					return -1
				}
				return *dep.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(3)))

			// Cleanup
			Expect(k8sClient.Delete(ctx, server)).To(Succeed())
		})
	})
})
```

### Quality Gate

- `make test-integration` passes all tests
- All four scenarios pass: create, delete, update, invalid spec
- Coverage of controller code is 80%+

### Testing Command

```bash
# Run integration tests
make test-integration

# Or directly:
KUBEBUILDER_ASSETS="$(bin/setup-envtest use release-0.19 --bin-dir bin -p path)" \
  go test ./internal/controller/... -v -count=1 -tags=integration -timeout 5m

# Coverage
KUBEBUILDER_ASSETS="$(bin/setup-envtest use release-0.19 --bin-dir bin -p path)" \
  go test ./internal/controller/... -coverprofile=cover-integration.out -tags=integration
go tool cover -func=cover-integration.out | tail -1
```

### Common Pitfalls

- **Missing CRD path:** The `CRDDirectoryPaths` in `suite_test.go` must point to `config/crd/bases/` where `make manifests` generates CRDs. Run `make manifests` first.
- **envtest binary not installed:** Run `make envtest` to install `setup-envtest`.
- **Test pollution:** Each test should use a unique resource name. Shared names cause conflicts between tests.
- **Eventually timeout:** Tests using `Eventually` need generous timeouts (30s). The controller may take time to reconcile.
- **Garbage collection in envtest:** envtest does not run a garbage collector by default. ownerReference-based cleanup will not work automatically. Test explicit deletion instead.
- **Build tag:** The `//go:build integration` tag ensures these tests only run with `-tags=integration`, not during `make test-unit`.
- **Context cancellation:** Always use the suite's `ctx` and call `cancel()` in `AfterSuite`. Leaking contexts causes goroutine leaks.

### Progress Marker

```
[x] 1.5 envtest integration tests complete
```

---

## Step 1.6: Example CRs

### Overview

Create example MCPServer custom resource YAML files that users can apply to test the operator.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `examples/echo-server.yaml` | Simple echo MCP server (stdio) |
| `examples/github-server.yaml` | GitHub MCP server with secrets |
| `examples/filesystem-server.yaml` | Filesystem MCP server |

### Key Code/Config

**examples/echo-server.yaml:**

```yaml
# Echo MCP Server - simplest possible example
# Deploys the MCP echo server that echoes back any input.
# Usage: kubectl apply -f examples/echo-server.yaml
apiVersion: gateway.mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: echo-server
  namespace: default
  labels:
    example: "true"
spec:
  # The MCP echo server container image
  image: ghcr.io/modelcontextprotocol/echo-server:latest

  # The echo server uses stdio transport
  transport: stdio

  # Single replica is sufficient for testing
  replicas: 1

  # Minimal resource requirements
  resources:
    requests:
      cpu: "50m"
      memory: "64Mi"
    limits:
      cpu: "200m"
      memory: "128Mi"
```

**examples/github-server.yaml:**

```yaml
# GitHub MCP Server - demonstrates secret injection and SSE transport
# Provides tools for interacting with GitHub repositories.
#
# Prerequisites:
#   kubectl create secret generic github-credentials \
#     --from-literal=GITHUB_TOKEN=ghp_your_token_here
#
# Usage: kubectl apply -f examples/github-server.yaml
apiVersion: gateway.mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: github-server
  namespace: default
  labels:
    example: "true"
spec:
  # GitHub MCP server image
  image: ghcr.io/modelcontextprotocol/github-server:latest

  # GitHub server supports SSE transport
  transport: sse

  # SSE endpoint configuration
  protocol:
    sseEndpoint: "/sse"
    messageEndpoint: "/message"
    version: "2025-03-26"

  # Source registry information
  source:
    registry: modelcontextprotocol
    name: github
    version: latest

  # Port the server listens on
  port: 3000

  # Number of replicas
  replicas: 1

  # Inject GitHub token from a Kubernetes Secret
  secrets:
    - name: github-credentials
      envPrefix: ""  # Keys are injected as-is (GITHUB_TOKEN)

  # Resource requirements
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "256Mi"

  # Autoscaling configuration
  scaling:
    minReplicas: 1
    maxReplicas: 5
    targetCPUUtilization: 80

  # Additional labels for this server
  labels:
    team: platform
    mcp-server-type: github
```

**examples/filesystem-server.yaml:**

```yaml
# Filesystem MCP Server - demonstrates volume mounts and custom command
# Provides read access to files within a specified directory.
#
# Usage: kubectl apply -f examples/filesystem-server.yaml
apiVersion: gateway.mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: filesystem-server
  namespace: default
  labels:
    example: "true"
spec:
  # Filesystem MCP server image
  image: ghcr.io/modelcontextprotocol/filesystem-server:latest

  # Filesystem server uses stdio transport
  transport: stdio

  # Custom command to specify the allowed directory
  command: ["node"]
  args: ["dist/index.js", "/data"]

  # Single replica
  replicas: 1

  # Mount a ConfigMap or PVC as the data directory
  # (For this example, the container uses its built-in /data directory)

  # Environment variables
  env:
    - name: LOG_LEVEL
      value: "info"
    - name: MAX_FILE_SIZE
      value: "10485760"  # 10MB

  # Resource requirements
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"

  # Health check configuration
  healthCheck:
    enabled: true
    intervalSeconds: 30
    timeoutSeconds: 5
    failureThreshold: 3

  # Annotations for monitoring
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "3000"
```

### Quality Gate

- All YAML files are valid: `kubectl apply --dry-run=client -f examples/`
- Each example has comments explaining its purpose
- The echo-server example works without any prerequisites
- The github-server example documents its Secret prerequisite

### Testing Command

```bash
# Validate YAML syntax
for f in examples/*.yaml; do
  python3 -c "import yaml; yaml.safe_load(open('$f')); print('OK: $f')"
done

# Dry-run against a cluster (requires CRD to be installed)
kubectl apply --dry-run=client -f examples/echo-server.yaml
kubectl apply --dry-run=client -f examples/github-server.yaml
kubectl apply --dry-run=client -f examples/filesystem-server.yaml

# Full test in Kind
kubectl apply -f examples/echo-server.yaml
kubectl get mcpservers
kubectl describe mcpserver echo-server
kubectl get pods -l app.kubernetes.io/name=echo-server
```

### Common Pitfalls

- Forgetting to install the CRD before applying examples. Run `kubectl apply -f config/crd/bases/` first.
- Using images that do not exist. Verify the image names and tags against the actual container registry.
- Not documenting Secret prerequisites. The github-server example requires a Secret to be created first.
- Using non-standard ports without specifying them in the spec. The default is 3000.
- Including real API keys or tokens in example files. Always use placeholder values.

### Progress Marker

```
[x] 1.6 Example CRs created
```

---

## Step 1.7: E2E Test for Kind

### Overview

Write an end-to-end test that deploys an MCPServer to a Kind cluster and verifies the pod runs and responds to MCP protocol requests.

### Files to Create/Modify

| File | Purpose |
|------|---------|
| `test/e2e/mcpserver_e2e_test.go` | E2E test suite |

### Key Code/Config

**test/e2e/mcpserver_e2e_test.go:**

```go
//go:build e2e
// +build e2e

/*
Copyright 2024 MCP Gateway Contributors.
Licensed under the Apache License, Version 2.0.
*/

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// namespace where the operator and test resources live
	operatorNamespace = "mcp-gateway-system"
	testNamespace     = "default"

	// timeouts
	deployTimeout = 120 * time.Second
	pollInterval  = 5 * time.Second
)

// kubectl runs a kubectl command and returns stdout.
func kubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// waitForCondition polls kubectl until a condition is met or timeout.
func waitForCondition(t *testing.T, timeout time.Duration, check func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

func TestOperatorIsRunning(t *testing.T) {
	out, err := kubectl("-n", operatorNamespace, "get", "pods",
		"-l", "app.kubernetes.io/component=operator",
		"-o", "jsonpath={.items[0].status.phase}")
	require.NoError(t, err, "failed to get operator pod: %s", out)
	assert.Equal(t, "Running", out, "operator pod should be Running")
}

func TestEchoServerLifecycle(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	serverName := "e2e-echo"
	mcpYAML := fmt.Sprintf(`
apiVersion: gateway.mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: %s
  namespace: %s
spec:
  image: ghcr.io/modelcontextprotocol/echo-server:latest
  transport: stdio
  replicas: 1
  resources:
    requests:
      cpu: "50m"
      memory: "64Mi"
    limits:
      cpu: "200m"
      memory: "128Mi"
`, serverName, testNamespace)

	// Step 1: Apply the MCPServer CR
	t.Log("Creating MCPServer...")
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(mcpYAML)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to apply MCPServer: %s", string(out))

	// Cleanup on test completion
	defer func() {
		t.Log("Cleaning up MCPServer...")
		cmd := exec.Command("kubectl", "delete", "-f", "-", "--ignore-not-found")
		cmd.Stdin = strings.NewReader(mcpYAML)
		cmd.CombinedOutput()
	}()

	// Step 2: Wait for the MCPServer to be created
	t.Log("Waiting for MCPServer resource...")
	waitForCondition(t, 30*time.Second, func() bool {
		out, err := kubectl("-n", testNamespace, "get", "mcpserver", serverName)
		return err == nil && strings.Contains(out, serverName)
	}, "MCPServer resource to exist")

	// Step 3: Wait for the Deployment to be created
	deploymentName := fmt.Sprintf("mcp-%s", serverName)
	t.Logf("Waiting for Deployment %s...", deploymentName)
	waitForCondition(t, deployTimeout, func() bool {
		out, err := kubectl("-n", testNamespace, "get", "deployment", deploymentName,
			"-o", "jsonpath={.metadata.name}")
		return err == nil && out == deploymentName
	}, "Deployment to be created")

	// Step 4: Wait for pod to be running
	t.Log("Waiting for pod to be running...")
	waitForCondition(t, deployTimeout, func() bool {
		out, err := kubectl("-n", testNamespace, "get", "pods",
			"-l", fmt.Sprintf("app.kubernetes.io/name=%s", serverName),
			"-o", "jsonpath={.items[0].status.phase}")
		return err == nil && out == "Running"
	}, "pod to be Running")

	// Step 5: Verify the Service exists
	t.Log("Verifying Service...")
	serviceName := fmt.Sprintf("mcp-%s", serverName)
	out, err = kubectl("-n", testNamespace, "get", "service", serviceName,
		"-o", "jsonpath={.spec.type}")
	require.NoError(t, err, "Service should exist: %s", out)
	assert.Equal(t, "ClusterIP", out)

	// Step 6: Verify MCPServer status
	t.Log("Checking MCPServer status...")
	waitForCondition(t, 60*time.Second, func() bool {
		out, err := kubectl("-n", testNamespace, "get", "mcpserver", serverName,
			"-o", "jsonpath={.status.phase}")
		return err == nil && (out == "Running" || out == "Deploying")
	}, "MCPServer to have a phase")

	// Step 7: Verify endpoint is set
	out, err = kubectl("-n", testNamespace, "get", "mcpserver", serverName,
		"-o", "jsonpath={.status.endpoint}")
	require.NoError(t, err)
	if out != "" {
		assert.Contains(t, out, serviceName, "endpoint should contain service name")
		t.Logf("MCPServer endpoint: %s", out)
	}

	// Step 8: Verify owner references on Deployment
	out, err = kubectl("-n", testNamespace, "get", "deployment", deploymentName,
		"-o", "jsonpath={.metadata.ownerReferences[0].kind}")
	require.NoError(t, err)
	assert.Equal(t, "MCPServer", out, "Deployment should have MCPServer owner reference")

	t.Log("Echo server E2E test passed!")
}

func TestMCPServerCustomColumns(t *testing.T) {
	// Verify that `kubectl get mcpservers` shows custom columns
	out, err := kubectl("get", "mcpservers", "-A")
	if err != nil {
		t.Skip("No MCPServers exist yet, skipping custom columns test")
	}

	// Check header contains expected columns
	lines := strings.Split(out, "\n")
	if len(lines) > 0 {
		header := lines[0]
		assert.Contains(t, header, "PHASE", "should have Phase column")
		assert.Contains(t, header, "TRANSPORT", "should have Transport column")
		assert.Contains(t, header, "READY", "should have Ready column")
		assert.Contains(t, header, "AGE", "should have Age column")
	}
}

func TestMCPServerDeletion(t *testing.T) {
	serverName := "e2e-delete-test"
	mcpYAML := fmt.Sprintf(`
apiVersion: gateway.mcp-gateway.io/v1alpha1
kind: MCPServer
metadata:
  name: %s
  namespace: %s
spec:
  image: ghcr.io/modelcontextprotocol/echo-server:latest
  transport: stdio
`, serverName, testNamespace)

	// Create
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(mcpYAML)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to create: %s", string(out))

	// Wait for Deployment
	deploymentName := fmt.Sprintf("mcp-%s", serverName)
	waitForCondition(t, deployTimeout, func() bool {
		_, err := kubectl("-n", testNamespace, "get", "deployment", deploymentName)
		return err == nil
	}, "Deployment to exist")

	// Delete
	t.Log("Deleting MCPServer...")
	cmd = exec.Command("kubectl", "delete", "-f", "-")
	cmd.Stdin = strings.NewReader(mcpYAML)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed to delete: %s", string(out))

	// Verify MCPServer is gone
	waitForCondition(t, 60*time.Second, func() bool {
		_, err := kubectl("-n", testNamespace, "get", "mcpserver", serverName)
		return err != nil // Should not be found
	}, "MCPServer to be deleted")

	// Verify Deployment is cleaned up (via ownerReferences)
	waitForCondition(t, 60*time.Second, func() bool {
		_, err := kubectl("-n", testNamespace, "get", "deployment", deploymentName)
		return err != nil // Should not be found
	}, "Deployment to be garbage collected")

	t.Log("Deletion E2E test passed!")
}
```

### Quality Gate

- `make test-e2e` passes against a Kind cluster with the operator deployed
- Echo server pod reaches Running state
- Deployment and Service are created with correct ownerReferences
- Deletion cascades properly

### Testing Command

```bash
# Full E2E sequence
make kind-up
make dev-deploy

# Run E2E tests
make test-e2e

# Or directly:
go test ./test/e2e/... -v -count=1 -timeout 15m -tags=e2e

# Cleanup
make kind-down
```

### Common Pitfalls

- Running E2E tests without the CRD installed. The operator Helm chart should install CRDs, or install them separately with `kubectl apply -f config/crd/bases/`.
- Image pull failures in Kind. The echo-server image must be pullable from within the cluster. Pre-load it with `docker pull && kind load docker-image` if the cluster has no internet access.
- Test pollution between runs. Always use unique resource names or clean up with `defer`.
- Forgetting the `//go:build e2e` tag. Without it, E2E tests run during `make test-unit` and fail (no cluster).
- `kubectl` not configured to talk to the Kind cluster. Ensure `KUBECONFIG` points to the Kind kubeconfig.
- Tests failing due to timeouts in CI. The deploy timeout (120s) may not be enough in slow CI environments. Increase to 300s if needed.

### Progress Marker

```
[x] 1.7 E2E test complete
```

---

## Phase 1 Checklist

```
[ ] 1.1 MCPServer CRD types defined
[ ] 1.2 MCPServerReconciler implemented
[ ] 1.3 MCP discovery client implemented
[ ] 1.4 Unit tests complete
[ ] 1.5 envtest integration tests complete
[ ] 1.6 Example CRs created
[ ] 1.7 E2E test complete
```

## Validation Sequence

Run these commands in order to validate the entire phase:

```bash
# 1. Generate code and manifests
make generate manifests
echo "Code generation: $?"

# 2. Build compiles
make build
echo "Build: $?"

# 3. Lint passes
make lint
echo "Lint: $?"

# 4. Unit tests pass
make test-unit
echo "Unit tests: $?"

# 5. Check unit test coverage
go tool cover -func=cover.out | tail -1
# Should be >= 80%

# 6. Integration tests pass
make test-integration
echo "Integration tests: $?"

# 7. Docker image builds
make docker-build
echo "Docker: $?"

# 8. Helm chart is valid
make helm-lint
echo "Helm: $?"

# 9. Kind cluster comes up
make kind-up
echo "Kind: $?"

# 10. Operator deploys
make dev-deploy
echo "Deploy: $?"

# 11. Apply example CRs
kubectl apply -f examples/echo-server.yaml
echo "Example CR: $?"

# 12. Wait for echo server pod
kubectl wait --for=condition=available deployment/mcp-echo-server \
  --timeout=120s -n default
echo "Echo server ready: $?"

# 13. Verify MCPServer status
kubectl get mcpservers
kubectl describe mcpserver echo-server

# 14. Run E2E tests
make test-e2e
echo "E2E: $?"

# 15. Cleanup
kubectl delete -f examples/echo-server.yaml
make kind-down
echo "Cleanup: $?"
```

**All commands should exit with code 0. If any fail, fix the issue before proceeding to Phase 2.**
