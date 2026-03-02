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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NginxServerSpec defines the desired state of an NGINX deployment instance.
// The operator will create and manage a Deployment, Service, ConfigMaps, and
// optionally a PodDisruptionBudget and HorizontalPodAutoscaler for this instance.
type NginxServerSpec struct {
	// Replicas is the desired number of NGINX pod replicas.
	// Ignored when autoscaling is enabled.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Image specifies the NGINX container image to use.
	// +kubebuilder:default="nginx:1.27-alpine"
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy defines the pull policy for the NGINX image.
	// +kubebuilder:default="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// ImagePullSecrets is a list of references to secrets for pulling the NGINX image.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Resources defines CPU/memory resource requests and limits for the NGINX container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// ReloaderResources defines CPU/memory resource requests and limits for the config reloader sidecar.
	// +optional
	ReloaderResources corev1.ResourceRequirements `json:"reloaderResources,omitempty"`

	// Service defines the Service configuration for exposing NGINX.
	// +optional
	Service *NginxServiceSpec `json:"service,omitempty"`

	// TLS defines global TLS settings for this NGINX instance.
	// +optional
	TLS *NginxTLSSpec `json:"tls,omitempty"`

	// GlobalConfig provides global NGINX directives applied to the main nginx.conf context.
	// +optional
	GlobalConfig *NginxGlobalConfig `json:"globalConfig,omitempty"`

	// Monitoring defines Prometheus metrics exposure settings.
	// +optional
	Monitoring *NginxMonitoringSpec `json:"monitoring,omitempty"`

	// Autoscaling defines HorizontalPodAutoscaler settings.
	// When enabled, the replicas field is ignored.
	// +optional
	Autoscaling *NginxAutoscalingSpec `json:"autoscaling,omitempty"`

	// PodDisruptionBudget defines PDB settings for high availability.
	// +optional
	PodDisruptionBudget *NginxPDBSpec `json:"podDisruptionBudget,omitempty"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations are applied to the NGINX pods for scheduling.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity defines scheduling affinity rules for the NGINX pods.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// TopologySpreadConstraints describes how pods should spread across topology domains.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// ExtraVolumes allows mounting additional volumes into the NGINX pods.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts allows mounting additional volume mounts into the NGINX container.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// ExtraEnvVars allows setting additional environment variables on the NGINX container.
	// +optional
	ExtraEnvVars []corev1.EnvVar `json:"extraEnvVars,omitempty"`

	// PodAnnotations are additional annotations to set on the NGINX pods.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// PodLabels are additional labels to set on the NGINX pods.
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// SecurityContext defines the security context for the NGINX pods.
	// +optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`

	// ContainerSecurityContext defines the security context for the NGINX container.
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
}

// NginxServiceSpec defines the Service configuration.
type NginxServiceSpec struct {
	// Type is the Kubernetes Service type.
	// +kubebuilder:default="ClusterIP"
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Ports defines the ports exposed by the Service.
	// +optional
	Ports []NginxServicePort `json:"ports,omitempty"`

	// Annotations are additional annotations for the Service.
	// Useful for cloud provider load balancer configuration.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// LoadBalancerIP specifies a fixed IP for LoadBalancer-type services.
	// +optional
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`

	// ExternalTrafficPolicy specifies whether to route external traffic to node-local or cluster-wide endpoints.
	// +kubebuilder:validation:Enum=Cluster;Local
	// +optional
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`
}

