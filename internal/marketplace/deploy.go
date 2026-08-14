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

package marketplace

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// DeployService handles deploying marketplace entries into a Kubernetes cluster.
type DeployService struct {
	Client       client.Client
	Scheme       *runtime.Scheme
	CatalogStore *CatalogStore
}

// DeployFromCatalog creates Kubernetes resources from a catalog entry.
// It provisions a Secret (if required), an MCPServer CR, and optionally an MCPPolicy CR.
// Returns the created MCPServer name and any error.
func (d *DeployService) DeployFromCatalog(ctx context.Context, entryName, namespace string, secrets map[string]string) (string, error) {
	// 1. Get entry from CatalogStore.
	entry, ok := d.CatalogStore.Get(entryName)
	if !ok {
		return "", fmt.Errorf("marketplace entry %q not found", entryName)
	}

	// 2. Validate all RequiredSecrets are provided.
	for _, rs := range entry.Spec.InstallTemplate.RequiredSecrets {
		if _, exists := secrets[rs.Name]; !exists {
			return "", fmt.Errorf("required secret %q not provided", rs.Name)
		}
	}

	serverName := entry.Name

	// 3. Create K8s Secret with provided values.
	if len(secrets) > 0 {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serverName + "-secrets",
				Namespace: namespace,
				Labels: map[string]string{
					"mcp-gateway.io/managed-by":  "marketplace",
					"mcp-gateway.io/entry-name":  entryName,
					"app.kubernetes.io/component": "mcp-server",
				},
			},
			StringData: secrets,
		}

		if _, err := controllerutil.CreateOrUpdate(ctx, d.Client, secret, func() error {
			secret.StringData = secrets
			secret.Labels = map[string]string{
				"mcp-gateway.io/managed-by":  "marketplace",
				"mcp-gateway.io/entry-name":  entryName,
				"app.kubernetes.io/component": "mcp-server",
			}
			return nil
		}); err != nil {
			return "", fmt.Errorf("failed to create secret: %w", err)
		}
	}

	// 4. Create MCPServer CR from InstallTemplate.MCPServerSpec, wire secret refs.
	mcpServer := &mcpv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName,
			Namespace: namespace,
			Labels: map[string]string{
				"mcp-gateway.io/managed-by":  "marketplace",
				"mcp-gateway.io/entry-name":  entryName,
				"app.kubernetes.io/component": "mcp-server",
			},
		},
		Spec: *entry.Spec.InstallTemplate.MCPServerSpec.DeepCopy(),
	}

	// Wire secret references from the provided secrets.
	if len(secrets) > 0 {
		secretName := serverName + "-secrets"
		for key := range secrets {
			mcpServer.Spec.Secrets = append(mcpServer.Spec.Secrets, mcpv1alpha1.MCPServerSecret{
				EnvVar: key,
				SecretRef: mcpv1alpha1.SecretKeyRef{
					Name: secretName,
					Key:  key,
				},
			})
		}
	}

	if err := d.Client.Create(ctx, mcpServer); err != nil {
		return "", fmt.Errorf("failed to create MCPServer: %w", err)
	}

	// 5. If DefaultPolicy exists, create MCPPolicy CR.
	if dp := entry.Spec.InstallTemplate.DefaultPolicy; dp != nil {
		policy := &mcpv1alpha1.MCPPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serverName + "-policy",
				Namespace: namespace,
				Labels: map[string]string{
					"mcp-gateway.io/managed-by":  "marketplace",
					"mcp-gateway.io/entry-name":  entryName,
					"app.kubernetes.io/component": "mcp-policy",
				},
			},
			Spec: mcpv1alpha1.MCPPolicySpec{
				Rules: buildPolicyRules(serverName, dp),
			},
		}

		if err := d.Client.Create(ctx, policy); err != nil {
			return "", fmt.Errorf("failed to create MCPPolicy: %w", err)
		}
	}

	// 6. Return server name.
	return serverName, nil
}

// buildPolicyRules converts a DefaultPolicy into MCPPolicy rules.
func buildPolicyRules(serverName string, dp *mcpv1alpha1.DefaultPolicy) []mcpv1alpha1.PolicyRule {
	var rules []mcpv1alpha1.PolicyRule

	if len(dp.AllowedTools) > 0 {
		rules = append(rules, mcpv1alpha1.PolicyRule{
			Effect:  mcpv1alpha1.PolicyEffectAllow,
			Actions: dp.AllowedTools,
			Resources: mcpv1alpha1.PolicyResources{
				ServerRef: &mcpv1alpha1.LocalObjectReference{
					Name: serverName,
				},
			},
		})
	}

	if len(dp.DeniedTools) > 0 {
		rules = append(rules, mcpv1alpha1.PolicyRule{
			Effect:  mcpv1alpha1.PolicyEffectDeny,
			Actions: dp.DeniedTools,
			Resources: mcpv1alpha1.PolicyResources{
				ServerRef: &mcpv1alpha1.LocalObjectReference{
					Name: serverName,
				},
			},
		})
	}

	// If no specific allow/deny tools, create a default allow-all rule.
	if len(rules) == 0 {
		rules = append(rules, mcpv1alpha1.PolicyRule{
			Effect:  mcpv1alpha1.PolicyEffectAllow,
			Actions: []string{"*"},
			Resources: mcpv1alpha1.PolicyResources{
				ServerRef: &mcpv1alpha1.LocalObjectReference{
					Name: serverName,
				},
			},
		})
	}

	return rules
}
