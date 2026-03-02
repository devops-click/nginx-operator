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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
	"github.com/devops-click/nginx-operator/internal/config"
)

// NginxUpstreamReconciler reconciles NginxUpstream objects.
// When an NginxUpstream is created, updated, or deleted, this controller
// triggers a re-reconciliation of the referenced NginxServer.
//
// The reconciliation loop:
//  1. Fetches the NginxUpstream resource
//  2. Validates the spec
//  3. Ensures the referenced NginxServer exists
//  4. Handles service discovery if enabled
//  5. Triggers NginxServer reconciliation
//  6. Updates status
type NginxUpstreamReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ConfigGen *config.Generator
}

// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxupstreams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxupstreams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxupstreams/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch

// Reconcile handles NginxUpstream create/update/delete events.
func (r *NginxUpstreamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling NginxUpstream", "name", req.NamespacedName)

	// 1. Fetch the NginxUpstream resource
	upstream := &nginxv1alpha1.NginxUpstream{}
	if err := r.Get(ctx, req.NamespacedName, upstream); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("NginxUpstream resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get NginxUpstream: %w", err)
	}

	// 2. Handle deletion with finalizer
	if !upstream.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, logger, upstream)
	}

	// 3. Ensure finalizer is set
	if !controllerutil.ContainsFinalizer(upstream, nginxv1alpha1.NginxUpstreamFinalizer) {
		controllerutil.AddFinalizer(upstream, nginxv1alpha1.NginxUpstreamFinalizer)
		if err := r.Update(ctx, upstream); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Validate the spec
	validation := config.ValidateNginxUpstream(upstream)
	if !validation.Valid {
		logger.Error(nil, "NginxUpstream spec validation failed", "errors", validation.ErrorMessages())
		return r.updateStatusCondition(ctx, upstream, nginxv1alpha1.ConditionReady,
			metav1.ConditionFalse, nginxv1alpha1.ReasonFailed, validation.ErrorMessages())
	}

	// 5. Verify the referenced NginxServer exists
	server := &nginxv1alpha1.NginxServer{}
	serverKey := types.NamespacedName{Name: upstream.Spec.ServerRef, Namespace: upstream.Namespace}
	if err := r.Get(ctx, serverKey, server); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("referenced NginxServer not found", "serverRef", upstream.Spec.ServerRef)
			return r.updateStatusCondition(ctx, upstream, nginxv1alpha1.ConditionReady,
				metav1.ConditionFalse, nginxv1alpha1.ReasonServerNotFound,
				fmt.Sprintf("NginxServer %q not found", upstream.Spec.ServerRef))
		}
		return ctrl.Result{}, fmt.Errorf("failed to get NginxServer: %w", err)
	}

	// 6. Handle service discovery if enabled
	if upstream.Spec.ServiceDiscovery != nil && upstream.Spec.ServiceDiscovery.Enabled {
		if err := r.resolveServiceEndpoints(ctx, logger, upstream); err != nil {
			logger.Error(err, "failed to resolve service endpoints")
			return r.updateStatusCondition(ctx, upstream, nginxv1alpha1.ConditionReady,
				metav1.ConditionFalse, nginxv1alpha1.ReasonDependencyNotReady,
				fmt.Sprintf("Failed to resolve endpoints: %v", err))
		}
	}

	// 7. Trigger NginxServer reconciliation
	if err := r.triggerServerReconcile(ctx, logger, server); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// 8. Update status
	activeBackends := int32(0)
	totalBackends := int32(len(upstream.Spec.Backends))
	for _, b := range upstream.Spec.Backends {
		if !b.Down {
			activeBackends++
		}
	}

	now := metav1.Now()
	upstream.Status.ActiveBackends = activeBackends
	upstream.Status.TotalBackends = totalBackends
	upstream.Status.ObservedGeneration = upstream.Generation
	upstream.Status.LastAppliedTime = &now

	setCondition(&upstream.Status.Conditions, nginxv1alpha1.ConditionReady,
		metav1.ConditionTrue, nginxv1alpha1.ReasonReconciled,
		"NginxUpstream reconciled successfully", upstream.Generation)

	if err := r.Status().Update(ctx, upstream); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, fmt.Errorf("failed to update NginxUpstream status: %w", err)
	}

	// If service discovery is enabled, requeue periodically to refresh endpoints
	if upstream.Spec.ServiceDiscovery != nil && upstream.Spec.ServiceDiscovery.Enabled {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// handleDeletion handles cleanup when an NginxUpstream is being deleted.
func (r *NginxUpstreamReconciler) handleDeletion(ctx context.Context, logger logr.Logger, upstream *nginxv1alpha1.NginxUpstream) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(upstream, nginxv1alpha1.NginxUpstreamFinalizer) {
		logger.Info("performing cleanup for NginxUpstream deletion")

		// Trigger NginxServer reconciliation to remove this upstream's config
		server := &nginxv1alpha1.NginxServer{}
		serverKey := types.NamespacedName{Name: upstream.Spec.ServerRef, Namespace: upstream.Namespace}
		if err := r.Get(ctx, serverKey, server); err == nil {
			if triggerErr := r.triggerServerReconcile(ctx, logger, server); triggerErr != nil {
				logger.Error(triggerErr, "failed to trigger server reconciliation during upstream deletion")
			}
		}

		controllerutil.RemoveFinalizer(upstream, nginxv1alpha1.NginxUpstreamFinalizer)
		if err := r.Update(ctx, upstream); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// resolveServiceEndpoints discovers backend endpoints from a Kubernetes Service.
// It updates the upstream's DiscoveredEndpoints status field.
func (r *NginxUpstreamReconciler) resolveServiceEndpoints(ctx context.Context, logger logr.Logger, upstream *nginxv1alpha1.NginxUpstream) error {
	sd := upstream.Spec.ServiceDiscovery

	namespace := upstream.Namespace
	if sd.Namespace != "" {
		namespace = sd.Namespace
	}

	// Fetch the Endpoints resource
	endpoints := &corev1.Endpoints{}
	endpointKey := types.NamespacedName{Name: sd.ServiceName, Namespace: namespace}
	if err := r.Get(ctx, endpointKey, endpoints); err != nil {
		return fmt.Errorf("failed to get Endpoints for Service %s/%s: %w", namespace, sd.ServiceName, err)
	}

	// Extract addresses
	var discovered []string
	var backends []nginxv1alpha1.NginxBackendSpec
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			endpoint := fmt.Sprintf("%s:%d", addr.IP, sd.ServicePort)
			discovered = append(discovered, endpoint)
			backends = append(backends, nginxv1alpha1.NginxBackendSpec{
				Address:     addr.IP,
				Port:        sd.ServicePort,
				Weight:      1,
				MaxFails:    3,
				FailTimeout: "10s",
			})
		}
	}

	// Update the upstream's backends with discovered endpoints
	upstream.Spec.Backends = backends
	upstream.Status.DiscoveredEndpoints = discovered

	logger.Info("resolved service endpoints",
		"service", sd.ServiceName,
		"namespace", namespace,
		"endpoints", len(discovered))

	return nil
}

