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

// NginxRouteReconciler reconciles NginxRoute objects.
// When an NginxRoute is created, updated, or deleted, this controller triggers
// a re-reconciliation of the referenced NginxServer to regenerate the full config.
//
// The reconciliation loop:
//  1. Fetches the NginxRoute resource
//  2. Validates the spec
//  3. Ensures the referenced NginxServer exists
//  4. Triggers NginxServer reconciliation by updating its annotation
//  5. Updates NginxRoute status
type NginxRouteReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	ConfigGen *config.Generator
}

// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxroutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=nginx.devops.click,resources=nginxservers,verbs=get;list;watch;update;patch

// Reconcile handles NginxRoute create/update/delete events.
// It validates the route configuration and triggers the parent NginxServer
// to regenerate its NGINX configuration.
func (r *NginxRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("reconciling NginxRoute", "name", req.NamespacedName)

	// 1. Fetch the NginxRoute resource
	route := &nginxv1alpha1.NginxRoute{}
	if err := r.Get(ctx, req.NamespacedName, route); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("NginxRoute resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get NginxRoute: %w", err)
	}

	// 2. Handle deletion with finalizer
	if !route.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, logger, route)
	}

	// 3. Ensure finalizer is set
	if !controllerutil.ContainsFinalizer(route, nginxv1alpha1.NginxRouteFinalizer) {
		controllerutil.AddFinalizer(route, nginxv1alpha1.NginxRouteFinalizer)
		if err := r.Update(ctx, route); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Validate the spec
	validation := config.ValidateNginxRoute(route)
	if !validation.Valid {
		logger.Error(nil, "NginxRoute spec validation failed", "errors", validation.ErrorMessages())
		return r.updateStatusCondition(ctx, route, nginxv1alpha1.ConditionReady,
			metav1.ConditionFalse, nginxv1alpha1.ReasonFailed, validation.ErrorMessages())
	}

	// 5. Verify the referenced NginxServer exists
	server := &nginxv1alpha1.NginxServer{}
	serverKey := types.NamespacedName{Name: route.Spec.ServerRef, Namespace: route.Namespace}
	if err := r.Get(ctx, serverKey, server); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("referenced NginxServer not found", "serverRef", route.Spec.ServerRef)
			return r.updateStatusCondition(ctx, route, nginxv1alpha1.ConditionReady,
				metav1.ConditionFalse, nginxv1alpha1.ReasonServerNotFound,
				fmt.Sprintf("NginxServer %q not found", route.Spec.ServerRef))
		}
		return ctrl.Result{}, fmt.Errorf("failed to get NginxServer: %w", err)
	}

	// 6. Trigger NginxServer reconciliation by touching its annotation
	if err := r.triggerServerReconcile(ctx, logger, server); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// 7. Update status
	now := metav1.Now()
	route.Status.ObservedGeneration = route.Generation
	route.Status.LastAppliedTime = &now
	setCondition(&route.Status.Conditions, nginxv1alpha1.ConditionReady,
		metav1.ConditionTrue, nginxv1alpha1.ReasonReconciled,
		"NginxRoute reconciled successfully", route.Generation)
	setCondition(&route.Status.Conditions, nginxv1alpha1.ConditionConfigValid,
		metav1.ConditionTrue, nginxv1alpha1.ReasonConfigGenerated,
		"Route configuration is valid", route.Generation)

	if err := r.Status().Update(ctx, route); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, fmt.Errorf("failed to update NginxRoute status: %w", err)
	}

	return ctrl.Result{}, nil
}

// handleDeletion cleans up when an NginxRoute is being deleted.
// It triggers a re-reconciliation of the referenced NginxServer to remove
// this route's config from the generated NGINX configuration.
func (r *NginxRouteReconciler) handleDeletion(ctx context.Context, logger logr.Logger, route *nginxv1alpha1.NginxRoute) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(route, nginxv1alpha1.NginxRouteFinalizer) {
		logger.Info("performing cleanup for NginxRoute deletion")

		// Trigger NginxServer reconciliation to remove this route's config
		server := &nginxv1alpha1.NginxServer{}
		serverKey := types.NamespacedName{Name: route.Spec.ServerRef, Namespace: route.Namespace}
		if err := r.Get(ctx, serverKey, server); err == nil {
			if triggerErr := r.triggerServerReconcile(ctx, logger, server); triggerErr != nil {
				logger.Error(triggerErr, "failed to trigger server reconciliation during route deletion")
			}
		}

		controllerutil.RemoveFinalizer(route, nginxv1alpha1.NginxRouteFinalizer)
		if err := r.Update(ctx, route); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// triggerServerReconcile forces the NginxServer to reconcile by updating an annotation.
func (r *NginxRouteReconciler) triggerServerReconcile(ctx context.Context, logger logr.Logger, server *nginxv1alpha1.NginxServer) error {
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

// updateStatusCondition updates a single condition on the NginxRoute status.
func (r *NginxRouteReconciler) updateStatusCondition(ctx context.Context, route *nginxv1alpha1.NginxRoute, condType string, status metav1.ConditionStatus, reason, message string) (ctrl.Result, error) {
	setCondition(&route.Status.Conditions, condType, status, reason, message, route.Generation)
	if err := r.Status().Update(ctx, route); err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the NginxRoute controller with the Manager.
// It watches NginxRoute resources and triggers NginxServer reconciliation
// when routes change.
func (r *NginxRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nginxv1alpha1.NginxRoute{}).
		// Watch NginxServer changes to re-validate route references
		Watches(
			&nginxv1alpha1.NginxServer{},
			handler.EnqueueRequestsFromMapFunc(r.findRoutesForServer),
		).
		Complete(r)
}

// findRoutesForServer maps an NginxServer event to all NginxRoutes that reference it.
func (r *NginxRouteReconciler) findRoutesForServer(ctx context.Context, obj client.Object) []reconcile.Request {
	server, ok := obj.(*nginxv1alpha1.NginxServer)
	if !ok {
		return nil
	}

	routeList := &nginxv1alpha1.NginxRouteList{}
	if err := r.List(ctx, routeList, client.InNamespace(server.Namespace)); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, route := range routeList.Items {
		if route.Spec.ServerRef == server.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      route.Name,
					Namespace: route.Namespace,
				},
			})
		}
	}

	return requests
}