// NginxServicePort defines a port on the NGINX Service.
type NginxServicePort struct {
	// Name is the name of the port.
	Name string `json:"name"`

	// Port is the port number exposed by the Service.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// TargetPort is the port on the NGINX container to route traffic to.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	TargetPort int32 `json:"targetPort,omitempty"`

	// Protocol is the protocol for this port (TCP or UDP).
	// +kubebuilder:default="TCP"
	// +kubebuilder:validation:Enum=TCP;UDP
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`
}

// NginxTLSSpec defines TLS settings for the NGINX instance.
type NginxTLSSpec struct {
	// Enabled enables TLS on the NGINX instance.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SecretName references a Kubernetes Secret containing the TLS certificate and key.
	// The secret must contain tls.crt and tls.key entries.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// Protocols defines allowed TLS protocols.
	// +kubebuilder:default={"TLSv1.2","TLSv1.3"}
	// +optional
	Protocols []string `json:"protocols,omitempty"`

	// Ciphers defines the allowed TLS cipher suites.
	// +optional
	Ciphers string `json:"ciphers,omitempty"`

	// PreferServerCiphers enables server cipher preference.
	// +kubebuilder:default=true
	// +optional
	PreferServerCiphers bool `json:"preferServerCiphers,omitempty"`

	// SessionCache configures TLS session caching.
	// +kubebuilder:default="shared:SSL:10m"
	// +optional
	SessionCache string `json:"sessionCache,omitempty"`

	// SessionTimeout defines TLS session timeout.
	// +kubebuilder:default="1d"
	// +optional
	SessionTimeout string `json:"sessionTimeout,omitempty"`
}

// NginxGlobalConfig provides global NGINX directives.
type NginxGlobalConfig struct {
	// WorkerProcesses sets the number of NGINX worker processes.
	// Use "auto" to match CPU cores.
	// +kubebuilder:default="auto"
	// +optional
	WorkerProcesses string `json:"workerProcesses,omitempty"`

	// WorkerConnections sets the maximum number of simultaneous connections per worker.
	// +kubebuilder:default=1024
	// +kubebuilder:validation:Minimum=128
	// +kubebuilder:validation:Maximum=65535
	// +optional
	WorkerConnections int32 `json:"workerConnections,omitempty"`

	// KeepaliveTimeout defines the timeout for keep-alive connections.
	// +kubebuilder:default="65s"
	// +optional
	KeepaliveTimeout string `json:"keepaliveTimeout,omitempty"`

	// KeepaliveRequests sets the maximum number of requests per keep-alive connection.
	// +kubebuilder:default=100
	// +optional
	KeepaliveRequests int32 `json:"keepaliveRequests,omitempty"`

	// ClientMaxBodySize sets the maximum allowed size of the client request body.
	// +kubebuilder:default="1m"
	// +optional
	ClientMaxBodySize string `json:"clientMaxBodySize,omitempty"`

	// ServerTokens controls whether NGINX version is shown in error pages and headers.
	// +kubebuilder:default=false
	// +optional
	ServerTokens bool `json:"serverTokens,omitempty"`

	// ErrorLogLevel sets the error log verbosity level.
	// +kubebuilder:default="warn"
	// +kubebuilder:validation:Enum=debug;info;notice;warn;error;crit;alert;emerg
	// +optional
	ErrorLogLevel string `json:"errorLogLevel,omitempty"`

	// AccessLogFormat defines the format string for access logs.
	// Leave empty to use the default combined format.
	// +optional
	AccessLogFormat string `json:"accessLogFormat,omitempty"`

	// AccessLogEnabled controls whether access logging is enabled.
	// +kubebuilder:default=true
	// +optional
	AccessLogEnabled bool `json:"accessLogEnabled,omitempty"`

	// GzipEnabled enables gzip compression.
	// +kubebuilder:default=true
	// +optional
	GzipEnabled bool `json:"gzipEnabled,omitempty"`

	// GzipTypes defines MIME types to compress.
	// +kubebuilder:default={"text/plain","text/css","application/json","application/javascript","text/xml","application/xml","image/svg+xml"}
	// +optional
	GzipTypes []string `json:"gzipTypes,omitempty"`

	// GzipMinLength sets the minimum response length for gzip compression.
	// +kubebuilder:default=256
	// +optional
	GzipMinLength int32 `json:"gzipMinLength,omitempty"`

	// CustomMainSnippet allows injecting raw NGINX directives into the main context.
	// Use with caution — no validation is performed on custom snippets.
	// +optional
	CustomMainSnippet string `json:"customMainSnippet,omitempty"`

	// CustomHTTPSnippet allows injecting raw NGINX directives into the http context.
	// +optional
	CustomHTTPSnippet string `json:"customHTTPSnippet,omitempty"`
}

// NginxMonitoringSpec defines Prometheus monitoring settings.
type NginxMonitoringSpec struct {
	// Enabled enables the NGINX stub_status module and Prometheus metrics endpoint.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Port is the port for the metrics endpoint.
	// +kubebuilder:default=9113
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// Path is the HTTP path for the metrics endpoint.
	// +kubebuilder:default="/metrics"
	// +optional
	Path string `json:"path,omitempty"`

	// ServiceMonitor enables creating a Prometheus ServiceMonitor resource.
	// +kubebuilder:default=false
	// +optional
	ServiceMonitor bool `json:"serviceMonitor,omitempty"`
}

// NginxAutoscalingSpec defines HPA settings.
type NginxAutoscalingSpec struct {
	// Enabled enables horizontal pod autoscaling.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// MinReplicas is the minimum number of replicas.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas.
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// TargetCPUUtilizationPercentage is the target average CPU utilization.
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// TargetMemoryUtilizationPercentage is the target average memory utilization.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +optional
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

// NginxPDBSpec defines PodDisruptionBudget settings.
type NginxPDBSpec struct {
	// Enabled enables PodDisruptionBudget creation.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// MinAvailable is the minimum number of pods that must be available.
	// Cannot be set together with MaxUnavailable.
	// +optional
	MinAvailable *int32 `json:"minAvailable,omitempty"`

	// MaxUnavailable is the maximum number of pods that can be unavailable.
	// Cannot be set together with MinAvailable.
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

// NginxServerStatus defines the observed state of NginxServer.
type NginxServerStatus struct {
	// Conditions represent the latest available observations of the NginxServer's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ReadyReplicas is the number of NGINX pods that are ready.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available NGINX pods.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ConfigHash is the SHA-256 hash of the current applied NGINX configuration.
	// +optional
	ConfigHash string `json:"configHash,omitempty"`

	// LastReloadTime is the timestamp of the last successful NGINX configuration reload.
	// +optional
	LastReloadTime *metav1.Time `json:"lastReloadTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// RouteCount is the number of NginxRoute resources associated with this server.
	// +optional
	RouteCount int32 `json:"routeCount,omitempty"`

	// UpstreamCount is the number of NginxUpstream resources associated with this server.
	// +optional
	UpstreamCount int32 `json:"upstreamCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ns;nxs
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Routes",type="integer",JSONPath=".status.routeCount"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NginxServer is the Schema for the nginxservers API.
// It represents a managed NGINX deployment instance in the cluster.
// The operator creates and manages a Deployment, Service, ConfigMaps,
// and optionally a PodDisruptionBudget and HorizontalPodAutoscaler.
type NginxServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxServerSpec   `json:"spec,omitempty"`
	Status NginxServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NginxServerList contains a list of NginxServer resources.
type NginxServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxServer{}, &NginxServerList{})
}
