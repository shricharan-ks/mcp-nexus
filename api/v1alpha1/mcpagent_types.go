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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Pending;Registering;Active;Suspended;Failed
type MCPAgentPhase string

const (
	MCPAgentPhasePending     MCPAgentPhase = "Pending"
	MCPAgentPhaseRegistering MCPAgentPhase = "Registering"
	MCPAgentPhaseActive      MCPAgentPhase = "Active"
	MCPAgentPhaseSuspended   MCPAgentPhase = "Suspended"
	MCPAgentPhaseFailed      MCPAgentPhase = "Failed"
)

// LocalObjectReference contains enough information to locate the referenced
// Kubernetes resource object within the same namespace.
type LocalObjectReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// MCPAgentIdentity defines the OIDC identity for an agent.
type MCPAgentIdentity struct {
	// OIDCClientID is the OIDC client identifier for this agent.
	// Defaults to "agent-<name>" if not specified.
	// +optional
	OIDCClientID string `json:"oidcClientId,omitempty"`
}

// ServerAccessEntry defines access to a single MCP server with an optional policy.
type ServerAccessEntry struct {
	// +kubebuilder:validation:Required
	ServerRef LocalObjectReference `json:"serverRef"`

	// +optional
	PolicyRef *LocalObjectReference `json:"policyRef,omitempty"`
}

// RateLimitEntry defines a rate limit as requests per minute.
type RateLimitEntry struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100000
	RequestsPerMinute int32 `json:"requestsPerMinute"`
}

// ToolRateLimitEntry defines a per-tool rate limit.
type ToolRateLimitEntry struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Tool string `json:"tool"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100000
	RequestsPerMinute int32 `json:"requestsPerMinute"`
}

// AgentRateLimits defines rate limiting for an agent.
type AgentRateLimits struct {
	// +optional
	Global *RateLimitEntry `json:"global,omitempty"`

	// +optional
	PerTool []ToolRateLimitEntry `json:"perTool,omitempty"`
}

// AgentQuota defines quota limits for an agent.
type AgentQuota struct {
	// +optional
	MaxConcurrentConnections *int32 `json:"maxConcurrentConnections,omitempty"`

	// +optional
	MaxMonthlyToolCalls *int64 `json:"maxMonthlyToolCalls,omitempty"`
}

// MCPAgentSpec defines the desired state of MCPAgent.
type MCPAgentSpec struct {
	// +optional
	Identity MCPAgentIdentity `json:"identity,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	ServerAccess []ServerAccessEntry `json:"serverAccess"`

	// +optional
	RateLimits *AgentRateLimits `json:"rateLimits,omitempty"`

	// +optional
	Quota *AgentQuota `json:"quota,omitempty"`
}

// MCPAgentStatus defines the observed state of MCPAgent.
type MCPAgentStatus struct {
	// +kubebuilder:default="Pending"
	Phase MCPAgentPhase `json:"phase,omitempty"`

	// +optional
	RegisteredAt *metav1.Time `json:"registeredAt,omitempty"`

	// +optional
	ClientSecretRef *LocalObjectReference `json:"clientSecretRef,omitempty"`

	CurrentMonthToolCalls int64 `json:"currentMonthToolCalls,omitempty"`

	ActiveConnections int32 `json:"activeConnections,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcpa
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Client ID",type="string",JSONPath=".spec.identity.oidcClientId"
// +kubebuilder:printcolumn:name="Monthly Calls",type="integer",JSONPath=".status.currentMonthToolCalls"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MCPAgent is the Schema for the mcpagents API.
type MCPAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPAgentSpec   `json:"spec,omitempty"`
	Status MCPAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPAgentList contains a list of MCPAgent.
type MCPAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPAgent{}, &MCPAgentList{})
}
