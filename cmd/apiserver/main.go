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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

func init() {
	_ = mcpv1alpha1.AddToScheme(scheme)
}

func main() {
	var port int
	var kubeconfig string

	flag.IntVar(&port, "port", 8090, "Port for the API server to listen on")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (optional, for local dev)")
	flag.Parse()

	// Build Kubernetes client configuration.
	cfg, err := buildConfig(kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build Kubernetes config: %v", err)
	}

	// Create controller-runtime client.
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", handleHealthz())

	// MCPServer endpoints.
	mux.HandleFunc("GET /api/v1/servers", handleListServers(c))
	mux.HandleFunc("GET /api/v1/servers/{name}", handleGetServer(c))
	mux.HandleFunc("POST /api/v1/servers", handleCreateServer(c))
	mux.HandleFunc("DELETE /api/v1/servers/{name}", handleDeleteServer(c))

	// MCPAgent endpoints.
	mux.HandleFunc("GET /api/v1/agents", handleListAgents(c))
	mux.HandleFunc("GET /api/v1/agents/{name}", handleGetAgent(c))

	// MCPPolicy endpoints.
	mux.HandleFunc("GET /api/v1/policies", handleListPolicies(c))

	// MCPMarketplaceEntry endpoints.
	mux.HandleFunc("GET /api/v1/marketplace", handleListMarketplace(c))

	// Wrap with CORS middleware.
	handler := corsMiddleware(mux)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("API server listening on :%d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down API server...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	log.Println("Server stopped")
}

// buildConfig creates a Kubernetes REST config, trying in-cluster first then
// falling back to the provided kubeconfig path or the default location.
func buildConfig(kubeconfig string) (*rest.Config, error) {
	// Try in-cluster config first.
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}

	// Fall back to kubeconfig.
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// corsMiddleware adds CORS headers for UI access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeJSON encodes v as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// writeError writes an error response as JSON.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- Health ---

func handleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// --- MCPServer handlers ---

func handleListServers(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var list mcpv1alpha1.MCPServerList
		if err := c.List(r.Context(), &list); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list.Items)
	}
}

func handleGetServer(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}

		var server mcpv1alpha1.MCPServer
		key := types.NamespacedName{Name: name, Namespace: namespace}
		if err := c.Get(r.Context(), key, &server); err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("MCPServer %q not found: %v", name, err))
			return
		}
		writeJSON(w, http.StatusOK, server)
	}
}

func handleCreateServer(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var server mcpv1alpha1.MCPServer
		if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
			return
		}

		if server.Namespace == "" {
			server.Namespace = "default"
		}

		if err := c.Create(r.Context(), &server); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create MCPServer: %v", err))
			return
		}
		writeJSON(w, http.StatusCreated, server)
	}
}

func handleDeleteServer(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}

		var server mcpv1alpha1.MCPServer
		key := types.NamespacedName{Name: name, Namespace: namespace}
		if err := c.Get(r.Context(), key, &server); err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("MCPServer %q not found: %v", name, err))
			return
		}

		if err := c.Delete(r.Context(), &server); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete MCPServer: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
	}
}

// --- MCPAgent handlers ---

func handleListAgents(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var list mcpv1alpha1.MCPAgentList
		if err := c.List(r.Context(), &list); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list.Items)
	}
}

func handleGetAgent(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		namespace := r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}

		var agent mcpv1alpha1.MCPAgent
		key := types.NamespacedName{Name: name, Namespace: namespace}
		if err := c.Get(r.Context(), key, &agent); err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("MCPAgent %q not found: %v", name, err))
			return
		}
		writeJSON(w, http.StatusOK, agent)
	}
}

// --- MCPPolicy handlers ---

func handleListPolicies(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var list mcpv1alpha1.MCPPolicyList
		if err := c.List(r.Context(), &list); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list.Items)
	}
}

// --- MCPMarketplaceEntry handlers ---

func handleListMarketplace(c client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var list mcpv1alpha1.MCPMarketplaceEntryList
		if err := c.List(r.Context(), &list); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list.Items)
	}
}
