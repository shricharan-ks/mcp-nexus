# Phase 3: Agent & RBAC (Weeks 10-13) -- MVP Complete

**Goal:** `MCPAgent` and `MCPPolicy` CRDs enforced by Cerbos deliver
per-agent, per-tool access control with audit logging.

By the end of this phase the system supports the full lifecycle: an admin
creates an `MCPAgent` CR, the operator provisions Keycloak credentials, a
`MCPPolicy` grants the agent access to specific tools on specific servers, and
every decision is audited.

---

## 3.1 MCPAgent CRD

### Go Type Definition

`api/v1alpha1/mcpagent_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPAgentSpec defines the desired state of an MCP agent identity.
type MCPAgentSpec struct {
	// Identity describes who this agent is.
	Identity AgentIdentity `json:"identity"`

	// ServerAccess lists the MCP servers this agent may contact.
	ServerAccess []ServerAccessEntry `json:"serverAccess"`

	// RateLimits optionally overrides the global rate limits for this agent.
	// +optional
	RateLimits *AgentRateLimits `json:"rateLimits,omitempty"`
}

// AgentIdentity holds the identity metadata for an agent.
type AgentIdentity struct {
	// DisplayName is a human-readable name for the agent.
	DisplayName string `json:"displayName"`

	// Description provides additional context about the agent's purpose.
	// +optional
	Description string `json:"description,omitempty"`

	// Labels are arbitrary key-value pairs for organization.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Owner identifies the team or individual responsible for this agent.
	// +optional
	Owner string `json:"owner,omitempty"`
}

// ServerAccessEntry references an MCPServer and an optional MCPPolicy.
type ServerAccessEntry struct {
	// ServerRef is the name of the MCPServer CR this agent can access.
	ServerRef string `json:"serverRef"`

	// PolicyRef is the name of the MCPPolicy CR governing access.
	// +optional
	PolicyRef string `json:"policyRef,omitempty"`

	// AllowedTools restricts the agent to a subset of the server's tools.
	// An empty list means all tools are allowed (subject to MCPPolicy).
	// +optional
	AllowedTools []string `json:"allowedTools,omitempty"`
}

// AgentRateLimits defines rate limiting overrides for this agent.
type AgentRateLimits struct {
	// PerTool sets the maximum requests per minute for each tool invocation.
	// +optional
	PerTool int `json:"perTool,omitempty"`

	// PerServer sets the maximum requests per minute across all tools on a
	// single server.
	// +optional
	PerServer int `json:"perServer,omitempty"`

	// Global sets the maximum requests per minute across all servers.
	// +optional
	Global int `json:"global,omitempty"`

	// Quota defines monthly usage limits.
	// +optional
	Quota *AgentQuota `json:"quota,omitempty"`
}

// AgentQuota defines monthly usage quotas.
type AgentQuota struct {
	// MonthlyRequests is the maximum total requests allowed per calendar month.
	MonthlyRequests int64 `json:"monthlyRequests"`

	// AlertThresholdPercent triggers a warning when this percentage of the
	// monthly quota has been consumed.  Defaults to 80.
	// +optional
	// +kubebuilder:default=80
	AlertThresholdPercent int `json:"alertThresholdPercent,omitempty"`
}

// MCPAgentPhase describes the lifecycle phase of the agent.
// +kubebuilder:validation:Enum=Pending;Registering;Active;Error
type MCPAgentPhase string

const (
	AgentPhasePending     MCPAgentPhase = "Pending"
	AgentPhaseRegistering MCPAgentPhase = "Registering"
	AgentPhaseActive      MCPAgentPhase = "Active"
	AgentPhaseError       MCPAgentPhase = "Error"
)

// MCPAgentStatus defines the observed state of an MCP agent.
type MCPAgentStatus struct {
	// Phase is the current lifecycle phase.
	Phase MCPAgentPhase `json:"phase,omitempty"`

	// KeycloakClientID is the Keycloak client ID once registration completes.
	// +optional
	KeycloakClientID string `json:"keycloakClientID,omitempty"`

	// CredentialSecretRef is the name of the Kubernetes Secret holding the
	// client_id and client_secret.
	// +optional
	CredentialSecretRef string `json:"credentialSecretRef,omitempty"`

	// Conditions tracks detailed status information.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastTransitionTime records when the phase last changed.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// QuotaUsed tracks the current month's total request count.
	// +optional
	QuotaUsed int64 `json:"quotaUsed,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.identity.displayName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MCPAgent represents an AI agent identity within the MCP Gateway.
type MCPAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPAgentSpec   `json:"spec,omitempty"`
	Status MCPAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPAgentList contains a list of MCPAgent resources.
type MCPAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPAgent{}, &MCPAgentList{})
}
```

### Validation Webhook

A validating admission webhook in `internal/webhook/mcpagent_webhook.go`
ensures:

1. Every `serverRef` in `spec.serverAccess` corresponds to an existing
   `MCPServer` CR in the same namespace.
2. Every `policyRef` (if set) corresponds to an existing `MCPPolicy` CR.
3. `rateLimits.perTool` is less than or equal to `rateLimits.perServer`, which
   is less than or equal to `rateLimits.global` (if all are specified).
4. `quota.alertThresholdPercent` is between 1 and 100.

---

## 3.2 MCPPolicy CRD

### Go Type Definition

`api/v1alpha1/mcppolicy_types.go`:

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPPolicySpec defines the desired access control rules for MCP resources.
type MCPPolicySpec struct {
	// Description explains the intent of this policy.
	// +optional
	Description string `json:"description,omitempty"`

	// Rules is the ordered list of access control rules.
	// Rules are evaluated top-down; the first matching rule wins.
	Rules []PolicyRule `json:"rules"`
}

// Effect is either Allow or Deny.
// +kubebuilder:validation:Enum=Allow;Deny
type Effect string

const (
	EffectAllow Effect = "Allow"
	EffectDeny  Effect = "Deny"
)

// PolicyRule defines a single access control rule.
type PolicyRule struct {
	// Name is a human-readable identifier for this rule.
	Name string `json:"name"`

	// Effect determines whether matching requests are allowed or denied.
	Effect Effect `json:"effect"`

	// Principals identifies who this rule applies to.
	// Supports exact agent names and glob patterns (e.g. "team-a-*").
	Principals []string `json:"principals"`

	// Actions lists the MCP methods this rule governs.
	// Examples: "tools/call", "tools/list", "resources/read", "*".
	Actions []string `json:"actions"`

	// Resources lists the MCP resources (tool names, resource URIs) this
	// rule governs. Supports glob patterns (e.g. "db-*").
	Resources []string `json:"resources"`

	// Conditions are optional additional constraints.
	// +optional
	Conditions *RuleConditions `json:"conditions,omitempty"`
}

// RuleConditions provides additional context-based constraints on a rule.
type RuleConditions struct {
	// TimeWindow restricts the rule to certain hours (cron-like).
	// +optional
	TimeWindow *TimeWindow `json:"timeWindow,omitempty"`

	// IPRanges restricts the rule to requests from specific CIDRs.
	// +optional
	IPRanges []string `json:"ipRanges,omitempty"`

	// RequireLabels matches only agents that carry all the listed labels.
	// +optional
	RequireLabels map[string]string `json:"requireLabels,omitempty"`
}

// TimeWindow restricts rule evaluation to specific times.
type TimeWindow struct {
	// Timezone for interpreting Start/End. Defaults to UTC.
	// +optional
	// +kubebuilder:default="UTC"
	Timezone string `json:"timezone,omitempty"`

	// Start is an HH:MM 24-hour time string.
	Start string `json:"start"`

	// End is an HH:MM 24-hour time string.
	End string `json:"end"`

	// Days lists the days of the week the window applies to.
	// +optional
	Days []string `json:"days,omitempty"`
}

// MCPPolicyStatus defines the observed state of the policy.
type MCPPolicyStatus struct {
	// CerbosPolicyID is the ID of the policy in the Cerbos PDP.
	// +optional
	CerbosPolicyID string `json:"cerbosPolicyID,omitempty"`

	// SyncedAt records when the policy was last synced to Cerbos.
	// +optional
	SyncedAt *metav1.Time `json:"syncedAt,omitempty"`

	// Conditions tracks detailed status information.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Rules",type=integer,JSONPath=`.spec.rules | length`
// +kubebuilder:printcolumn:name="Synced",type=date,JSONPath=`.status.syncedAt`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MCPPolicy defines access control rules governing which agents can
// invoke which tools on which MCP servers.
type MCPPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPPolicySpec   `json:"spec,omitempty"`
	Status MCPPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPPolicyList contains a list of MCPPolicy resources.
type MCPPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPPolicy{}, &MCPPolicyList{})
}
```

### Cerbos Policy Translation

`internal/cerbos/translate.go` converts `MCPPolicy` CRs into Cerbos resource
policies and pushes them via the Admin API.

