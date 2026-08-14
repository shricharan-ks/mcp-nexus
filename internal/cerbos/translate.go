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

// Package cerbos translates MCPPolicy custom resources into Cerbos resource
// policies that can be loaded by a Cerbos PDP.
package cerbos

import (
	"fmt"
	"strings"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Cerbos policy model
// --------------------------------------------------------------------------

// CerbosPolicy is the top-level Cerbos policy document.
type CerbosPolicy struct {
	APIVersion     string               `json:"apiVersion"`
	ResourcePolicy CerbosResourcePolicy `json:"resourcePolicy"`
}

// CerbosResourcePolicy defines access rules for a particular resource kind.
type CerbosResourcePolicy struct {
	Version  string       `json:"version"`
	Resource string       `json:"resource"`
	Rules    []CerbosRule `json:"rules"`
}

// CerbosRule is a single access rule inside a resource policy.
type CerbosRule struct {
	Actions   []string         `json:"actions"`
	Effect    string           `json:"effect"`
	Roles     []string         `json:"roles"`
	Condition *CerbosCondition `json:"condition,omitempty"`
}

// CerbosCondition wraps a match expression tree.
type CerbosCondition struct {
	Match CerbosMatch `json:"match"`
}

// CerbosMatch represents a Cerbos condition match node. Exactly one of its
// fields should be set.
type CerbosMatch struct {
	Any *CerbosMatchList `json:"any,omitempty"`
}

// CerbosMatchList holds a list of match expressions joined by OR semantics.
type CerbosMatchList struct {
	Of []CerbosExpr `json:"of"`
}

// CerbosExpr is a single CEL expression used inside a condition.
type CerbosExpr struct {
	Expr string `json:"expr"`
}

// --------------------------------------------------------------------------
// Translation logic
// --------------------------------------------------------------------------

// TranslatePolicy converts an MCPPolicy custom resource into a Cerbos
// resource policy. It returns an error if the policy contains no rules.
func TranslatePolicy(policy *mcpv1alpha1.MCPPolicy) (*CerbosPolicy, error) {
	if len(policy.Spec.Rules) == 0 {
		return nil, fmt.Errorf("MCPPolicy %s/%s has no rules", policy.Namespace, policy.Name)
	}

	cerbosRules := make([]CerbosRule, 0, len(policy.Spec.Rules))

	for i, rule := range policy.Spec.Rules {
		// Determine the effect.
		effect, err := mapEffect(rule.Effect)
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", i, err)
		}

		// Map actions: replace "/" with ":" (e.g. tools/call -> tools:call).
		actions := make([]string, len(rule.Actions))
		for j, a := range rule.Actions {
			actions[j] = strings.ReplaceAll(a, "/", ":")
		}

		// Map principals to Cerbos roles.
		roles := mapPrincipals(rule.Principals)

		cr := CerbosRule{
			Actions: actions,
			Effect:  effect,
			Roles:   roles,
		}

		// If specific tools are listed, add a condition to match them.
		if len(rule.Resources.Tools) > 0 {
			cr.Condition = buildToolCondition(rule.Resources.Tools)
		}

		cerbosRules = append(cerbosRules, cr)
	}

	// Determine the resource kind.
	resource := resourceKind(policy.Spec.Rules)

	return &CerbosPolicy{
		APIVersion: "api.cerbos.dev/v1",
		ResourcePolicy: CerbosResourcePolicy{
			Version:  "default",
			Resource: resource,
			Rules:    cerbosRules,
		},
	}, nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// mapEffect converts a Kubernetes PolicyEffect enum value to the
// corresponding Cerbos effect string.
func mapEffect(effect mcpv1alpha1.PolicyEffect) (string, error) {
	switch effect {
	case mcpv1alpha1.PolicyEffectAllow:
		return "EFFECT_ALLOW", nil
	case mcpv1alpha1.PolicyEffectDeny:
		return "EFFECT_DENY", nil
	default:
		return "", fmt.Errorf("unknown effect %q", effect)
	}
}

// mapPrincipals converts PolicyPrincipals to a slice of Cerbos role strings.
// If explicit roles are set they are used directly. Otherwise each agentRef
// is mapped to "agent:<name>".
func mapPrincipals(p mcpv1alpha1.PolicyPrincipals) []string {
	if len(p.Roles) > 0 {
		return p.Roles
	}
	roles := make([]string, 0, len(p.AgentRefs))
	for _, ref := range p.AgentRefs {
		roles = append(roles, "agent:"+ref.Name)
	}
	if len(roles) == 0 {
		return []string{"*"}
	}
	return roles
}

// resourceKind returns the Cerbos resource string derived from the first
// rule's serverRef. If no serverRef is set the generic "mcp:tool" is used.
func resourceKind(rules []mcpv1alpha1.PolicyRule) string {
	for _, r := range rules {
		if r.Resources.ServerRef != nil {
			return "mcp:server:" + r.Resources.ServerRef.Name
		}
	}
	return "mcp:tool"
}

// buildToolCondition creates a Cerbos condition that matches any of the
// listed tool names via the request.resource.attr.tool attribute.
func buildToolCondition(tools []string) *CerbosCondition {
	exprs := make([]CerbosExpr, len(tools))
	for i, tool := range tools {
		exprs[i] = CerbosExpr{
			Expr: fmt.Sprintf(`request.resource.attr.tool == %q`, tool),
		}
	}
	return &CerbosCondition{
		Match: CerbosMatch{
			Any: &CerbosMatchList{
				Of: exprs,
			},
		},
	}
}
