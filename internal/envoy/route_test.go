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

package envoy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

func newTestServer(name, namespace string, port int32) *mcpv1alpha1.MCPServer {
	return &mcpv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: mcpv1alpha1.MCPServerSpec{
			Source: mcpv1alpha1.MCPServerSource{
				Image: "example.com/mcp-server:latest",
				Port:  port,
			},
			Protocol: mcpv1alpha1.MCPServerProtocol{
				Transport: mcpv1alpha1.TransportStreamableHTTP,
				Endpoint:  "/mcp",
			},
		},
	}
}

func TestBuildHTTPRoute_Basic(t *testing.T) {
	server := newTestServer("my-server", "test-ns", 8080)
	gatewayName := "mcp-gateway"
	gatewayNamespace := "gateway-ns"

	route := BuildHTTPRoute(server, gatewayName, gatewayNamespace)

	// Verify name.
	assert.Equal(t, "mcp-route-my-server", route.Name)
	assert.Equal(t, "test-ns", route.Namespace)

	// Verify parentRef.
	require.Len(t, route.Spec.ParentRefs, 1)
	assert.Equal(t, gatewayv1.ObjectName("mcp-gateway"), route.Spec.ParentRefs[0].Name)
	require.NotNil(t, route.Spec.ParentRefs[0].Namespace)
	assert.Equal(t, gatewayv1.Namespace("gateway-ns"), *route.Spec.ParentRefs[0].Namespace)

	// Verify rules.
	require.Len(t, route.Spec.Rules, 1)
	rule := route.Spec.Rules[0]

	// Path match.
	require.Len(t, rule.Matches, 1)
	match := rule.Matches[0]
	require.NotNil(t, match.Path)
	assert.Equal(t, gatewayv1.PathMatchPathPrefix, *match.Path.Type)
	assert.Equal(t, "/my-server/mcp", *match.Path.Value)

	// Method match.
	require.NotNil(t, match.Method)
	assert.Equal(t, gatewayv1.HTTPMethodPost, *match.Method)

	// BackendRef.
	require.Len(t, rule.BackendRefs, 1)
	backendRef := rule.BackendRefs[0]
	assert.Equal(t, gatewayv1.ObjectName("my-server"), backendRef.BackendRef.Name)
	require.NotNil(t, backendRef.BackendRef.Port)
	assert.Equal(t, gatewayv1.PortNumber(8080), *backendRef.BackendRef.Port)
}

func TestBuildHTTPRoute_Labels(t *testing.T) {
	server := newTestServer("label-test", "default", 9090)

	route := BuildHTTPRoute(server, "gw", "gw-ns")

	assert.Equal(t, "mcp-gateway", route.Labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "label-test", route.Labels["mcp-gateway.io/server"])
}

func TestHTTPRouteNameForServer(t *testing.T) {
	tests := []struct {
		serverName string
		expected   string
	}{
		{"my-server", "mcp-route-my-server"},
		{"github-tools", "mcp-route-github-tools"},
		{"a", "mcp-route-a"},
	}

	for _, tt := range tests {
		t.Run(tt.serverName, func(t *testing.T) {
			assert.Equal(t, tt.expected, HTTPRouteNameForServer(tt.serverName))
		})
	}
}
