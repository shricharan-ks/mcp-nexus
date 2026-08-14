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
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
	"github.com/mcp-gateway/mcp-gateway/internal/keycloak"
)

const (
	agentFinalizerName = "mcp-gateway.io/agent-cleanup"
)

// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcpagents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcpagents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcpagents/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// MCPAgentReconciler reconciles an MCPAgent object.
type MCPAgentReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	KeycloakClient *keycloak.Client
	KeycloakAdmin  string
	KeycloakPass   string
}

// Reconcile handles the reconciliation loop for MCPAgent resources.
func (r *MCPAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var agent mcpv1alpha1.MCPAgent
	if err := r.Get(ctx, req.NamespacedName, &agent); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("MCPAgent resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion.
	if !agent.DeletionTimestamp.IsZero() {
		return r.reconcileAgentDelete(ctx, &agent)
	}

	// Ensure finalizer is present.
	if !controllerutil.ContainsFinalizer(&agent, agentFinalizerName) {
		controllerutil.AddFinalizer(&agent, agentFinalizerName)
		if err := r.Update(ctx, &agent); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Determine clientID: use spec value or default to "agent-<name>".
	clientID := agent.Spec.Identity.OIDCClientID
	if clientID == "" {
		clientID = fmt.Sprintf("agent-%s", agent.Name)
	}

	// If phase is empty or Pending, transition to Registering.
	if agent.Status.Phase == "" || agent.Status.Phase == mcpv1alpha1.MCPAgentPhasePending {
		agent.Status.Phase = mcpv1alpha1.MCPAgentPhaseRegistering
		if err := r.Status().Update(ctx, &agent); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Guard: Keycloak client must be configured.
	if r.KeycloakClient == nil {
		logger.Info("KeycloakClient not configured, skipping agent registration")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Get admin token from Keycloak.
	adminToken, err := r.KeycloakClient.GetAdminToken(ctx, r.KeycloakAdmin, r.KeycloakPass)
	if err != nil {
		logger.Error(err, "Failed to get Keycloak admin token")
		agent.Status.Phase = mcpv1alpha1.MCPAgentPhaseFailed
		_ = r.Status().Update(ctx, &agent)
		return ctrl.Result{RequeueAfter: requeueFailed}, err
	}

	// Check if a credentials Secret already exists.
	secretName := fmt.Sprintf("%s-credentials", agent.Name)
	var existingSecret corev1.Secret
	secretExists := true
	if err := r.Get(ctx, types.NamespacedName{Namespace: agent.Namespace, Name: secretName}, &existingSecret); err != nil {
		if apierrors.IsNotFound(err) {
			secretExists = false
		} else {
			return ctrl.Result{}, err
		}
	}

	if !secretExists {
		// Create the Keycloak client.
		clientSecret, err := r.KeycloakClient.CreateClient(ctx, adminToken, keycloak.ClientRepresentation{
			ClientID:               clientID,
			Enabled:                true,
			ServiceAccountsEnabled: true,
			PublicClient:           false,
			ProtocolMappers: []keycloak.ProtocolMapper{
				{
					Name:           "agent_id",
					Protocol:       "openid-connect",
					ProtocolMapper: "oidc-hardcoded-claim-mapper",
					Config: map[string]string{
						"claim.name":         "agent_id",
						"claim.value":        agent.Name,
						"jsonType.label":     "String",
						"id.token.claim":     "true",
						"access.token.claim": "true",
					},
				},
				{
					Name:           "audience",
					Protocol:       "openid-connect",
					ProtocolMapper: "oidc-audience-mapper",
					Config: map[string]string{
						"included.client.audience": "mcp-gateway",
						"id.token.claim":           "false",
						"access.token.claim":       "true",
					},
				},
			},
		})
		if err != nil {
			logger.Error(err, "Failed to create Keycloak client", "clientID", clientID)
			agent.Status.Phase = mcpv1alpha1.MCPAgentPhaseFailed
			_ = r.Status().Update(ctx, &agent)
			return ctrl.Result{RequeueAfter: requeueFailed}, err
		}

		// Build the token URL from the Keycloak client config.
		tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
			r.KeycloakClient.BaseURL, r.KeycloakClient.Realm)

		// Create K8s Secret with credentials.
		credentialSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: agent.Namespace,
			},
			StringData: map[string]string{
				"client-id":     clientID,
				"client-secret": clientSecret,
				"token-url":     tokenURL,
			},
		}

		// Set ownerReference so the Secret is garbage-collected with the MCPAgent.
		if err := controllerutil.SetControllerReference(&agent, credentialSecret, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting owner reference on Secret: %w", err)
		}

		if err := r.Create(ctx, credentialSecret); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				logger.Error(err, "Failed to create credentials Secret")
				return ctrl.Result{}, err
			}
		}

		logger.Info("Created credentials Secret", "secret", secretName, "clientID", clientID)
	}

	// Update status: Phase=Active, RegisteredAt=now, ClientSecretRef.
	now := metav1.Now()
	agent.Status.Phase = mcpv1alpha1.MCPAgentPhaseActive
	agent.Status.RegisteredAt = &now
	agent.Status.ClientSecretRef = &mcpv1alpha1.LocalObjectReference{Name: secretName}
	if err := r.Status().Update(ctx, &agent); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("MCPAgent reconciled successfully", "agent", agent.Name, "phase", agent.Status.Phase)
	return ctrl.Result{}, nil
}

// reconcileAgentDelete handles the deletion of an MCPAgent by removing the
// Keycloak client and the finalizer.
func (r *MCPAgentReconciler) reconcileAgentDelete(ctx context.Context, agent *mcpv1alpha1.MCPAgent) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(agent, agentFinalizerName) {
		// Clean up Keycloak client (best-effort).
		if r.KeycloakClient != nil {
			clientID := agent.Spec.Identity.OIDCClientID
			if clientID == "" {
				clientID = fmt.Sprintf("agent-%s", agent.Name)
			}

			adminToken, err := r.KeycloakClient.GetAdminToken(ctx, r.KeycloakAdmin, r.KeycloakPass)
			if err != nil {
				logger.Error(err, "Failed to get Keycloak admin token during cleanup, continuing")
			} else {
				if err := r.KeycloakClient.DeleteClient(ctx, adminToken, clientID); err != nil {
					logger.Error(err, "Failed to delete Keycloak client during cleanup, continuing", "clientID", clientID)
				} else {
					logger.Info("Deleted Keycloak client", "clientID", clientID)
				}
			}
		}

		// Remove finalizer.
		controllerutil.RemoveFinalizer(agent, agentFinalizerName)
		if err := r.Update(ctx, agent); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the MCPAgentReconciler with the controller manager.
func (r *MCPAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcpv1alpha1.MCPAgent{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
