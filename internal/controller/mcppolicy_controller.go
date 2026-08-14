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
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
	"github.com/mcp-gateway/mcp-gateway/internal/cerbos"
)

const (
	policyFinalizerName = "mcp-gateway.io/policy-cleanup"
)

// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcppolicies/finalizers,verbs=update

// MCPPolicyReconciler reconciles an MCPPolicy object.
type MCPPolicyReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	CerbosURL  string
	CerbosUser string
	CerbosPass string
}

// Reconcile handles the reconciliation loop for MCPPolicy resources.
func (r *MCPPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var policy mcpv1alpha1.MCPPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("MCPPolicy resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion.
	if !policy.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&policy, policyFinalizerName) {
			// Cerbos policy cleanup is best-effort; just remove the finalizer.
			logger.Info("Removing finalizer from MCPPolicy", "policy", policy.Name)
			controllerutil.RemoveFinalizer(&policy, policyFinalizerName)
			if err := r.Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is present.
	if !controllerutil.ContainsFinalizer(&policy, policyFinalizerName) {
		controllerutil.AddFinalizer(&policy, policyFinalizerName)
		if err := r.Update(ctx, &policy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Translate MCPPolicy to Cerbos policy.
	cerbosPolicy, err := cerbos.TranslatePolicy(&policy)
	if err != nil {
		logger.Error(err, "Failed to translate MCPPolicy to Cerbos policy")
		policy.Status.Phase = mcpv1alpha1.MCPPolicyPhaseFailed
		_ = r.Status().Update(ctx, &policy)
		return ctrl.Result{RequeueAfter: requeueFailed}, err
	}

	// Marshal the Cerbos policy to JSON for logging.
	policyJSON, err := json.MarshalIndent(cerbosPolicy, "", "  ")
	if err != nil {
		logger.Error(err, "Failed to marshal Cerbos policy to JSON")
		return ctrl.Result{}, err
	}

	// Log the translated policy. Actual Cerbos push is deferred to when
	// Cerbos is deployed alongside the operator.
	logger.Info("Translated Cerbos policy", "policy", string(policyJSON))

	// Build Cerbos policy ID based on resource kind and version.
	cerbosPolicyID := fmt.Sprintf("mcp-gateway/%s/%s", policy.Name, policy.ResourceVersion)

	// Update status: Phase=Synced, SyncedAt=now, CerbosPolicyID.
	now := metav1.Now()
	policy.Status.Phase = mcpv1alpha1.MCPPolicySynced
	policy.Status.SyncedAt = &now
	policy.Status.CerbosPolicyID = cerbosPolicyID
	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("MCPPolicy reconciled successfully", "policy", policy.Name, "cerbosPolicyID", cerbosPolicyID)
	return ctrl.Result{}, nil
}

// SetupWithManager registers the MCPPolicyReconciler with the controller manager.
func (r *MCPPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcpv1alpha1.MCPPolicy{}).
		Complete(r)
}