```go
package cerbos

import (
	"fmt"
	"strings"

	policyv1 "github.com/cerbos/cerbos/api/genpb/cerbos/policy/v1"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"

	mcpv1alpha1 "github.com/mcp-gateway/api/v1alpha1"
)

const (
	CerbosResourceKind = "mcp:tool"
	PolicyVersion      = "default"
)

// TranslatePolicy converts an MCPPolicy CR into a Cerbos ResourcePolicy.
func TranslatePolicy(policy *mcpv1alpha1.MCPPolicy) (*policyv1.Policy, error) {
	if len(policy.Spec.Rules) == 0 {
		return nil, fmt.Errorf("policy %s/%s has no rules", policy.Namespace, policy.Name)
	}

	cerbosRules := make([]*policyv1.ResourceRule, 0, len(policy.Spec.Rules))

	for i, rule := range policy.Spec.Rules {
		effect, err := translateEffect(rule.Effect)
		if err != nil {
			return nil, fmt.Errorf("rule %d (%s): %w", i, rule.Name, err)
		}

		cerbosRule := &policyv1.ResourceRule{
			Actions:   translateActions(rule.Actions),
			Effect:    effect,
			Roles:     []string{"mcp-agent"}, // all agents share one Cerbos role
			DerivedRoles: translatePrincipals(rule.Principals),
		}

		// Add condition if time window or IP ranges specified
		if rule.Conditions != nil {
			cerbosRule.Condition = translateConditions(rule.Conditions)
		}

		cerbosRules = append(cerbosRules, cerbosRule)
	}

	resourcePolicy := &policyv1.Policy{
		ApiVersion: "api.cerbos.dev/v1",
		PolicyType: &policyv1.Policy_ResourcePolicy{
			ResourcePolicy: &policyv1.ResourcePolicy{
				Resource: CerbosResourceKind,
				Version:  PolicyVersion,
				ImportDerivedRoles: []string{
					fmt.Sprintf("mcp_%s_derived_roles", policy.Name),
				},
				Rules: cerbosRules,
			},
		},
		Metadata: &policyv1.Metadata{
			SourceAttributes: &policyv1.SourceAttributes{
				Attributes: map[string]*policyv1.SourceAttributes_SourceAttributeValue{
					"mcp-policy-name":      {Value: stringValue(policy.Name)},
					"mcp-policy-namespace": {Value: stringValue(policy.Namespace)},
				},
			},
		},
	}

	return resourcePolicy, nil
}

// TranslateDerivedRoles creates Cerbos DerivedRoles that map MCPPolicy
// principals (agent names / glob patterns) to Cerbos roles.
func TranslateDerivedRoles(policy *mcpv1alpha1.MCPPolicy) *policyv1.Policy {
	definitions := make([]*policyv1.RoleDef, 0)

	seen := make(map[string]bool)
	for _, rule := range policy.Spec.Rules {
		for _, principal := range rule.Principals {
			if seen[principal] {
				continue
			}
			seen[principal] = true

			roleDef := &policyv1.RoleDef{
				Name:        sanitizeRoleName(principal),
				ParentRoles: []string{"mcp-agent"},
			}

			// If the principal is a glob, match via condition on agent_id
			if strings.Contains(principal, "*") {
				roleDef.Condition = &policyv1.Condition{
					Condition: &policyv1.Condition_Match{
						Match: &policyv1.Match{
							Op: &policyv1.Match_Expr{
								Expr: fmt.Sprintf(
									"P.attr.agent_id.matches(\"%s\")",
									globToRegex(principal),
								),
							},
						},
					},
				}
			} else {
				roleDef.Condition = &policyv1.Condition{
					Condition: &policyv1.Condition_Match{
						Match: &policyv1.Match{
							Op: &policyv1.Match_Expr{
								Expr: fmt.Sprintf(
									"P.attr.agent_id == \"%s\"",
									principal,
								),
							},
						},
					},
				}
			}

			definitions = append(definitions, roleDef)
		}
	}

	return &policyv1.Policy{
		ApiVersion: "api.cerbos.dev/v1",
		PolicyType: &policyv1.Policy_DerivedRoles{
			DerivedRoles: &policyv1.DerivedRoles{
				Name:        fmt.Sprintf("mcp_%s_derived_roles", policy.Name),
				Definitions: definitions,
			},
		},
	}
}

func translateEffect(e mcpv1alpha1.Effect) (effectv1.Effect, error) {
	switch e {
	case mcpv1alpha1.EffectAllow:
		return effectv1.Effect_EFFECT_ALLOW, nil
	case mcpv1alpha1.EffectDeny:
		return effectv1.Effect_EFFECT_DENY, nil
	default:
		return effectv1.Effect_EFFECT_UNSPECIFIED, fmt.Errorf("unknown effect: %s", e)
	}
}

func translateActions(actions []string) []string {
	result := make([]string, len(actions))
	for i, a := range actions {
		// Normalize MCP method names to Cerbos action format
		result[i] = strings.ReplaceAll(a, "/", ":")
	}
	return result
}

func translatePrincipals(principals []string) []string {
	result := make([]string, len(principals))
	for i, p := range principals {
		result[i] = sanitizeRoleName(p)
	}
	return result
}

func translateConditions(cond *mcpv1alpha1.RuleConditions) *policyv1.Condition {
	var exprs []string

	if cond.TimeWindow != nil {
		exprs = append(exprs, fmt.Sprintf(
			"now().getHours() >= %s && now().getHours() < %s",
			strings.Split(cond.TimeWindow.Start, ":")[0],
			strings.Split(cond.TimeWindow.End, ":")[0],
		))
	}

	if len(cond.IPRanges) > 0 {
		for _, cidr := range cond.IPRanges {
			exprs = append(exprs, fmt.Sprintf(
				"R.attr.source_ip.inIPAddrRange(\"%s\")", cidr,
			))
		}
	}

	if len(cond.RequireLabels) > 0 {
		for k, v := range cond.RequireLabels {
			exprs = append(exprs, fmt.Sprintf(
				"P.attr.labels[\"%s\"] == \"%s\"", k, v,
			))
		}
	}

	if len(exprs) == 0 {
		return nil
	}

	// AND all expressions
	combined := strings.Join(exprs, " && ")
	return &policyv1.Condition{
		Condition: &policyv1.Condition_Match{
			Match: &policyv1.Match{
				Op: &policyv1.Match_Expr{
					Expr: combined,
				},
			},
		},
	}
}

// globToRegex converts simple glob patterns (with *) to regex.
func globToRegex(glob string) string {
	escaped := strings.ReplaceAll(glob, ".", "\\.")
	return "^" + strings.ReplaceAll(escaped, "*", ".*") + "$"
}

// sanitizeRoleName converts a principal name into a valid Cerbos role name.
func sanitizeRoleName(name string) string {
	r := strings.NewReplacer("*", "_wildcard", "-", "_", ".", "_")
	return r.Replace(name)
}

// stringValue is a helper to create a SourceAttributeValue from a string.
func stringValue(s string) *policyv1.SourceAttributes_SourceAttributeValue {
	return &policyv1.SourceAttributes_SourceAttributeValue{
		// Implementation depends on the Cerbos API version
	}
}
```

---

## 3.3 MCPAgentReconciler

The reconciler drives the agent through its lifecycle phases:
`Pending -> Registering -> Active` (or `Error`).

### Keycloak Go Client

`internal/keycloak/client.go`:

