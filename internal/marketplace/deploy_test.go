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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = mcpv1alpha1.AddToScheme(scheme)
	return scheme
}

func newCatalogEntryWithSecrets(name string, requiredSecrets []mcpv1alpha1.RequiredSecret, defaultPolicy *mcpv1alpha1.DefaultPolicy) mcpv1alpha1.MCPMarketplaceEntry {
	return mcpv1alpha1.MCPMarketplaceEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: mcpv1alpha1.MCPMarketplaceEntrySpec{
			DisplayName: "Test " + name,
			Vendor:      "test-vendor",
			Version:     "1.0.0",
			Category:    mcpv1alpha1.CategoryDeveloperTools,
			Source: mcpv1alpha1.MarketplaceSource{
				Image: "ghcr.io/test/" + name + ":v1",
			},
			InstallTemplate: mcpv1alpha1.InstallTemplate{
				MCPServerSpec: mcpv1alpha1.MCPServerSpec{
					Source: mcpv1alpha1.MCPServerSource{
						Image: "ghcr.io/test/" + name + ":v1",
						Port:  8080,
					},
					Protocol: mcpv1alpha1.MCPServerProtocol{
						Transport: mcpv1alpha1.TransportStreamableHTTP,
						Endpoint:  "/mcp",
					},
				},
				RequiredSecrets: requiredSecrets,
				DefaultPolicy:   defaultPolicy,
			},
		},
	}
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestDeployFromCatalog_Success(t *testing.T) {
	scheme := newTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	store := NewCatalogStore()

	entry := newCatalogEntryWithSecrets("github-mcp",
		[]mcpv1alpha1.RequiredSecret{
			{Name: "GITHUB_TOKEN", Description: "Personal access token"},
		},
		&mcpv1alpha1.DefaultPolicy{
			AllowedTools: []string{"list_repos", "get_file"},
			DeniedTools:  []string{"delete_repo"},
		},
	)
	store.Add(entry)

	svc := &DeployService{
		Client:       fakeClient,
		Scheme:       scheme,
		CatalogStore: store,
	}

	ctx := context.Background()
	serverName, err := svc.DeployFromCatalog(ctx, "github-mcp", "default", map[string]string{
		"GITHUB_TOKEN": "ghp_test1234567890",
	})

	require.NoError(t, err)
	assert.Equal(t, "github-mcp", serverName)

	// Verify Secret was created.
	var secret corev1.Secret
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "github-mcp-secrets", Namespace: "default"}, &secret)
	require.NoError(t, err)
	assert.Equal(t, "marketplace", secret.Labels["mcp-gateway.io/managed-by"])

	// Verify MCPServer was created.
	var mcpServer mcpv1alpha1.MCPServer
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "github-mcp", Namespace: "default"}, &mcpServer)
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/test/github-mcp:v1", mcpServer.Spec.Source.Image)
	assert.Equal(t, "marketplace", mcpServer.Labels["mcp-gateway.io/managed-by"])
	require.Len(t, mcpServer.Spec.Secrets, 1)
	assert.Equal(t, "GITHUB_TOKEN", mcpServer.Spec.Secrets[0].EnvVar)
	assert.Equal(t, "github-mcp-secrets", mcpServer.Spec.Secrets[0].SecretRef.Name)
	assert.Equal(t, "GITHUB_TOKEN", mcpServer.Spec.Secrets[0].SecretRef.Key)

	// Verify MCPPolicy was created.
	var policy mcpv1alpha1.MCPPolicy
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "github-mcp-policy", Namespace: "default"}, &policy)
	require.NoError(t, err)
	assert.Equal(t, "marketplace", policy.Labels["mcp-gateway.io/managed-by"])
	require.Len(t, policy.Spec.Rules, 2, "expected ALLOW and DENY rules")

	// Check ALLOW rule
	assert.Equal(t, mcpv1alpha1.PolicyEffectAllow, policy.Spec.Rules[0].Effect)
	assert.Equal(t, []string{"list_repos", "get_file"}, policy.Spec.Rules[0].Actions)

	// Check DENY rule
	assert.Equal(t, mcpv1alpha1.PolicyEffectDeny, policy.Spec.Rules[1].Effect)
	assert.Equal(t, []string{"delete_repo"}, policy.Spec.Rules[1].Actions)
}

func TestDeployFromCatalog_MissingSecret(t *testing.T) {
	scheme := newTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	store := NewCatalogStore()

	entry := newCatalogEntryWithSecrets("github-mcp",
		[]mcpv1alpha1.RequiredSecret{
			{Name: "GITHUB_TOKEN", Description: "Personal access token"},
			{Name: "WEBHOOK_SECRET", Description: "Webhook verification secret"},
		},
		nil,
	)
	store.Add(entry)

	svc := &DeployService{
		Client:       fakeClient,
		Scheme:       scheme,
		CatalogStore: store,
	}

	ctx := context.Background()

	// Only provide one of two required secrets.
	_, err := svc.DeployFromCatalog(ctx, "github-mcp", "default", map[string]string{
		"GITHUB_TOKEN": "ghp_test1234567890",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "required secret \"WEBHOOK_SECRET\" not provided")
}

func TestDeployFromCatalog_EntryNotFound(t *testing.T) {
	scheme := newTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	store := NewCatalogStore()

	svc := &DeployService{
		Client:       fakeClient,
		Scheme:       scheme,
		CatalogStore: store,
	}

	ctx := context.Background()
	_, err := svc.DeployFromCatalog(ctx, "nonexistent", "default", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "marketplace entry \"nonexistent\" not found")
}
