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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
	envoyPkg "github.com/mcp-gateway/mcp-gateway/internal/envoy"
)

const (
	finalizerName = "mcp.mcp-gateway.io/finalizer"

	requeueDeploying = 5 * time.Second
	requeueRunning   = 30 * time.Second
	requeueFailed    = 60 * time.Second

	labelName      = "app.kubernetes.io/name"
	labelInstance   = "app.kubernetes.io/instance"
	labelManagedBy = "app.kubernetes.io/managed-by"

	conditionReady    = "Ready"
	conditionDeployed = "Deployed"
)

// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcpservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcpservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mcp.mcp-gateway.io,resources=mcpservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete

// MCPServerReconciler reconciles an MCPServer object.
type MCPServerReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	GatewayName      string
	GatewayNamespace string
}

// Reconcile handles the reconciliation loop for MCPServer resources.
func (r *MCPServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var server mcpv1alpha1.MCPServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("MCPServer resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion.
	if !server.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &server)
	}

	// Ensure finalizer is present.
	if !controllerutil.ContainsFinalizer(&server, finalizerName) {
		controllerutil.AddFinalizer(&server, finalizerName)
		if err := r.Update(ctx, &server); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// State machine dispatch.
	switch server.Status.Phase {
	case "", mcpv1alpha1.MCPServerPhasePending:
		return r.reconcilePending(ctx, &server)
	case mcpv1alpha1.MCPServerPhaseDeploying:
		return r.reconcileDeploying(ctx, &server)
	case mcpv1alpha1.MCPServerPhaseRunning:
		return r.reconcileRunning(ctx, &server)
	case mcpv1alpha1.MCPServerPhaseUpdating:
		return r.reconcileUpdating(ctx, &server)
	case mcpv1alpha1.MCPServerPhaseFailed:
		return r.reconcileFailed(ctx, &server)
	default:
		logger.Info("Unknown phase, resetting to Pending", "phase", server.Status.Phase)
		return r.setPhase(ctx, &server, mcpv1alpha1.MCPServerPhasePending, "Unknown phase, resetting")
	}
}

// reconcilePending validates the spec and referenced secrets, then transitions to Deploying.
func (r *MCPServerReconciler) reconcilePending(ctx context.Context, server *mcpv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Validate that the image is specified.
	if server.Spec.Source.Image == "" {
		logger.Info("MCPServer has no image specified")
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ValidationFailed",
			Message:            "spec.source.image must not be empty",
			ObservedGeneration: server.Generation,
		})
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, "spec.source.image must not be empty")
	}

	// Validate that referenced secrets exist.
	for _, secret := range server.Spec.Secrets {
		var s corev1.Secret
		key := types.NamespacedName{Namespace: server.Namespace, Name: secret.SecretRef.Name}
		if err := r.Get(ctx, key, &s); err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("referenced secret %q not found", secret.SecretRef.Name)
				logger.Info(msg)
				meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
					Type:               conditionReady,
					Status:             metav1.ConditionFalse,
					Reason:             "SecretNotFound",
					Message:            msg,
					ObservedGeneration: server.Generation,
				})
				return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, msg)
			}
			return ctrl.Result{}, err
		}
	}

	logger.Info("Validation passed, transitioning to Deploying")
	return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseDeploying, "Validation passed")
}