```go
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client wraps the Keycloak Admin REST API.
type Client struct {
	baseURL    string
	realm      string
	httpClient *http.Client

	mu          sync.Mutex
	adminToken  string
	tokenExpiry time.Time
}

// ClientRepresentation mirrors the Keycloak client JSON structure.
type ClientRepresentation struct {
	ID                       string            `json:"id,omitempty"`
	ClientID                 string            `json:"clientId"`
	Enabled                  bool              `json:"enabled"`
	Protocol                 string            `json:"protocol"`
	PublicClient             bool              `json:"publicClient"`
	ServiceAccountsEnabled   bool              `json:"serviceAccountsEnabled"`
	DirectAccessGrantsEnabled bool             `json:"directAccessGrantsEnabled"`
	StandardFlowEnabled      bool              `json:"standardFlowEnabled"`
	Attributes               map[string]string `json:"attributes,omitempty"`
	ProtocolMappers          []ProtocolMapper  `json:"protocolMappers,omitempty"`
}

// ProtocolMapper maps claims in tokens.
type ProtocolMapper struct {
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	ProtocolMapper string            `json:"protocolMapper"`
	Config         map[string]string `json:"config"`
}

// ClientSecret holds the client secret response.
type ClientSecret struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// NewClient creates a Keycloak admin client.
func NewClient(baseURL, realm string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		realm:   realm,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Authenticate obtains an admin token using client_credentials grant.
func (c *Client) Authenticate(ctx context.Context, clientID, clientSecret string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", c.baseURL),
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("building auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed (HTTP %d): %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding token response: %w", err)
	}

	c.adminToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-30) * time.Second)
	return nil
}

// CreateClient registers a new Keycloak client for the given agent.
func (c *Client) CreateClient(ctx context.Context, agentName string) (*ClientRepresentation, error) {
	client := &ClientRepresentation{
		ClientID:                 agentName,
		Enabled:                  true,
		Protocol:                 "openid-connect",
		PublicClient:             false,
		ServiceAccountsEnabled:   true,
		DirectAccessGrantsEnabled: false,
		StandardFlowEnabled:      false,
		Attributes: map[string]string{
			"agent_id": agentName,
		},
		ProtocolMappers: []ProtocolMapper{
			{
				Name:           "agent-id-mapper",
				Protocol:       "openid-connect",
				ProtocolMapper: "oidc-usermodel-attribute-mapper",
				Config: map[string]string{
					"user.attribute":      "agent_id",
					"claim.name":          "agent_id",
					"id.token.claim":      "true",
					"access.token.claim":  "true",
					"jsonType.label":      "String",
				},
			},
		},
	}

	body, err := json.Marshal(client)
	if err != nil {
		return nil, fmt.Errorf("marshalling client: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/admin/realms/%s/clients", c.baseURL, c.realm),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("building create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.getToken())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		// Client already exists; fetch and return it
		return c.GetClient(ctx, agentName)
	}

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create failed (HTTP %d): %s", resp.StatusCode, respBody)
	}

	return c.GetClient(ctx, agentName)
}

// GetClient retrieves a client by clientId.
func (c *Client) GetClient(ctx context.Context, clientID string) (*ClientRepresentation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/admin/realms/%s/clients?clientId=%s", c.baseURL, c.realm, url.QueryEscape(clientID)),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.getToken())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var clients []ClientRepresentation
	if err := json.NewDecoder(resp.Body).Decode(&clients); err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return nil, fmt.Errorf("client %s not found", clientID)
	}
	return &clients[0], nil
}

// GetClientSecret retrieves the client secret for the given internal client ID.
func (c *Client) GetClientSecret(ctx context.Context, internalID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/admin/realms/%s/clients/%s/client-secret", c.baseURL, c.realm, internalID),
		nil,
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.getToken())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var secret ClientSecret
	if err := json.NewDecoder(resp.Body).Decode(&secret); err != nil {
		return "", err
	}
	return secret.Value, nil
}

// DeleteClient removes a Keycloak client by its internal ID.
func (c *Client) DeleteClient(ctx context.Context, internalID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/admin/realms/%s/clients/%s", c.baseURL, c.realm, internalID),
		nil,
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.getToken())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete failed (HTTP %d)", resp.StatusCode)
	}
	return nil
}

func (c *Client) getToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.adminToken
}
```

### Reconciler Flow

`internal/controller/mcpagent_controller.go` (key logic):

```go
func (r *MCPAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	agent := &mcpv1alpha1.MCPAgent{}
	if err := r.Get(ctx, req.NamespacedName, agent); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !agent.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, agent)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(agent, finalizerName) {
		controllerutil.AddFinalizer(agent, finalizerName)
		return ctrl.Result{}, r.Update(ctx, agent)
	}

	switch agent.Status.Phase {
	case "", mcpv1alpha1.AgentPhasePending:
		return r.handlePending(ctx, agent)
	case mcpv1alpha1.AgentPhaseRegistering:
		return r.handleRegistering(ctx, agent)
	case mcpv1alpha1.AgentPhaseActive:
		return r.handleActive(ctx, agent)
	case mcpv1alpha1.AgentPhaseError:
		// Retry after backoff
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *MCPAgentReconciler) handlePending(ctx context.Context, agent *mcpv1alpha1.MCPAgent) (ctrl.Result, error) {
	agent.Status.Phase = mcpv1alpha1.AgentPhaseRegistering
	now := metav1.Now()
	agent.Status.LastTransitionTime = &now
	return ctrl.Result{Requeue: true}, r.Status().Update(ctx, agent)
}

func (r *MCPAgentReconciler) handleRegistering(ctx context.Context, agent *mcpv1alpha1.MCPAgent) (ctrl.Result, error) {
	// 1. Create Keycloak client
	kcClient, err := r.KeycloakClient.CreateClient(ctx, agent.Name)
	if err != nil {
		return r.setError(ctx, agent, fmt.Errorf("keycloak registration: %w", err))
	}

	// 2. Retrieve client secret
	secret, err := r.KeycloakClient.GetClientSecret(ctx, kcClient.ID)
	if err != nil {
		return r.setError(ctx, agent, fmt.Errorf("fetching secret: %w", err))
	}

	// 3. Store credentials in K8s Secret
	secretName := fmt.Sprintf("mcp-agent-%s-credentials", agent.Name)
	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: agent.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(agent, mcpv1alpha1.GroupVersion.WithKind("MCPAgent")),
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"client_id":     agent.Name,
			"client_secret": secret,
		},
	}
	if err := r.createOrUpdate(ctx, k8sSecret); err != nil {
		return r.setError(ctx, agent, fmt.Errorf("storing secret: %w", err))
	}

	// 4. Transition to Active
	agent.Status.Phase = mcpv1alpha1.AgentPhaseActive
	agent.Status.KeycloakClientID = kcClient.ID
	agent.Status.CredentialSecretRef = secretName
	now := metav1.Now()
	agent.Status.LastTransitionTime = &now

	return ctrl.Result{}, r.Status().Update(ctx, agent)
}
```

---

## 3.4 Cerbos Deployment & Integration

### Standalone Deployment

Cerbos runs as a Deployment in `mcp-system`, configured via a ConfigMap:

```yaml
# templates/cerbos-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cerbos
  namespace: mcp-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: cerbos
  template:
    metadata:
      labels:
        app: cerbos
    spec:
      containers:
        - name: cerbos
          image: ghcr.io/cerbos/cerbos:0.38.0
          args: ["server", "--config=/config/config.yaml"]
          ports:
            - containerPort: 3593  # gRPC
              name: grpc
            - containerPort: 3592  # HTTP
              name: http
          volumeMounts:
            - name: config
              mountPath: /config
      volumes:
        - name: config
          configMap:
            name: cerbos-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cerbos-config
  namespace: mcp-system
data:
  config.yaml: |
    server:
      httpListenAddr: ":3592"
      grpcListenAddr: ":3593"
      adminAPI:
        enabled: true
        adminCredentials:
          username: "${CERBOS_ADMIN_USER}"
          passwordHash: "${CERBOS_ADMIN_PASS_HASH}"
    storage:
      driver: sqlite3
      sqlite3:
        dsn: ":memory:"
    audit:
      enabled: true
      backend: local
      local:
        storagePath: /tmp/cerbos-audit
```

