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

package v1alpha1

// Condition type constants used across all CRDs for consistent status reporting.
const (
	// ConditionReady indicates the resource is fully reconciled and operational.
	ConditionReady = "Ready"

	// ConditionConfigValid indicates the NGINX configuration passed validation (nginx -t).
	ConditionConfigValid = "ConfigValid"

	// ConditionDeploymentReady indicates the managed NGINX Deployment has all replicas available.
	ConditionDeploymentReady = "DeploymentReady"

	// ConditionServiceReady indicates the managed Service is created and configured.
	ConditionServiceReady = "ServiceReady"

	// ConditionConfigApplied indicates the generated config has been applied to the ConfigMap.
	ConditionConfigApplied = "ConfigApplied"

	// ConditionDegraded indicates the resource is operational but with reduced capability.
	ConditionDegraded = "Degraded"
)

// Condition reason constants provide machine-readable reasons for condition transitions.
const (
	// ReasonReconciling indicates the resource is being reconciled.
	ReasonReconciling = "Reconciling"

	// ReasonReconciled indicates the resource was successfully reconciled.
	ReasonReconciled = "Reconciled"

	// ReasonFailed indicates a reconciliation failure.
	ReasonFailed = "Failed"

	// ReasonConfigInvalid indicates the generated NGINX config failed validation.
	ReasonConfigInvalid = "ConfigInvalid"

	// ReasonConfigGenerated indicates config was successfully generated.
	ReasonConfigGenerated = "ConfigGenerated"

	// ReasonConfigApplied indicates config was applied to the target ConfigMap.
	ReasonConfigApplied = "ConfigApplied"

	// ReasonDeploymentNotReady indicates the Deployment does not have desired replicas.
	ReasonDeploymentNotReady = "DeploymentNotReady"

	// ReasonDeploymentReady indicates all desired replicas are available.
	ReasonDeploymentReady = "DeploymentReady"

	// ReasonServerNotFound indicates the referenced NginxServer was not found.
	ReasonServerNotFound = "ServerNotFound"

	// ReasonDependencyNotReady indicates a dependent resource is not ready.
	ReasonDependencyNotReady = "DependencyNotReady"

	// ReasonFinalizerFailed indicates finalizer cleanup failed.
	ReasonFinalizerFailed = "FinalizerFailed"
)

// Finalizer names used by the operator for cleanup.
const (
	// NginxServerFinalizer is applied to NginxServer resources to ensure cleanup.
	NginxServerFinalizer = "nginx.devops.click/server-finalizer"

	// NginxRouteFinalizer is applied to NginxRoute resources to ensure config removal.
	NginxRouteFinalizer = "nginx.devops.click/route-finalizer"

	// NginxUpstreamFinalizer is applied to NginxUpstream resources to ensure config removal.
	NginxUpstreamFinalizer = "nginx.devops.click/upstream-finalizer"
)

// Annotation keys used by the operator.
const (
	// AnnotationConfigHash stores the SHA-256 hash of the current NGINX configuration.
	AnnotationConfigHash = "nginx.devops.click/config-hash"

	// AnnotationLastReload stores the timestamp of the last successful NGINX reload.
	AnnotationLastReload = "nginx.devops.click/last-reload"

	// AnnotationTargetInstance specifies which operator instance should handle this resource.
	AnnotationTargetInstance = "nginx.devops.click/target-instance"
)

// Label keys used by the operator to identify managed resources.
const (
	// LabelManagedBy identifies resources managed by this operator.
	LabelManagedBy = "app.kubernetes.io/managed-by"

	// LabelManagedByValue is the value for the managed-by label.
	LabelManagedByValue = "nginx-operator"

	// LabelInstance identifies the NginxServer instance name.
	LabelInstance = "app.kubernetes.io/instance"

	// LabelComponent identifies the component type (e.g., "nginx", "reloader").
	LabelComponent = "app.kubernetes.io/component"

	// LabelPartOf identifies the application this is part of.
	LabelPartOf = "app.kubernetes.io/part-of"
)
