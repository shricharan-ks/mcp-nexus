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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func int32Ptr(i int32) *int32 { return &i }

func newTestMCPServer(name, namespace, image string) *mcpv1alpha1.MCPServer {
	return &mcpv1alpha1.MCPServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid-1234",
		},
		Spec: mcpv1alpha1.MCPServerSpec{
			Source: mcpv1alpha1.MCPServerSource{
				Image: image,
				Port:  8080,
			},
			Protocol: mcpv1alpha1.MCPServerProtocol{
				Transport: mcpv1alpha1.TransportStreamableHTTP,
				Endpoint:  "/mcp",
			},
		},
	}
}

func newTestReconciler() *MCPServerReconciler {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = mcpv1alpha1.AddToScheme(scheme)

	return &MCPServerReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}
}

// --------------------------------------------------------------------------
// BuildDeployment tests
// --------------------------------------------------------------------------

func TestBuildDeployment_BasicDefaults(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("my-server", "default", "ghcr.io/example/my-mcp:v1")

	dep := r.buildDeployment(mcps)

	assert.Equal(t, "my-server", dep.Name)
	assert.Equal(t, "default", dep.Namespace)
	require.NotNil(t, dep.Spec.Replicas)
	assert.Equal(t, int32(1), *dep.Spec.Replicas)

	require.Len(t, dep.Spec.Template.Spec.Containers, 1)
	container := dep.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "ghcr.io/example/my-mcp:v1", container.Image)
	require.Len(t, container.Ports, 1)
	assert.Equal(t, int32(8080), container.Ports[0].ContainerPort)
}

func TestBuildDeployment_WithSecrets(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("secret-server", "ns1", "ghcr.io/example/mcp:v2")
	mcps.Spec.Secrets = []mcpv1alpha1.MCPServerSecret{
		{
			EnvVar: "API_KEY",
			SecretRef: mcpv1alpha1.SecretKeyRef{
				Name: "my-secret",
				Key:  "api-key",
			},
		},
		{
			EnvVar: "DB_PASSWORD",
			SecretRef: mcpv1alpha1.SecretKeyRef{
				Name: "db-creds",
				Key:  "password",
			},
		},
	}

	dep := r.buildDeployment(mcps)

	container := dep.Spec.Template.Spec.Containers[0]
	require.Len(t, container.Env, 2)

	assert.Equal(t, "API_KEY", container.Env[0].Name)
	require.NotNil(t, container.Env[0].ValueFrom)
	require.NotNil(t, container.Env[0].ValueFrom.SecretKeyRef)
	assert.Equal(t, "my-secret", container.Env[0].ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "api-key", container.Env[0].ValueFrom.SecretKeyRef.Key)

	assert.Equal(t, "DB_PASSWORD", container.Env[1].Name)
	require.NotNil(t, container.Env[1].ValueFrom)
	require.NotNil(t, container.Env[1].ValueFrom.SecretKeyRef)
	assert.Equal(t, "db-creds", container.Env[1].ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "password", container.Env[1].ValueFrom.SecretKeyRef.Key)
}

func TestBuildDeployment_WithResources(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("resource-server", "default", "ghcr.io/example/mcp:v1")
	mcps.Spec.Resources = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
	}

	dep := r.buildDeployment(mcps)

	container := dep.Spec.Template.Spec.Containers[0]
	assert.Equal(t, resource.MustParse("500m"), container.Resources.Limits[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("256Mi"), container.Resources.Limits[corev1.ResourceMemory])
	assert.Equal(t, resource.MustParse("100m"), container.Resources.Requests[corev1.ResourceCPU])
	assert.Equal(t, resource.MustParse("128Mi"), container.Resources.Requests[corev1.ResourceMemory])
}

func TestBuildDeployment_StdioTransport(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("stdio-server", "default", "ghcr.io/example/mcp-stdio:v1")
	mcps.Spec.Protocol.Transport = mcpv1alpha1.TransportStdio

	dep := r.buildDeployment(mcps)

	require.Len(t, dep.Spec.Template.Spec.Containers, 2, "stdio transport should have 2 containers (mcp-server + stdio-bridge)")

	var containerNames []string
	for _, c := range dep.Spec.Template.Spec.Containers {
		containerNames = append(containerNames, c.Name)
	}
	assert.Contains(t, containerNames, "mcp-server")
	assert.Contains(t, containerNames, "stdio-bridge")
}