### MCPPolicyReconciler

Watches `MCPPolicy` CRs and syncs them to Cerbos via the Admin API:

```go
func (r *MCPPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	policy := &mcpv1alpha1.MCPPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Translate to Cerbos policy
	cerbosPolicy, err := cerbos.TranslatePolicy(policy)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("translating policy: %w", err)
	}

	derivedRoles := cerbos.TranslateDerivedRoles(policy)

	// Push to Cerbos Admin API
	if err := r.CerbosAdmin.AddOrUpdatePolicy(ctx, derivedRoles); err != nil {
		return ctrl.Result{}, fmt.Errorf("syncing derived roles: %w", err)
	}
	if err := r.CerbosAdmin.AddOrUpdatePolicy(ctx, cerbosPolicy); err != nil {
		return ctrl.Result{}, fmt.Errorf("syncing resource policy: %w", err)
	}

	// Update status
	now := metav1.Now()
	policy.Status.SyncedAt = &now
	policy.Status.CerbosPolicyID = fmt.Sprintf("mcp_%s", policy.Name)
	return ctrl.Result{}, r.Status().Update(ctx, policy)
}
```

### ext_authz Adapter

An adapter bridges Envoy's ext_authz protocol to Cerbos check requests.
Envoy calls the adapter for every MCP request after JWT validation has
injected the `x-mcp-agent-id` and `x-mcp-roles` headers.

`internal/authz/extauthz.go` (gRPC service):

```go
package authz

import (
	"context"
	"strings"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	cerbosSDK "github.com/cerbos/cerbos-sdk-go/cerbos"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

type ExtAuthzServer struct {
	cerbos cerbosSDK.Client
	authv3.UnimplementedAuthorizationServer
}

func NewExtAuthzServer(cerbosClient cerbosSDK.Client) *ExtAuthzServer {
	return &ExtAuthzServer{cerbos: cerbosClient}
}

func (s *ExtAuthzServer) Check(
	ctx context.Context,
	req *authv3.CheckRequest,
) (*authv3.CheckResponse, error) {
	headers := req.Attributes.Request.Http.Headers

	agentID := headers["x-mcp-agent-id"]
	mcpMethod := headers["mcp-method"]
	mcpName := headers["mcp-name"]
	path := req.Attributes.Request.Http.Path

	if agentID == "" {
		return deny(codes.Unauthenticated, "missing agent identity"), nil
	}

	// Extract server name from path: /<server>/mcp -> server
	serverName := extractServerName(path)

	// Build Cerbos check request
	principal := cerbosSDK.NewPrincipal(agentID, "mcp-agent").
		WithAttr("agent_id", agentID).
		WithAttr("source_ip", headers["x-forwarded-for"])

	resource := cerbosSDK.NewResource("mcp:tool", mcpName).
		WithAttr("server", serverName)

	action := strings.ReplaceAll(mcpMethod, "/", ":")
	if action == "" {
		action = "tools:call" // default
	}

	result, err := s.cerbos.IsAllowed(ctx, principal, resource, action)
	if err != nil {
		return deny(codes.Internal, "authz check failed"), nil
	}

	if !result {
		return deny(codes.PermissionDenied, "access denied by policy"), nil
	}

	return &authv3.CheckResponse{
		Status: &status.Status{Code: int32(codes.OK)},
		HttpResponse: &authv3.CheckResponse_OkResponse{
			OkResponse: &authv3.OkHttpResponse{},
		},
	}, nil
}

func deny(code codes.Code, msg string) *authv3.CheckResponse {
	httpCode := typev3.StatusCode_Forbidden
	if code == codes.Unauthenticated {
		httpCode = typev3.StatusCode_Unauthorized
	}
	return &authv3.CheckResponse{
		Status: &status.Status{Code: int32(code), Message: msg},
		HttpResponse: &authv3.CheckResponse_DeniedResponse{
			DeniedResponse: &authv3.DeniedHttpResponse{
				Status: &typev3.HttpStatus{Code: httpCode},
			},
		},
	}
}

func extractServerName(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return "unknown"
}
```

---

## 3.5 Per-Agent Rate Limiting

### Descriptors Keyed by agent_id + server + tool

The rate limit ConfigMap generated by the operator uses per-agent overrides
from `MCPAgent.Spec.RateLimits`. A ConfigMap generator runs as part of the
`MCPAgentReconciler`:

```go
func (r *MCPAgentReconciler) generateRateLimitConfig(ctx context.Context) error {
	agents := &mcpv1alpha1.MCPAgentList{}
	if err := r.List(ctx, agents); err != nil {
		return err
	}

	descriptors := []map[string]interface{}{}

	for _, agent := range agents.Items {
		if agent.Spec.RateLimits == nil {
			continue
		}
		rl := agent.Spec.RateLimits

		// Global per-agent override
		if rl.Global > 0 {
			descriptors = append(descriptors, map[string]interface{}{
				"key":   "agent_id",
				"value": agent.Name,
				"rate_limit": map[string]interface{}{
					"unit":              "minute",
					"requests_per_unit": rl.Global,
				},
			})
		}

		// Per-server per-agent
		if rl.PerServer > 0 {
			for _, sa := range agent.Spec.ServerAccess {
				descriptors = append(descriptors, map[string]interface{}{
					"key":   "agent_id",
					"value": agent.Name,
					"descriptors": []map[string]interface{}{
						{
							"key":   "mcp_server",
							"value": sa.ServerRef,
							"rate_limit": map[string]interface{}{
								"unit":              "minute",
								"requests_per_unit": rl.PerServer,
							},
						},
					},
				})
			}
		}

		// Per-tool per-agent
		if rl.PerTool > 0 {
			for _, sa := range agent.Spec.ServerAccess {
				for _, tool := range sa.AllowedTools {
					descriptors = append(descriptors, map[string]interface{}{
						"key":   "agent_id",
						"value": agent.Name,
						"descriptors": []map[string]interface{}{
							{
								"key":   "mcp_server",
								"value": sa.ServerRef,
								"descriptors": []map[string]interface{}{
									{
										"key":   "mcp_tool",
										"value": tool,
										"rate_limit": map[string]interface{}{
											"unit":              "minute",
											"requests_per_unit": rl.PerTool,
										},
									},
								},
							},
						},
					})
				}
			}
		}
	}

	// Marshal and update ConfigMap
	// ...
	return nil
}
```