// reconcileDeploying creates or updates the Deployment and Service, then checks readiness.
func (r *MCPServerReconciler) reconcileDeploying(ctx context.Context, server *mcpv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Create or update the Deployment.
	if err := r.reconcileDeployment(ctx, server); err != nil {
		logger.Error(err, "Failed to reconcile Deployment")
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               conditionDeployed,
			Status:             metav1.ConditionFalse,
			Reason:             "DeploymentFailed",
			Message:            err.Error(),
			ObservedGeneration: server.Generation,
		})
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, fmt.Sprintf("Failed to reconcile Deployment: %v", err))
	}

	// Create or update the Service.
	if err := r.reconcileService(ctx, server); err != nil {
		logger.Error(err, "Failed to reconcile Service")
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               conditionDeployed,
			Status:             metav1.ConditionFalse,
			Reason:             "ServiceFailed",
			Message:            err.Error(),
			ObservedGeneration: server.Generation,
		})
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, fmt.Sprintf("Failed to reconcile Service: %v", err))
	}

	meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
		Type:               conditionDeployed,
		Status:             metav1.ConditionTrue,
		Reason:             "DeploymentCreated",
		Message:            "Deployment and Service created",
		ObservedGeneration: server.Generation,
	})

	// Check if the Deployment is ready.
	deploy, err := r.getDeployment(ctx, server)
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueDeploying}, nil
	}

	server.Status.Replicas = deploy.Status.Replicas
	server.Status.ReadyReplicas = deploy.Status.ReadyReplicas
	server.Status.DeploymentName = deploy.Name
	server.Status.ServiceName = serviceName(server)

	if deploy.Status.ReadyReplicas > 0 && deploy.Status.ReadyReplicas == deploy.Status.Replicas {
		logger.Info("Deployment is ready, transitioning to Running")

		// Reconcile HTTPRoute for gateway integration.
		if err := r.reconcileHTTPRoute(ctx, server); err != nil {
			logger.Error(err, "Failed to reconcile HTTPRoute")
			return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, fmt.Sprintf("Failed to reconcile HTTPRoute: %v", err))
		}

		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "DeploymentReady",
			Message:            "All replicas are ready",
			ObservedGeneration: server.Generation,
		})
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseRunning, "All replicas are ready")
	}

	logger.Info("Waiting for Deployment to become ready", "ready", deploy.Status.ReadyReplicas, "desired", deploy.Status.Replicas)
	if err := r.statusUpdate(ctx, server); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueDeploying}, nil
}

// reconcileRunning monitors the running MCPServer for spec changes and replica health.
func (r *MCPServerReconciler) reconcileRunning(ctx context.Context, server *mcpv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Detect spec change (generation drift).
	if server.Generation != server.Status.ObservedGeneration {
		logger.Info("Spec changed, transitioning to Updating")
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseUpdating, "Spec changed, updating")
	}

	// Check the current Deployment status.
	deploy, err := r.getDeployment(ctx, server)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Deployment disappeared, transitioning to Deploying")
			return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseDeploying, "Deployment not found, redeploying")
		}
		return ctrl.Result{}, err
	}

	server.Status.Replicas = deploy.Status.Replicas
	server.Status.ReadyReplicas = deploy.Status.ReadyReplicas

	if deploy.Status.ReadyReplicas < deploy.Status.Replicas {
		logger.Info("Not all replicas ready", "ready", deploy.Status.ReadyReplicas, "desired", deploy.Status.Replicas)
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ReplicasNotReady",
			Message:            fmt.Sprintf("%d/%d replicas ready", deploy.Status.ReadyReplicas, deploy.Status.Replicas),
			ObservedGeneration: server.Generation,
		})
	} else {
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "DeploymentReady",
			Message:            "All replicas are ready",
			ObservedGeneration: server.Generation,
		})
	}

	if err := r.statusUpdate(ctx, server); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueRunning}, nil
}

// reconcileUpdating reconciles the Deployment and Service after a spec change,
// and transitions back to Running once the rollout is complete.
func (r *MCPServerReconciler) reconcileUpdating(ctx context.Context, server *mcpv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if err := r.reconcileDeployment(ctx, server); err != nil {
		logger.Error(err, "Failed to update Deployment")
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, fmt.Sprintf("Failed to update Deployment: %v", err))
	}

	if err := r.reconcileService(ctx, server); err != nil {
		logger.Error(err, "Failed to update Service")
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, fmt.Sprintf("Failed to update Service: %v", err))
	}

	deploy, err := r.getDeployment(ctx, server)
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueDeploying}, nil
	}

	server.Status.Replicas = deploy.Status.Replicas
	server.Status.ReadyReplicas = deploy.Status.ReadyReplicas

	if deploy.Status.ReadyReplicas > 0 &&
		deploy.Status.ReadyReplicas == deploy.Status.Replicas &&
		deploy.Status.UpdatedReplicas == deploy.Status.Replicas {
		logger.Info("Rollout complete, transitioning to Running")

		// Reconcile HTTPRoute for gateway integration.
		if err := r.reconcileHTTPRoute(ctx, server); err != nil {
			logger.Error(err, "Failed to reconcile HTTPRoute")
			return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseFailed, fmt.Sprintf("Failed to reconcile HTTPRoute: %v", err))
		}

		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "RolloutComplete",
			Message:            "Update rollout completed",
			ObservedGeneration: server.Generation,
		})
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhaseRunning, "Update rollout completed")
	}

	logger.Info("Waiting for rollout to complete", "updated", deploy.Status.UpdatedReplicas, "ready", deploy.Status.ReadyReplicas)
	if err := r.statusUpdate(ctx, server); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeueDeploying}, nil
}

