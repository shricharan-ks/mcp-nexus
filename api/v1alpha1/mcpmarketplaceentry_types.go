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

// +kubebuilder:validation:Enum=developer-tools;data;communication;productivity;security;infrastructure;ai-ml;custom
type MarketplaceCategory string

const (
	CategoryDeveloperTools MarketplaceCategory = "developer-tools"
	CategoryData           MarketplaceCategory = "data"
	CategoryCommunication  MarketplaceCategory = "communication"
	CategoryProductivity   MarketplaceCategory = "productivity"
	CategorySecurity       MarketplaceCategory = "security"
	CategoryInfrastructure MarketplaceCategory = "infrastructure"
	CategoryAIML           MarketplaceCategory = "ai-ml"
	CategoryCustom         MarketplaceCategory = "custom"
)

// +kubebuilder:validation:Enum=passed;failed;warning;pending;not-scanned
type ScanStatus string

const (
	ScanStatusPassed     ScanStatus = "passed"
	ScanStatusFailed     ScanStatus = "failed"
	ScanStatusWarning    ScanStatus = "warning"
	ScanStatusPending    ScanStatus = "pending"
	ScanStatusNotScanned ScanStatus = "not-scanned"
)

// +kubebuilder:validation:Enum=Active;Deprecated;Blocked;PendingScan
type MarketplaceEntryPhase string

const (
	MarketplaceEntryPhaseActive      MarketplaceEntryPhase = "Active"
	MarketplaceEntryPhaseDeprecated  MarketplaceEntryPhase = "Deprecated"
	MarketplaceEntryPhaseBlocked     MarketplaceEntryPhase = "Blocked"
	MarketplaceEntryPhasePendingScan MarketplaceEntryPhase = "PendingScan"
)

// MarketplaceSource defines the container image source for a marketplace entry.
type MarketplaceSource struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +optional
	SignatureRef string `json:"signatureRef,omitempty"`

	// +optional
	Digest string `json:"digest,omitempty"`
}

// RequiredSecret describes a secret that must be provided when installing from the marketplace.
type RequiredSecret struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// +optional
	Description string `json:"description,omitempty"`
}

// DefaultPolicy defines a default policy to apply when deploying from the marketplace.
type DefaultPolicy struct {
	// +optional
	AllowedTools []string `json:"allowedTools,omitempty"`

	// +optional
	DeniedTools []string `json:"deniedTools,omitempty"`
}

// InstallTemplate defines the resources to create when installing a marketplace entry.
type InstallTemplate struct {
	// +kubebuilder:validation:Required
	MCPServerSpec MCPServerSpec `json:"mcpServerSpec"`

	// +optional
	RequiredSecrets []RequiredSecret `json:"requiredSecrets,omitempty"`

	// +optional
	DefaultPolicy *DefaultPolicy `json:"defaultPolicy,omitempty"`
}

// SecurityInfo contains security scan information for a marketplace entry.
type SecurityInfo struct {
	// +kubebuilder:default="not-scanned"
	ScanStatus ScanStatus `json:"scanStatus,omitempty"`

	// +optional
	LastScannedAt *metav1.Time `json:"lastScannedAt,omitempty"`

	// +optional
	CVECount int `json:"cveCount,omitempty"`

	// +optional
	CriticalCVECount int `json:"criticalCveCount,omitempty"`

	// +optional
	SBOMRef string `json:"sbomRef,omitempty"`
}

// MCPMarketplaceEntrySpec defines the desired state of MCPMarketplaceEntry.
type MCPMarketplaceEntrySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Vendor string `json:"vendor"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?(\+[a-zA-Z0-9.]+)?$`
	Version string `json:"version"`

	// +optional
	// +kubebuilder:validation:MaxLength=500
	Description string `json:"description,omitempty"`

	// +kubebuilder:validation:Required
	Category MarketplaceCategory `json:"category"`

	// +optional
	Tags []string `json:"tags,omitempty"`

	// +optional
	Homepage string `json:"homepage,omitempty"`

	// +optional
	DocumentationURL string `json:"documentationUrl,omitempty"`

	// +kubebuilder:validation:Required
	Source MarketplaceSource `json:"source"`

	// +kubebuilder:validation:Required
	InstallTemplate InstallTemplate `json:"installTemplate"`

	// +optional
	Security SecurityInfo `json:"security,omitempty"`

	// +optional
	Verified bool `json:"verified,omitempty"`

	// +optional
	Deprecated bool `json:"deprecated,omitempty"`
}

// MCPMarketplaceEntryStatus defines the observed state of MCPMarketplaceEntry.
type MCPMarketplaceEntryStatus struct {
	// +kubebuilder:default="PendingScan"
	Phase MarketplaceEntryPhase `json:"phase,omitempty"`

	// +optional
	InstallCount int64 `json:"installCount,omitempty"`

	// +optional
	LastInstalledAt *metav1.Time `json:"lastInstalledAt,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mcpme
// +kubebuilder:printcolumn:name="Display Name",type="string",JSONPath=".spec.displayName"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Category",type="string",JSONPath=".spec.category"
// +kubebuilder:printcolumn:name="Scan",type="string",JSONPath=".spec.security.scanStatus"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// MCPMarketplaceEntry is the Schema for the mcpmarketplaceentries API.
type MCPMarketplaceEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPMarketplaceEntrySpec   `json:"spec,omitempty"`
	Status MCPMarketplaceEntryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPMarketplaceEntryList contains a list of MCPMarketplaceEntry.
type MCPMarketplaceEntryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPMarketplaceEntry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPMarketplaceEntry{}, &MCPMarketplaceEntryList{})
}