### Monthly Quota via Prometheus Counter

A Prometheus counter `mcp_agent_requests_total` with labels
`{agent_id, server, tool}` is incremented by the ext_authz adapter on every
allowed request. The reconciler queries Prometheus to populate
`MCPAgent.Status.QuotaUsed` and rejects requests when the quota is exceeded:

```go
// In the ext_authz Check method, after a successful Cerbos check:
requestsTotal.WithLabelValues(agentID, serverName, mcpName).Inc()

// In the reconciler, periodically:
func (r *MCPAgentReconciler) syncQuotaUsage(ctx context.Context, agent *mcpv1alpha1.MCPAgent) error {
	if agent.Spec.RateLimits == nil || agent.Spec.RateLimits.Quota == nil {
		return nil
	}

	query := fmt.Sprintf(
		`sum(increase(mcp_agent_requests_total{agent_id="%s"}[30d]))`,
		agent.Name,
	)
	result, _, err := r.PrometheusAPI.Query(ctx, query, time.Now())
	if err != nil {
		return err
	}

	usage := int64(extractScalarValue(result))
	agent.Status.QuotaUsed = usage

	quota := agent.Spec.RateLimits.Quota
	threshold := float64(quota.MonthlyRequests) * float64(quota.AlertThresholdPercent) / 100.0

	if float64(usage) >= threshold {
		meta.SetStatusCondition(&agent.Status.Conditions, metav1.Condition{
			Type:    "QuotaWarning",
			Status:  metav1.ConditionTrue,
			Reason:  "ApproachingLimit",
			Message: fmt.Sprintf("Used %d of %d monthly requests (%.0f%%)",
				usage, quota.MonthlyRequests, float64(usage)/float64(quota.MonthlyRequests)*100),
		})
	}

	return r.Status().Update(ctx, agent)
}
```

---

## 3.6 Audit Logging

### Logger Implementation

`internal/audit/logger.go`:

```go
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

// Entry represents a single auditable event in the MCP Gateway.
type Entry struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// RequestID is a unique identifier for the request.
	RequestID string `json:"requestID"`

	// AgentID identifies the agent that made the request.
	AgentID string `json:"agentID"`

	// ServerName is the MCP server targeted.
	ServerName string `json:"serverName"`

	// Method is the MCP method invoked (e.g. "tools/call").
	Method string `json:"method"`

	// ToolName is the specific tool called (empty for non-tool methods).
	ToolName string `json:"toolName,omitempty"`

	// Decision is the authorization outcome: "ALLOW" or "DENY".
	Decision string `json:"decision"`

	// DenyReason provides context when Decision is "DENY".
	DenyReason string `json:"denyReason,omitempty"`

	// PolicyName is the MCPPolicy that produced the decision.
	PolicyName string `json:"policyName,omitempty"`

	// SourceIP is the client IP address.
	SourceIP string `json:"sourceIP,omitempty"`

	// RequestHash is the SHA-256 hash of the request body, providing a
	// tamper-evident link to the original request without storing
	// potentially sensitive payload data.
	RequestHash string `json:"requestHash"`

	// ResponseCode is the HTTP status returned to the client.
	ResponseCode int `json:"responseCode"`

	// Duration is how long the request took end-to-end.
	Duration time.Duration `json:"duration"`

	// Labels carries additional key-value metadata.
	Labels map[string]string `json:"labels,omitempty"`
}

// Sink defines where audit entries are written.
type Sink interface {
	// Write sends an audit entry to the sink.
	Write(ctx context.Context, entry Entry) error

	// Close flushes and shuts down the sink.
	Close() error
}

// Logger fans out audit entries to one or more sinks.
type Logger struct {
	sinks []Sink
	log   logr.Logger
}

// NewLogger creates a Logger that writes to all provided sinks.
func NewLogger(log logr.Logger, sinks ...Sink) *Logger {
	return &Logger{
		sinks: sinks,
		log:   log,
	}
}

// Record writes an audit entry to all configured sinks.
// Errors are logged but do not block the request pipeline.
func (l *Logger) Record(ctx context.Context, entry Entry) {
	for _, sink := range l.sinks {
		if err := sink.Write(ctx, entry); err != nil {
			l.log.Error(err, "audit sink write failed",
				"sink", fmt.Sprintf("%T", sink),
				"requestID", entry.RequestID,
			)
		}
	}
}

// HashRequest computes the SHA-256 hash of a request body.
func HashRequest(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

// --- WebhookSink ---

// WebhookSink sends audit entries to an HTTP endpoint as JSON.
type WebhookSink struct {
	url        string
	httpClient *http.Client
	headers    map[string]string

	// Buffering for batch delivery
	mu      sync.Mutex
	buffer  []Entry
	maxBuf  int
	flushCh chan struct{}
	done    chan struct{}
}

// WebhookConfig configures the WebhookSink.
type WebhookConfig struct {
	// URL is the webhook endpoint.
	URL string

	// Headers are added to every request (e.g. auth tokens).
	Headers map[string]string

	// BufferSize is the max entries to batch before flushing. Default 100.
	BufferSize int

	// FlushInterval controls how often the buffer is flushed. Default 5s.
	FlushInterval time.Duration

	// Timeout is the HTTP request timeout. Default 10s.
	Timeout time.Duration
}

// NewWebhookSink creates a webhook-based audit sink.
func NewWebhookSink(cfg WebhookConfig) *WebhookSink {
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 100
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	ws := &WebhookSink{
		url: cfg.URL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		headers: cfg.Headers,
		buffer:  make([]Entry, 0, cfg.BufferSize),
		maxBuf:  cfg.BufferSize,
		flushCh: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}

	// Background flusher
	go ws.flusher(cfg.FlushInterval)

	return ws
}

// Write adds an entry to the buffer. Triggers a flush if the buffer is full.
func (ws *WebhookSink) Write(ctx context.Context, entry Entry) error {
	ws.mu.Lock()
	ws.buffer = append(ws.buffer, entry)
	shouldFlush := len(ws.buffer) >= ws.maxBuf
	ws.mu.Unlock()

	if shouldFlush {
		select {
		case ws.flushCh <- struct{}{}:
		default:
		}
	}
	return nil
}

// Close flushes remaining entries and stops the background goroutine.
func (ws *WebhookSink) Close() error {
	close(ws.done)
	return ws.flush()
}

func (ws *WebhookSink) flusher(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = ws.flush()
		case <-ws.flushCh:
			_ = ws.flush()
		case <-ws.done:
			return
		}
	}
}

func (ws *WebhookSink) flush() error {
	ws.mu.Lock()
	if len(ws.buffer) == 0 {
		ws.mu.Unlock()
		return nil
	}
	batch := ws.buffer
	ws.buffer = make([]Entry, 0, ws.maxBuf)
	ws.mu.Unlock()

	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshalling audit batch: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, ws.url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range ws.headers {
		req.Header.Set(k, v)
	}

	resp, err := ws.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}
```

