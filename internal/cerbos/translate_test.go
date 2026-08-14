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

package cerbos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

func TestTranslatePolicy_AllowWithTools(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-read",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Effect: mcpv1alpha1.PolicyEffectAllow,
					Principals: mcpv1alpha1.PolicyPrincipals{
						Roles: []string{"editor", "admin"},
					},
					Actions: []string{"tools/call", "tools/list"},
					Resources: mcpv1alpha1.PolicyResources{
						ServerRef: &mcpv1alpha1.LocalObjectReference{Name: "my-server"},
						Tools:     []string{"read_file", "write_file"},
					},
				},
			},
		},
	}

	result, err := TranslatePolicy(policy)
	require.NoError(t, err)

	assert.Equal(t, "api.cerbos.dev/v1", result.APIVersion)
	assert.Equal(t, "mcp:server:my-server", result.ResourcePolicy.Resource)
	assert.Equal(t, "default", result.ResourcePolicy.Version)

	require.Len(t, result.ResourcePolicy.Rules, 1)
	rule := result.ResourcePolicy.Rules[0]

	assert.Equal(t, "EFFECT_ALLOW", rule.Effect)
	assert.Equal(t, []string{"tools:call", "tools:list"}, rule.Actions)
	assert.Equal(t, []string{"editor", "admin"}, rule.Roles)

	require.NotNil(t, rule.Condition)
	require.NotNil(t, rule.Condition.Match.Any)
	require.Len(t, rule.Condition.Match.Any.Of, 2)
	assert.Equal(t, `request.resource.attr.tool == "read_file"`, rule.Condition.Match.Any.Of[0].Expr)
	assert.Equal(t, `request.resource.attr.tool == "write_file"`, rule.Condition.Match.Any.Of[1].Expr)
}

func TestTranslatePolicy_DenyWithTools(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deny-dangerous",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Effect: mcpv1alpha1.PolicyEffectDeny,
					Principals: mcpv1alpha1.PolicyPrincipals{
						Roles: []string{"viewer"},
					},
					Actions: []string{"tools/call"},
					Resources: mcpv1alpha1.PolicyResources{
						ServerRef: &mcpv1alpha1.LocalObjectReference{Name: "prod-server"},
						Tools:     []string{"delete_all"},
					},
				},
			},
		},
	}

	result, err := TranslatePolicy(policy)
	require.NoError(t, err)

	require.Len(t, result.ResourcePolicy.Rules, 1)
	rule := result.ResourcePolicy.Rules[0]

	assert.Equal(t, "EFFECT_DENY", rule.Effect)
	assert.Equal(t, "mcp:server:prod-server", result.ResourcePolicy.Resource)
}

func TestTranslatePolicy_WildcardActions(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Effect: mcpv1alpha1.PolicyEffectAllow,
					Principals: mcpv1alpha1.PolicyPrincipals{
						Roles: []string{"admin"},
					},
					Actions: []string{"*"},
					Resources: mcpv1alpha1.PolicyResources{
						ServerRef: &mcpv1alpha1.LocalObjectReference{Name: "my-server"},
					},
				},
			},
		},
	}

	result, err := TranslatePolicy(policy)
	require.NoError(t, err)

	require.Len(t, result.ResourcePolicy.Rules, 1)
	assert.Equal(t, []string{"*"}, result.ResourcePolicy.Rules[0].Actions)
	assert.Nil(t, result.ResourcePolicy.Rules[0].Condition)
}

func TestTranslatePolicy_AgentRefs(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-policy",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Effect: mcpv1alpha1.PolicyEffectAllow,
					Principals: mcpv1alpha1.PolicyPrincipals{
						AgentRefs: []mcpv1alpha1.LocalObjectReference{
							{Name: "code-agent"},
							{Name: "review-agent"},
						},
					},
					Actions: []string{"tools/call"},
					Resources: mcpv1alpha1.PolicyResources{
						ServerRef: &mcpv1alpha1.LocalObjectReference{Name: "my-server"},
					},
				},
			},
		},
	}

	result, err := TranslatePolicy(policy)
	require.NoError(t, err)

	require.Len(t, result.ResourcePolicy.Rules, 1)
	rule := result.ResourcePolicy.Rules[0]

	assert.Equal(t, []string{"agent:code-agent", "agent:review-agent"}, rule.Roles)
}

func TestTranslatePolicy_NoRules(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-policy",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{},
		},
	}

	result, err := TranslatePolicy(policy)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no rules")
}

func TestTranslatePolicy_NoServerRef(t *testing.T) {
	policy := &mcpv1alpha1.MCPPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "generic-policy",
			Namespace: "default",
		},
		Spec: mcpv1alpha1.MCPPolicySpec{
			Rules: []mcpv1alpha1.PolicyRule{
				{
					Effect: mcpv1alpha1.PolicyEffectAllow,
					Principals: mcpv1alpha1.PolicyPrincipals{
						Roles: []string{"admin"},
					},
					Actions: []string{"tools/call"},
				},
			},
		},
	}

	result, err := TranslatePolicy(policy)
	require.NoError(t, err)

	assert.Equal(t, "mcp:tool", result.ResourcePolicy.Resource)
}