// reconcileFailed checks if the spec has been changed (generation drift) and retries by resetting to Pending.
func (r *MCPServerReconciler) reconcileFailed(ctx context.Context, server *mcpv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if server.Generation != server.Status.ObservedGeneration {
		logger.Info("Spec changed on failed resource, retrying")
		return r.setPhase(ctx, server, mcpv1alpha1.MCPServerPhasePending, "Spec changed, retrying")
	}

	return ctrl.Result{RequeueAfter: requeueFailed}, nil
}

// reconcileDelete handles the deletion of an MCPServer by removing the finalizer.
// Owned resources (Deployment, Service) are garbage-collected via OwnerReferences.
func (r *MCPServerReconciler) reconcileDelete(ctx context.Context, server *mcpv1alpha1.MCPServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(server, finalizerName) {
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(server, finalizerName)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// ---------------------------------------------------------------------------
// Deployment helpers
// ---------------------------------------------------------------------------

func (r *MCPServerReconciler) reconcileDeployment(ctx context.Context, server *mcpv1alpha1.MCPServer) error {
	desired := r.buildDeployment(server)
	if err := controllerutil.SetControllerReference(server, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on Deployment: %w", err)
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		log.FromContext(ctx).Info("Creating Deployment", "name", desired.Name)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update only if the spec has diverged.
	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		log.FromContext(ctx).Info("Updating Deployment", "name", desired.Name)
		return r.Update(ctx, &existing)
	}

	return nil
}

func (r *MCPServerReconciler) buildDeployment(server *mcpv1alpha1.MCPServer) *appsv1.Deployment {
	labels := commonLabels(server)

	replicas := int32(1)
	if server.Spec.Scaling != nil && server.Spec.Scaling.MinReplicas != nil {
		replicas = *server.Spec.Scaling.MinReplicas
	}

	// Build environment variables from secrets.
	var envVars []corev1.EnvVar
	for _, s := range server.Spec.Secrets {
		envVars = append(envVars, corev1.EnvVar{
			Name: s.EnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: s.SecretRef.Name},
					Key:                  s.SecretRef.Key,
				},
			},
		})
	}
	// Append plain env vars from spec.
	envVars = append(envVars, server.Spec.Env...)

	// Main container.
	mainContainer := corev1.Container{
		Name:  "mcp-server",
		Image: server.Spec.Source.Image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "mcp",
				ContainerPort: server.Spec.Source.Port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: envVars,
	}

	// Resources.
	if server.Spec.Resources != nil {
		mainContainer.Resources = *server.Spec.Resources
	}

	// Health probes.
	if server.Spec.Source.HealthCheck != nil {
		probe := &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: server.Spec.Source.HealthCheck.Path,
					Port: intstr.FromInt32(server.Spec.Source.Port),
				},
			},
			PeriodSeconds: server.Spec.Source.HealthCheck.PeriodSeconds,
		}
		mainContainer.LivenessProbe = probe
		mainContainer.ReadinessProbe = probe.DeepCopy()
	}

	containers := []corev1.Container{mainContainer}

	// For stdio transport, add a sidecar bridge container.
	if server.Spec.Protocol.Transport == mcpv1alpha1.TransportStdio {
		stdioBridge := corev1.Container{
			Name:  "stdio-bridge",
			Image: server.Spec.Source.Image,
			Ports: []corev1.ContainerPort{
				{
					Name:          "bridge",
					ContainerPort: server.Spec.Source.Port,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Env: envVars,
		}
		if server.Spec.Resources != nil {
			stdioBridge.Resources = *server.Spec.Resources
		}
		containers = append(containers, stdioBridge)
	}

	podAnnotations := make(map[string]string)
	for k, v := range server.Spec.PodAnnotations {
		podAnnotations[k] = v
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(server),
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}

	return deploy
}

func (r *MCPServerReconciler) getDeployment(ctx context.Context, server *mcpv1alpha1.MCPServer) (*appsv1.Deployment, error) {
	var deploy appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Namespace: server.Namespace, Name: deploymentName(server)}, &deploy)
	return &deploy, err
}

