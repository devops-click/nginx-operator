/*
Copyright 2024 DevOps Click.

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

// Package controller implements the Kubernetes controllers (reconcilers) for
// the NGINX Operator CRDs. Each controller watches its respective CRD and
// reconciles the desired state into the cluster.
package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
	"github.com/devops-click/nginx-operator/internal/config"
	"github.com/devops-click/nginx-operator/internal/nginx"
	"github.com/devops-click/nginx-operator/internal/version"
)

// NginxServerReconciler reconciles NginxServer objects.
// It creates and manages Deployments, Services, and ConfigMaps for NGINX instances.
//
// The reconciliation loop:
//  1. Fetches the NginxServer resource
//  2. Handles finalizer for cleanup
//  3. Collects associated NginxRoute and NginxUpstream resources
//  4. Generates NGINX configuration
//  5. Creates/updates ConfigMap, Deployment, and Service
//  6. Updates status with current state
type NginxServerReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	ConfigGen      *config.Generator
	ResourceMgr    *nginx.ResourceManager
	ReloaderTag    string
}

// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxroutes,verbs=get;list;watch
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxupstreams,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is the main reconciliation loop for NginxServer resources.
// It ensures the actual cluster state matches the desired state defined in the CRD.
func (r *NginxServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling NginxServer", "name", req.NamespacedName)

	// 1. Fetch the NginxServer resource
	server := &nginxv1alpha1.NginxServer{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("NginxServer resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get NginxServer: %w", err)
	}

	// 2. Handle deletion with finalizer
	if !server.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, logger, server)
	}

	// 3. Ensure finalizer is set
	if !controllerutil.ContainsFinalizer(server, nginxv1alpha1.NginxServerFinalizer) {
		controllerutil.AddFinalizer(server, nginxv1alpha1.NginxServerFinalizer)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Validate the spec
	validation := config.ValidateNginxServer(server)
	if !validation.Valid {
		logger.Error(nil, "NginxServer spec validation failed", "errors", validation.ErrorMessages())
		return r.updateStatusCondition(ctx, server, nginxv1alpha1.ConditionReady,
			metav1.ConditionFalse, nginxv1alpha1.ReasonFailed, validation.ErrorMessages())
	}

	// 5. Collect associated routes and upstreams
	routes, err := r.listRoutesForServer(ctx, server)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list routes: %w", err)
	}

	upstreams, err := r.listUpstreamsForServer(ctx, server)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list upstreams: %w", err)
	}

	// 6. Generate full NGINX configuration
	fullConfig, err := r.ConfigGen.GenerateFullConfig(server, routes, upstreams)
	if err != nil {
		logger.Error(err, "failed to generate NGINX configuration")
		if _, statusErr := r.updateStatusCondition(ctx, server, nginxv1alpha1.ConditionConfigValid,
			metav1.ConditionFalse, nginxv1alpha1.ReasonConfigInvalid, err.Error()); statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	configHash := r.ConfigGen.Hash(fullConfig)

	// 7. Create/update ConfigMap
	if err := r.reconcileConfigMap(ctx, logger, server, fullConfig); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile ConfigMap: %w", err)
	}

	// 8. Create/update Deployment
	if err := r.reconcileDeployment(ctx, logger, server, configHash); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile Deployment: %w", err)
	}

	// 9. Create/update Service
	if err := r.reconcileService(ctx, logger, server); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile Service: %w", err)
	}

	// 10. Update status
	return r.updateStatus(ctx, logger, server, configHash, int32(len(routes)), int32(len(upstreams)))
}

// handleDeletion handles cleanup when an NginxServer is being deleted.
func (r *NginxServerReconciler) handleDeletion(ctx context.Context, logger logr.Logger, server *nginxv1alpha1.NginxServer) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(server, nginxv1alpha1.NginxServerFinalizer) {
		logger.Info("performing cleanup for NginxServer deletion")

		// Owned resources (Deployment, Service, ConfigMap) are automatically garbage collected
		// via OwnerReferences. We only need to remove the finalizer.

		controllerutil.RemoveFinalizer(server, nginxv1alpha1.NginxServerFinalizer)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// reconcileConfigMap creates or updates the NGINX configuration ConfigMap.
func (r *NginxServerReconciler) reconcileConfigMap(ctx context.Context, logger logr.Logger, server *nginxv1alpha1.NginxServer, fullConfig string) error {
	// Split config into main config and server blocks
	serverConfigs := make(map[string]string)

	desired := r.ResourceMgr.BuildConfigMap(server, fullConfig, serverConfigs)

	if err := r.ResourceMgr.SetOwnerReference(server, desired); err != nil {
		return fmt.Errorf("failed to set owner reference on ConfigMap: %w", err)
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("creating ConfigMap", "name", desired.Name)
			return r.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	// Update if data changed
	if !equality.Semantic.DeepEqual(existing.Data, desired.Data) {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		logger.Info("updating ConfigMap", "name", desired.Name)
		return r.Update(ctx, existing)
	}

	return nil
}

// reconcileDeployment creates or updates the NGINX Deployment.
func (r *NginxServerReconciler) reconcileDeployment(ctx context.Context, logger logr.Logger, server *nginxv1alpha1.NginxServer, configHash string) error {
	reloaderTag := r.ReloaderTag
	if reloaderTag == "" {
		reloaderTag = version.Version
	}

	desired := r.ResourceMgr.BuildDeployment(server, reloaderTag)

	if err := r.ResourceMgr.SetOwnerReference(server, desired); err != nil {
		return fmt.Errorf("failed to set owner reference on Deployment: %w", err)
	}

	// Add config hash as pod annotation to trigger rolling update on config change
	if desired.Spec.Template.Annotations == nil {
		desired.Spec.Template.Annotations = make(map[string]string)
	}
	desired.Spec.Template.Annotations[nginxv1alpha1.AnnotationConfigHash] = configHash

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("creating Deployment", "name", desired.Name)
			return r.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	// Update if spec changed
	if !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		logger.Info("updating Deployment", "name", desired.Name)
		return r.Update(ctx, existing)
	}

	return nil
}

// reconcileService creates or updates the NGINX Service.
func (r *NginxServerReconciler) reconcileService(ctx context.Context, logger logr.Logger, server *nginxv1alpha1.NginxServer) error {
	desired := r.ResourceMgr.BuildService(server)

	if err := r.ResourceMgr.SetOwnerReference(server, desired); err != nil {
		return fmt.Errorf("failed to set owner reference on Service: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("creating Service", "name", desired.Name)
			return r.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get Service: %w", err)
	}

	// Update if spec changed (preserving ClusterIP)
	if !equality.Semantic.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) ||
		existing.Spec.Type != desired.Spec.Type ||
		!equality.Semantic.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
		existing.Spec.Ports = desired.Spec.Ports
		existing.Spec.Type = desired.Spec.Type
		existing.Spec.Selector = desired.Spec.Selector
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations
		logger.Info("updating Service", "name", desired.Name)
		return r.Update(ctx, existing)
	}

	return nil
}

// listRoutesForServer returns all NginxRoutes that reference this NginxServer.
func (r *NginxServerReconciler) listRoutesForServer(ctx context.Context, server *nginxv1alpha1.NginxServer) ([]nginxv1alpha1.NginxRoute, error) {
	routeList := &nginxv1alpha1.NginxRouteList{}
	if err := r.List(ctx, routeList, client.InNamespace(server.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list NginxRoutes: %w", err)
	}

	var routes []nginxv1alpha1.NginxRoute
	for _, route := range routeList.Items {
		if route.Spec.ServerRef == server.Name {
			routes = append(routes, route)
		}
	}

	return routes, nil
}

// listUpstreamsForServer returns all NginxUpstreams that reference this NginxServer.
func (r *NginxServerReconciler) listUpstreamsForServer(ctx context.Context, server *nginxv1alpha1.NginxServer) ([]nginxv1alpha1.NginxUpstream, error) {
	upstreamList := &nginxv1alpha1.NginxUpstreamList{}
	if err := r.List(ctx, upstreamList, client.InNamespace(server.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list NginxUpstreams: %w", err)
	}

	var upstreams []nginxv1alpha1.NginxUpstream
	for _, upstream := range upstreamList.Items {
		if upstream.Spec.ServerRef == server.Name {
			upstreams = append(upstreams, upstream)
		}
	}

	return upstreams, nil
}

// updateStatus updates the NginxServer status with current state.
func (r *NginxServerReconciler) updateStatus(ctx context.Context, logger logr.Logger, server *nginxv1alpha1.NginxServer, configHash string, routeCount, upstreamCount int32) (ctrl.Result, error) {
	// Fetch current deployment status
	deployment := &appsv1.Deployment{}
	deployReady := false
	err := r.Get(ctx, types.NamespacedName{
		Name:      nginx.DeploymentName(server),
		Namespace: server.Namespace,
	}, deployment)
	if err == nil {
		server.Status.ReadyReplicas = deployment.Status.ReadyReplicas
		server.Status.AvailableReplicas = deployment.Status.AvailableReplicas
		deployReady = deployment.Status.ReadyReplicas > 0 &&
			deployment.Status.ReadyReplicas == deployment.Status.Replicas
	}

	server.Status.ConfigHash = configHash
	server.Status.ObservedGeneration = server.Generation
	server.Status.RouteCount = routeCount
	server.Status.UpstreamCount = upstreamCount
	now := metav1.Now()
	server.Status.LastReloadTime = &now

	// Set conditions
	readyStatus := metav1.ConditionTrue
	readyReason := nginxv1alpha1.ReasonReconciled
	readyMessage := "NginxServer is fully reconciled and ready"
	if !deployReady {
		readyStatus = metav1.ConditionFalse
		readyReason = nginxv1alpha1.ReasonDeploymentNotReady
		readyMessage = "Deployment is not yet ready"
	}

	setCondition(&server.Status.Conditions, nginxv1alpha1.ConditionReady, readyStatus, readyReason, readyMessage, server.Generation)
	setCondition(&server.Status.Conditions, nginxv1alpha1.ConditionConfigValid, metav1.ConditionTrue, nginxv1alpha1.ReasonConfigGenerated, "Configuration generated successfully", server.Generation)
	setCondition(&server.Status.Conditions, nginxv1alpha1.ConditionConfigApplied, metav1.ConditionTrue, nginxv1alpha1.ReasonConfigApplied, "Configuration applied to ConfigMap", server.Generation)

	if err := r.Status().Update(ctx, server); err != nil {
		logger.Error(err, "failed to update NginxServer status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// Requeue if deployment not ready yet
	if !deployReady {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// updateStatusCondition is a helper for updating a single condition on the status.
func (r *NginxServerReconciler) updateStatusCondition(ctx context.Context, server *nginxv1alpha1.NginxServer, condType string, status metav1.ConditionStatus, reason, message string) (ctrl.Result, error) {
	setCondition(&server.Status.Conditions, condType, status, reason, message, server.Generation)
	if err := r.Status().Update(ctx, server); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
// It watches NginxServer resources and also watches owned Deployments, Services, and ConfigMaps.
func (r *NginxServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nginxv1alpha1.NginxServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

// setCondition updates or adds a condition to the conditions slice.
func setCondition(conditions *[]metav1.Condition, condType string, status metav1.ConditionStatus, reason, message string, generation int64) {
	now := metav1.Now()
	for i, c := range *conditions {
		if c.Type == condType {
			if c.Status != status {
				(*conditions)[i].LastTransitionTime = now
			}
			(*conditions)[i].Status = status
			(*conditions)[i].Reason = reason
			(*conditions)[i].Message = message
			(*conditions)[i].ObservedGeneration = generation
			return
		}
	}

	*conditions = append(*conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	})
}