---

## 3.7 Comprehensive Tests

### Unit Tests: Policy Translation

`internal/cerbos/translate_test.go`:

```go
package cerbos_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpv1alpha1 "github.com/mcp-gateway/api/v1alpha1"
	"github.com/mcp-gateway/internal/cerbos"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTranslatePolicy_BasicAllow(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-reads",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Name:       "allow-list-tools",
					Effect:     mcpv1alpha1.EffectAllow,
					Principals: []string{"agent-a"},
					Actions:    []string{"tools/list"},
					Resources:  []string{"*"},
				},
			},
		},
	}

	result, err := cerbos.TranslatePolicy(policy)
	require.NoError(t, err)
	require.NotNil(t, result)

	rp := result.GetResourcePolicy()
	require.NotNil(t, rp)
	assert.Equal(t, "mcp:tool", rp.Resource)
	assert.Len(t, rp.Rules, 1)
	assert.Equal(t, []string{"tools:list"}, rp.Rules[0].Actions)
}

func TestTranslatePolicy_DenyRule(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deny-dangerous",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Name:       "deny-delete",
					Effect:     mcpv1alpha1.EffectDeny,
					Principals: []string{"*"},
					Actions:    []string{"tools/call"},
					Resources:  []string{"db-delete", "db-drop"},
				},
			},
		},
	}

	result, err := cerbos.TranslatePolicy(policy)
	require.NoError(t, err)
	require.NotNil(t, result)

	rp := result.GetResourcePolicy()
	require.Len(t, rp.Rules, 1)
	assert.Equal(t, []string{"tools:call"}, rp.Rules[0].Actions)
}

func TestTranslatePolicy_EmptyRules_Error(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "default"},
		Spec:       mcpv1alpha1.MCPPolicySpec{Rules: []mcpv1alpha1.PolicyRule{}},
	}

	_, err := cerbos.TranslatePolicy(policy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rules")
}

func TestTranslateDerivedRoles_GlobPattern(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "team-a-policy"},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Name:       "team-a-access",
					Effect:     mcpv1alpha1.EffectAllow,
					Principals: []string{"team-a-*"},
					Actions:    []string{"*"},
					Resources:  []string{"*"},
				},
			},
		},
	}

	result := cerbos.TranslateDerivedRoles(policy)
	require.NotNil(t, result)

	dr := result.GetDerivedRoles()
	require.NotNil(t, dr)
	require.Len(t, dr.Definitions, 1)
	assert.Contains(t, dr.Definitions[0].Condition.GetMatch().GetExpr(), "matches")
}
```

### envtest Tests: MCPAgent and MCPPolicy CRUD

`internal/controller/mcpagent_controller_test.go`:

```go
package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpv1alpha1 "github.com/mcp-gateway/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMCPAgent_CreateAndReconcile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agent := &mcpv1alpha1.MCPAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPAgentSpec{
			Identity: mcpv1alpha1.AgentIdentity{
				DisplayName: "Test Agent",
				Description: "Integration test agent",
				Owner:       "platform-team",
			},
			ServerAccess: []mcpv1alpha1.ServerAccessEntry{
				{
					ServerRef:    "test-server",
					PolicyRef:    "test-policy",
					AllowedTools: []string{"search", "query"},
				},
			},
			RateLimits: &mcpv1alpha1.AgentRateLimits{
				PerTool:   10,
				PerServer: 30,
				Global:    60,
				Quota: &mcpv1alpha1.AgentQuota{
					MonthlyRequests:       10000,
					AlertThresholdPercent: 80,
				},
			},
		},
	}

	// Create the agent
	err := k8sClient.Create(ctx, agent)
	require.NoError(t, err)

	// Wait for reconciliation to reach Active phase
	eventually(t, ctx, func() bool {
		fetched := &mcpv1alpha1.MCPAgent{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name: "test-agent", Namespace: "default",
		}, fetched)
		return err == nil && fetched.Status.Phase == mcpv1alpha1.AgentPhaseActive
	}, 15*time.Second, "agent did not reach Active phase")

	// Verify credential secret was created
	secret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name: "mcp-agent-test-agent-credentials", Namespace: "default",
	}, secret)
	require.NoError(t, err)
	assert.NotEmpty(t, secret.Data["client_id"])
	assert.NotEmpty(t, secret.Data["client_secret"])

	// Cleanup
	err = k8sClient.Delete(ctx, agent)
	require.NoError(t, err)
}

func TestMCPPolicy_CreateAndSync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Description: "Test policy for integration tests",
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Name:       "allow-search",
					Effect:     mcpv1alpha1.EffectAllow,
					Principals: []string{"test-agent"},
					Actions:    []string{"tools/call"},
					Resources:  []string{"search"},
				},
				{
					Name:       "deny-delete",
					Effect:     mcpv1alpha1.EffectDeny,
					Principals: []string{"*"},
					Actions:    []string{"tools/call"},
					Resources:  []string{"delete-*"},
				},
			},
		},
	}

	err := k8sClient.Create(ctx, policy)
	require.NoError(t, err)

	// Wait for sync to Cerbos
	eventually(t, ctx, func() bool {
		fetched := &mcpv1alpha1.MCPPolicy{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name: "test-policy", Namespace: "default",
		}, fetched)
		return err == nil && fetched.Status.SyncedAt != nil
	}, 15*time.Second, "policy did not sync to Cerbos")

	// Update the policy
	fetched := &mcpv1alpha1.MCPPolicy{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name: "test-policy", Namespace: "default",
	}, fetched)
	require.NoError(t, err)

	fetched.Spec.Rules = append(fetched.Spec.Rules, mcpv1alpha1.PolicyRule{
		Name:       "allow-query",
		Effect:     mcpv1alpha1.EffectAllow,
		Principals: []string{"test-agent"},
		Actions:    []string{"tools/call"},
		Resources:  []string{"query"},
	})
	err = k8sClient.Update(ctx, fetched)
	require.NoError(t, err)

	// Cleanup
	err = k8sClient.Delete(ctx, policy)
	require.NoError(t, err)
}

// eventually polls a condition function until it returns true or the timeout elapses.
func eventually(t *testing.T, ctx context.Context, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("context cancelled while waiting:", msg)
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Fatal("timed out:", msg)
}
```

