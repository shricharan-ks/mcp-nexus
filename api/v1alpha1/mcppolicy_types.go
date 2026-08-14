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

// +kubebuilder:validation:Enum=ALLOW;DENY
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "ALLOW"
	PolicyEffectDeny  PolicyEffect = "DENY"
)

// +kubebuilder:validation:Enum=Pending;Synced;Failed
type MCPPolicyPhase string

const (
	MCPPolicyPhasePending MCPPolicyPhase = "Pending"
	MCPPolicySynced       MCPPolicyPhase = "Synced"
	MCPPolicyPhaseFailed  MCPPolicyPhase = "Failed"
)

// PolicyPrincipals defines who a policy rule applies to.
type PolicyPrincipals struct {
	// +optional
	Roles []string `json:"roles,omitempty"`

	// +optional
	AgentRefs []LocalObjectReference `json:"agentRefs,omitempty"`
}

// PolicyResources defines the scope of resources a policy rule targets.
type PolicyResources struct {
	// +optional
	ServerRef *LocalObjectReference `json:"serverRef,omitempty"`

	// +optional
	Tools []string `json:"tools,omitempty"`
}

// PolicyRule defines a single policy rule with effect, principals, actions, and resources.
type PolicyRule struct {
	// +kubebuilder:validation:Required
	Effect PolicyEffect `json:"effect"`

	// +optional
	Principals PolicyPrincipals `json:"principals,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Actions []string `json:"actions"`

	// +optional
	Resources PolicyResources `json:"resources,omitempty"`
}

// MCPPolicySpec defines the desired state of MCPPolicy.
type MCPPolicySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Rules []PolicyRule `json:"rules"`
}

// MCPPolicyStatus defines the observed state of MCPPolicy.
type MCPPolicyStatus struct {
	// +optional
	CerbosPolicyID string `json:"cerbosPolicyId,omitempty"`

	// +optional
	SyncedAt *metav1.Time `json:"syncedAt,omitempty"`

	// +kubebuilder:default="Pending"
	Phase MCPPolicyPhase `json:"phase,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcpp
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Cerbos ID",type="string",JSONPath=".status.cerbosPolicyId"
// +kubebuilder:printcolumn:name="Synced",type="date",JSONPath=".status.syncedAt"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MCPPolicy is the Schema for the mcppolicies API.
type MCPPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPPolicySpec   `json:"spec,omitempty"`
	Status MCPPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPPolicyList contains a list of MCPPolicy.
type MCPPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPPolicy{}, &MCPPolicyList{})
}