// ---------------------------------------------------------------------------
// HTTPRoute helpers
// ---------------------------------------------------------------------------

func (r *MCPServerReconciler) reconcileHTTPRoute(ctx context.Context, server *mcpv1alpha1.MCPServer) error {
	if r.GatewayName == "" {
		return nil // gateway not configured
	}

	desired := envoyPkg.BuildHTTPRoute(server, r.GatewayName, r.GatewayNamespace)
	if err := controllerutil.SetControllerReference(server, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on HTTPRoute: %w", err)
	}

	var existing gatewayv1.HTTPRoute
	err := r.Get(ctx, types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		log.FromContext(ctx).Info("Creating HTTPRoute", "name", desired.Name)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update only if the spec has diverged.
	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		log.FromContext(ctx).Info("Updating HTTPRoute", "name", desired.Name)
		return r.Update(ctx, &existing)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Service helpers
// ---------------------------------------------------------------------------

func (r *MCPServerReconciler) reconcileService(ctx context.Context, server *mcpv1alpha1.MCPServer) error {
	desired := r.buildService(server)
	if err := controllerutil.SetControllerReference(server, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on Service: %w", err)
	}

	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		log.FromContext(ctx).Info("Creating Service", "name", desired.Name)
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update only if the spec has diverged.
	if !equality.Semantic.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) ||
		!equality.Semantic.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
		existing.Spec.Ports = desired.Spec.Ports
		existing.Spec.Selector = desired.Spec.Selector
		existing.Annotations = desired.Annotations
		log.FromContext(ctx).Info("Updating Service", "name", desired.Name)
		return r.Update(ctx, &existing)
	}

	return nil
}

func (r *MCPServerReconciler) buildService(server *mcpv1alpha1.MCPServer) *corev1.Service {
	labels := commonLabels(server)

	serviceAnnotations := make(map[string]string)
	for k, v := range server.Spec.ServiceAnnotations {
		serviceAnnotations[k] = v
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName(server),
			Namespace:   server.Namespace,
			Labels:      labels,
			Annotations: serviceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "mcp",
					Port:       server.Spec.Source.Port,
					TargetPort: intstr.FromInt32(server.Spec.Source.Port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return svc
}

// ---------------------------------------------------------------------------
// Status helpers
// ---------------------------------------------------------------------------

// setPhase updates the phase, message, and observedGeneration on the MCPServer status.
func (r *MCPServerReconciler) setPhase(ctx context.Context, server *mcpv1alpha1.MCPServer, phase mcpv1alpha1.MCPServerPhase, message string) (ctrl.Result, error) {
	server.Status.Phase = phase
	server.Status.Message = message
	server.Status.ObservedGeneration = server.Generation

	if err := r.statusUpdate(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	switch phase {
	case mcpv1alpha1.MCPServerPhaseDeploying:
		return ctrl.Result{RequeueAfter: requeueDeploying}, nil
	case mcpv1alpha1.MCPServerPhaseRunning:
		return ctrl.Result{RequeueAfter: requeueRunning}, nil
	case mcpv1alpha1.MCPServerPhaseFailed:
		return ctrl.Result{RequeueAfter: requeueFailed}, nil
	default:
		return ctrl.Result{Requeue: true}, nil
	}
}

func (r *MCPServerReconciler) statusUpdate(ctx context.Context, server *mcpv1alpha1.MCPServer) error {
	return r.Status().Update(ctx, server)
}

// ---------------------------------------------------------------------------
// Naming and labeling helpers
// ---------------------------------------------------------------------------

func deploymentName(server *mcpv1alpha1.MCPServer) string {
	return server.Name
}

func serviceName(server *mcpv1alpha1.MCPServer) string {
	return server.Name
}

func commonLabels(server *mcpv1alpha1.MCPServer) map[string]string {
	return map[string]string{
		labelName:      "mcpserver",
		labelInstance:   server.Name,
		labelManagedBy: "mcp-gateway",
	}
}

// ---------------------------------------------------------------------------
// SetupWithManager
// ---------------------------------------------------------------------------

// SetupWithManager registers the MCPServerReconciler with the controller manager.
func (r *MCPServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcpv1alpha1.MCPServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Complete(r)
}
