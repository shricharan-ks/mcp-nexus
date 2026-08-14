/*
Copyright 2026 The MCP Gateway Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Pending;Deploying;Running;Updating;Scaling;Failed;Terminating
type MCPServerPhase string

const (
	MCPServerPhasePending     MCPServerPhase = "Pending"
	MCPServerPhaseDeploying   MCPServerPhase = "Deploying"
	MCPServerPhaseRunning     MCPServerPhase = "Running"
	MCPServerPhaseUpdating    MCPServerPhase = "Updating"
	MCPServerPhaseScaling     MCPServerPhase = "Scaling"
	MCPServerPhaseFailed      MCPServerPhase = "Failed"
	MCPServerPhaseTerminating MCPServerPhase = "Terminating"
)

// +kubebuilder:validation:Enum=streamable-http;stdio
type TransportType string

const (
	TransportStreamableHTTP TransportType = "streamable-http"
	TransportStdio          TransportType = "stdio"
)

type MCPServerSpec struct {
	// +kubebuilder:validation:Required
	Source MCPServerSource `json:"source"`

	// +kubebuilder:validation:Required
	Protocol MCPServerProtocol `json:"protocol"`

	// +optional
	Scaling *MCPServerScaling `json:"scaling,omitempty"`

	// +optional
	Secrets []MCPServerSecret `json:"secrets,omitempty"`

	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty"`

	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
}

type MCPServerSource struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	Port int32 `json:"port"`

	// +optional
	HealthCheck *MCPServerHealthCheck `json:"healthCheck,omitempty"`
}

type MCPServerHealthCheck struct {
	// +kubebuilder:default="/health"
	Path string `json:"path"`

	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	PeriodSeconds int32 `json:"periodSeconds"`
}

type MCPServerProtocol struct {
	// +kubebuilder:default="streamable-http"
	Transport TransportType `json:"transport"`

	// +kubebuilder:default="/mcp"
	Endpoint string `json:"endpoint"`
}

type MCPServerScaling struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=10
	MaxReplicas int32 `json:"maxReplicas"`

	// +optional
	ScaleToZero *ScaleToZeroConfig `json:"scaleToZero,omitempty"`
}

type ScaleToZeroConfig struct {
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// +kubebuilder:default=300
	// +kubebuilder:validation:Minimum=30
	IdleTimeoutSeconds int32 `json:"idleTimeoutSeconds,omitempty"`
}

type MCPServerSecret struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	EnvVar string `json:"envVar"`

	// +kubebuilder:validation:Required
	SecretRef SecretKeyRef `json:"secretRef"`
}

type SecretKeyRef struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type MCPServerStatus struct {
	// +kubebuilder:default="Pending"
	Phase MCPServerPhase `json:"phase,omitempty"`

	Replicas      int32 `json:"replicas,omitempty"`
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// +optional
	DiscoveredCapabilities *DiscoveredCapabilities `json:"discoveredCapabilities,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	DeploymentName string `json:"deploymentName,omitempty"`

	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`
}

type DiscoveredCapabilities struct {
	// +optional
	Tools []string `json:"tools,omitempty"`

	// +optional
	Resources []string `json:"resources,omitempty"`

	// +optional
	Prompts []string `json:"prompts,omitempty"`

	// +optional
	LastDiscoveredAt *metav1.Time `json:"lastDiscoveredAt,omitempty"`

	// +optional
	CacheTTLMs int64 `json:"cacheTTLMs,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcps
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.source.image"
// +kubebuilder:printcolumn:name="Transport",type="string",JSONPath=".spec.protocol.transport"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPServerSpec   `json:"spec,omitempty"`
	Status MCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type MCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPServer{}, &MCPServerList{})
}
