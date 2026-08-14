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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// HTTPRouteNameForServer returns the HTTPRoute name for a given MCPServer.
func HTTPRouteNameForServer(serverName string) string {
	return "mcp-route-" + serverName
}

// BuildHTTPRoute creates a Gateway API HTTPRoute for an MCPServer.
// Route pattern: /<server-name>/mcp -> server's Service
// Matches POST method with optional Mcp-Method header matching.
func BuildHTTPRoute(server *mcpv1alpha1.MCPServer, gatewayName, gatewayNamespace string) *gatewayv1.HTTPRoute {
	pathPrefix := "/" + server.Name + "/mcp"
	pathType := gatewayv1.PathMatchPathPrefix
	methodPost := gatewayv1.HTTPMethodPost
	port := gatewayv1.PortNumber(server.Spec.Source.Port)
	ns := gatewayv1.Namespace(gatewayNamespace)

	return &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HTTPRouteNameForServer(server.Name),
			Namespace: server.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "mcp-gateway",
				"mcp-gateway.io/server":        server.Name,
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:      gatewayv1.ObjectName(gatewayName),
						Namespace: &ns,
					},
				},
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathType,
								Value: &pathPrefix,
							},
							Method: &methodPost,
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(server.Name),
									Port: &port,
								},
							},
						},
					},
				},
			},
		},
	}
}
