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

package main

import (
	"context"
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
	controller "github.com/mcp-gateway/mcp-gateway/internal/controller"
	"github.com/mcp-gateway/mcp-gateway/internal/keycloak"
	"github.com/mcp-gateway/mcp-gateway/internal/observability"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mcpv1alpha1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func main() {
	var metricsAddr string
	var healthProbeAddr string
	var enableLeaderElection bool
	var gatewayName string
	var gatewayNamespace string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.StringVar(&gatewayName, "gateway-name", getEnvOrDefault("GATEWAY_NAME", "mcp-gateway"), "The name of the Gateway resource for HTTPRoute parentRef.")
	flag.StringVar(&gatewayNamespace, "gateway-namespace", getEnvOrDefault("GATEWAY_NAMESPACE", ""), "The namespace of the Gateway resource. Defaults to the operator's namespace.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	otelShutdown, err := observability.InitOTel(context.Background(), "mcp-gateway-operator",
		getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", ""))
	if err != nil {
		setupLog.Error(err, "failed to initialize OTel")
		os.Exit(1)
	}
	defer otelShutdown(context.Background())

	if err := observability.InitMetrics(); err != nil {
		setupLog.Error(err, "failed to initialize metrics")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: healthProbeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "mcp-gateway-operator-lock",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if err = (&controller.MCPServerReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		GatewayName:      gatewayName,
		GatewayNamespace: gatewayNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MCPServer")
		os.Exit(1)
	}

	// Set up Keycloak client if the environment is configured.
	var kcClient *keycloak.Client
	keycloakURL := os.Getenv("KEYCLOAK_URL")
	keycloakRealm := getEnvOrDefault("KEYCLOAK_REALM", "mcp-gateway")
	if keycloakURL != "" {
		kcClient = keycloak.NewClient(keycloakURL, keycloakRealm)
		setupLog.Info("Keycloak client configured", "url", keycloakURL, "realm", keycloakRealm)
	} else {
		setupLog.Info("KEYCLOAK_URL not set, MCPAgent Keycloak integration disabled")
	}

	if err = (&controller.MCPAgentReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		KeycloakClient: kcClient,
		KeycloakAdmin:  getEnvOrDefault("KEYCLOAK_ADMIN_USER", "admin"),
		KeycloakPass:   os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MCPAgent")
		os.Exit(1)
	}

	if err = (&controller.MCPPolicyReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		CerbosURL: os.Getenv("CERBOS_URL"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MCPPolicy")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