// triggerServerReconcile forces the NginxServer to reconcile.
func (r *NginxUpstreamReconciler) triggerServerReconcile(ctx context.Context, logger logr.Logger, server *nginxv1alpha1.NginxServer) error {
	if server.Annotations == nil {
		server.Annotations = make(map[string]string)
	}
	server.Annotations[nginxv1alpha1.AnnotationLastReload] = time.Now().UTC().Format(time.RFC3339Nano)

	if err := r.Update(ctx, server); err != nil {
		return fmt.Errorf("failed to trigger NginxServer reconciliation: %w", err)
	}

	logger.Info("triggered NginxServer reconciliation", "server", server.Name)
	return nil
}

// updateStatusCondition updates a single condition on the NginxUpstream status.
func (r *NginxUpstreamReconciler) updateStatusCondition(ctx context.Context, upstream *nginxv1alpha1.NginxUpstream, condType string, status metav1.ConditionStatus, reason, message string) (ctrl.Result, error) {
	setCondition(&upstream.Status.Conditions, condType, status, reason, message, upstream.Generation)
	if err := r.Status().Update(ctx, upstream); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the NginxUpstream controller with the Manager.
func (r *NginxUpstreamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nginxv1alpha1.NginxUpstream{}).
		Watches(
			&nginxv1alpha1.NginxServer{},
			handler.EnqueueRequestsFromMapFunc(r.findUpstreamsForServer),
		).
		Complete(r)
}

// findUpstreamsForServer maps an NginxServer event to all NginxUpstreams that reference it.
func (r *NginxUpstreamReconciler) findUpstreamsForServer(ctx context.Context, obj client.Object) []reconcile.Request {
	server, ok := obj.(*nginxv1alpha1.NginxServer)
	if !ok {
		return nil
	}

	upstreamList := &nginxv1alpha1.NginxUpstreamList{}
	if err := r.List(ctx, upstreamList, client.InNamespace(server.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, upstream := range upstreamList.Items {
		if upstream.Spec.ServerRef == server.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      upstream.Name,
					Namespace: upstream.Namespace,
				},
			})
		}
	}

	return requests
}