### E2E Tests: Full Authorization Flow

`test/e2e/authz_test.go`:

```go
package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedToolCall_Returns200(t *testing.T) {
	secret := getClientSecret(t)
	token := getToken(t, "test-agent-001", secret)

	body := `{
		"jsonrpc": "2.0",
		"method": "tools/call",
		"params": {"name": "search", "arguments": {"q": "test"}},
		"id": 1
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		gatewayURL+"/test-server/mcp",
		strings.NewReader(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", "search")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"allowed tool call must return 200")
}

func TestDeniedToolCall_Returns403(t *testing.T) {
	secret := getClientSecret(t)
	token := getToken(t, "test-agent-001", secret)

	body := `{
		"jsonrpc": "2.0",
		"method": "tools/call",
		"params": {"name": "delete-all", "arguments": {}},
		"id": 2
	}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		gatewayURL+"/test-server/mcp",
		strings.NewReader(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", "delete-all")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"denied tool call must return 403")
}

func TestPerAgentRateLimit_Returns429(t *testing.T) {
	secret := getClientSecret(t)
	token := getToken(t, "test-agent-001", secret)

	var lastStatus int
	// The test-agent-001 has perTool=10 for the "search" tool
	for i := 0; i < 15; i++ {
		body := `{
			"jsonrpc": "2.0",
			"method": "tools/call",
			"params": {"name": "search", "arguments": {"q": "test"}},
			"id": 1
		}`
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			gatewayURL+"/test-server/mcp",
			strings.NewReader(body),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Mcp-Method", "tools/call")
		req.Header.Set("Mcp-Name", "search")

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		lastStatus = resp.StatusCode

		if resp.StatusCode == http.StatusTooManyRequests {
			break
		}
	}

	assert.Equal(t, http.StatusTooManyRequests, lastStatus,
		"per-agent rate limited requests must return 429")
}

func TestAuditLogEntry_Recorded(t *testing.T) {
	secret := getClientSecret(t)
	token := getToken(t, "test-agent-001", secret)

	// Make a request that will be audited
	body := `{
		"jsonrpc": "2.0",
		"method": "tools/call",
		"params": {"name": "search", "arguments": {"q": "audit-test"}},
		"id": 99
	}`
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		gatewayURL+"/test-server/mcp",
		strings.NewReader(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", "search")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	// Wait a moment for async audit delivery
	time.Sleep(2 * time.Second)

	// Query the audit log API
	auditReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		gatewayURL+"/admin/audit-log?agent_id=test-agent-001&limit=5",
		nil,
	)
	require.NoError(t, err)
	auditReq.Header.Set("Authorization", "Bearer "+token)

	auditResp, err := httpClient.Do(auditReq)
	require.NoError(t, err)
	defer auditResp.Body.Close()

	auditBody, err := io.ReadAll(auditResp.Body)
	require.NoError(t, err)

	var entries []struct {
		AgentID     string `json:"agentID"`
		Method      string `json:"method"`
		ToolName    string `json:"toolName"`
		Decision    string `json:"decision"`
		RequestHash string `json:"requestHash"`
	}
	require.NoError(t, json.Unmarshal(auditBody, &entries))
	require.NotEmpty(t, entries, "audit log must contain at least one entry")

	latest := entries[0]
	assert.Equal(t, "test-agent-001", latest.AgentID)
	assert.Equal(t, "tools/call", latest.Method)
	assert.Equal(t, "search", latest.ToolName)
	assert.Equal(t, "ALLOW", latest.Decision)
	assert.Len(t, latest.RequestHash, 64, "SHA-256 hash must be 64 hex chars")
}
```

---

## Deliverables Checklist

| Item | Path | Status |
|------|------|--------|
| MCPAgent CRD types | `api/v1alpha1/mcpagent_types.go` | |
| MCPAgent validation webhook | `internal/webhook/mcpagent_webhook.go` | |
| MCPPolicy CRD types | `api/v1alpha1/mcppolicy_types.go` | |
| Cerbos policy translation | `internal/cerbos/translate.go` | |
| Cerbos translation tests | `internal/cerbos/translate_test.go` | |
| MCPAgentReconciler | `internal/controller/mcpagent_controller.go` | |
| Keycloak Go client | `internal/keycloak/client.go` | |
| Cerbos deployment manifests | `deploy/charts/mcp-gateway/templates/cerbos-*` | |
| MCPPolicyReconciler | `internal/controller/mcppolicy_controller.go` | |
| ext_authz adapter | `internal/authz/extauthz.go` | |
| Rate limit ConfigMap generator | `internal/controller/mcpagent_controller.go` | |
| Audit logger + WebhookSink | `internal/audit/logger.go` | |
| envtest controller tests | `internal/controller/mcpagent_controller_test.go` | |
| E2E authorization tests | `test/e2e/authz_test.go` | |