func TestBuildDeployment_WithHealthCheck(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("health-server", "default", "ghcr.io/example/mcp:v1")
	mcps.Spec.Source.HealthCheck = &mcpv1alpha1.MCPServerHealthCheck{
		Path:          "/health",
		PeriodSeconds: 15,
	}

	dep := r.buildDeployment(mcps)

	container := dep.Spec.Template.Spec.Containers[0]
	require.NotNil(t, container.LivenessProbe, "expected liveness probe when healthCheck is set")
	require.NotNil(t, container.ReadinessProbe, "expected readiness probe when healthCheck is set")

	assert.Equal(t, "/health", container.LivenessProbe.HTTPGet.Path)
	assert.Equal(t, int32(15), container.LivenessProbe.PeriodSeconds)
	assert.Equal(t, "/health", container.ReadinessProbe.HTTPGet.Path)
	assert.Equal(t, int32(15), container.ReadinessProbe.PeriodSeconds)
}

func TestBuildDeployment_NoHealthCheck(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("no-health-server", "default", "ghcr.io/example/mcp:v1")

	dep := r.buildDeployment(mcps)

	container := dep.Spec.Template.Spec.Containers[0]
	assert.Nil(t, container.LivenessProbe, "expected no liveness probe when healthCheck is nil")
	assert.Nil(t, container.ReadinessProbe, "expected no readiness probe when healthCheck is nil")
}

func TestBuildDeployment_CustomReplicas(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("scaled-server", "production", "ghcr.io/example/mcp:v1")
	mcps.Spec.Scaling = &mcpv1alpha1.MCPServerScaling{
		MinReplicas: int32Ptr(3),
		MaxReplicas: 10,
	}

	dep := r.buildDeployment(mcps)

	require.NotNil(t, dep.Spec.Replicas)
	assert.Equal(t, int32(3), *dep.Spec.Replicas, "replicas should match scaling.minReplicas")
}

// --------------------------------------------------------------------------
// BuildService tests
// --------------------------------------------------------------------------

func TestBuildService_Basic(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("svc-server", "default", "ghcr.io/example/mcp:v1")

	svc := r.buildService(mcps)

	assert.Equal(t, "svc-server", svc.Name)
	assert.Equal(t, "default", svc.Namespace)

	require.Len(t, svc.Spec.Ports, 1)
	assert.Equal(t, int32(8080), svc.Spec.Ports[0].Port)

	// Selector labels must match the pod template labels.
	expectedLabels := commonLabels(mcps)
	for k, v := range svc.Spec.Selector {
		assert.Equal(t, expectedLabels[k], v, "selector label %s mismatch", k)
	}
}

func TestBuildService_CustomAnnotations(t *testing.T) {
	r := newTestReconciler()
	mcps := newTestMCPServer("annotated-svc", "default", "ghcr.io/example/mcp:v1")
	mcps.Spec.ServiceAnnotations = map[string]string{
		"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
		"prometheus.io/scrape":                              "true",
	}

	svc := r.buildService(mcps)

	require.NotNil(t, svc.Annotations)
	assert.Equal(t, "nlb", svc.Annotations["service.beta.kubernetes.io/aws-load-balancer-type"])
	assert.Equal(t, "true", svc.Annotations["prometheus.io/scrape"])
}

// --------------------------------------------------------------------------
// commonLabels tests
// --------------------------------------------------------------------------

func TestCommonLabels(t *testing.T) {
	mcps := newTestMCPServer("label-test", "default", "ghcr.io/example/mcp:v1")
	labels := commonLabels(mcps)

	expectedKeys := []string{
		"app.kubernetes.io/name",
		"app.kubernetes.io/instance",
		"app.kubernetes.io/managed-by",
	}
	for _, key := range expectedKeys {
		assert.Contains(t, labels, key, "expected label %s to be present", key)
	}

	assert.Equal(t, "label-test", labels["app.kubernetes.io/instance"])
	assert.Equal(t, "mcp-gateway", labels["app.kubernetes.io/managed-by"])
}
