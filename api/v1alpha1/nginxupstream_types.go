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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NginxUpstreamSpec defines the desired state of an NGINX upstream block.
// Each NginxUpstream maps to an NGINX upstream {} block and must reference
// an NginxServer instance that will include this upstream configuration.
type NginxUpstreamSpec struct {
	// ServerRef is the name of the NginxServer resource this upstream belongs to.
	// The NginxServer must exist in the same namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`

	// Backends defines the list of upstream backend servers.
	// At least one backend must be specified.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Backends []NginxBackendSpec `json:"backends"`

	// LoadBalancing defines the load balancing algorithm.
	// +optional
	LoadBalancing *NginxLoadBalancingSpec `json:"loadBalancing,omitempty"`

	// HealthCheck defines active health checking for upstream servers.
	// +optional
	HealthCheck *NginxHealthCheckSpec `json:"healthCheck,omitempty"`

	// Keepalive sets the maximum number of idle keepalive connections to upstream servers.
	// +kubebuilder:default=32
	// +kubebuilder:validation:Minimum=0
	// +optional
	Keepalive int32 `json:"keepalive,omitempty"`

	// KeepaliveTimeout is the timeout during which an idle keepalive connection will stay open.
	// +kubebuilder:default="60s"
	// +optional
	KeepaliveTimeout string `json:"keepaliveTimeout,omitempty"`

	// KeepaliveRequests sets the maximum number of requests through one keepalive connection.
	// +kubebuilder:default=100
	// +optional
	KeepaliveRequests int32 `json:"keepaliveRequests,omitempty"`

	// ServiceDiscovery enables automatic backend discovery from a Kubernetes Service.
	// When enabled, Backends field is ignored and endpoints are auto-populated.
	// +optional
	ServiceDiscovery *NginxServiceDiscoverySpec `json:"serviceDiscovery,omitempty"`

	// CustomUpstreamSnippet allows injecting raw NGINX directives into the upstream block.
	// +optional
	CustomUpstreamSnippet string `json:"customUpstreamSnippet,omitempty"`
}

// NginxBackendSpec defines an upstream backend server.
type NginxBackendSpec struct {
	// Address is the backend server address (IP or hostname).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`

	// Port is the backend server port.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Weight sets the weight for weighted load balancing.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	Weight int32 `json:"weight,omitempty"`

	// MaxConnections limits the maximum number of simultaneous active connections.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxConnections int32 `json:"maxConnections,omitempty"`

	// MaxFails sets the number of unsuccessful attempts before marking the server as unavailable.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxFails int32 `json:"maxFails,omitempty"`

	// FailTimeout sets the time during which the specified number of unsuccessful attempts
	// should happen and the time the server is considered unavailable.
	// +kubebuilder:default="10s"
	// +optional
	FailTimeout string `json:"failTimeout,omitempty"`

	// Backup marks this server as a backup server.
	// It receives requests only when all primary servers are unavailable.
	// +kubebuilder:default=false
	// +optional
	Backup bool `json:"backup,omitempty"`

	// Down marks this server as permanently unavailable.
	// +kubebuilder:default=false
	// +optional
	Down bool `json:"down,omitempty"`
}

// NginxLoadBalancingSpec defines the load balancing algorithm.
type NginxLoadBalancingSpec struct {
	// Algorithm defines the load balancing method.
	// - round_robin: Default, distributes requests evenly.
	// - least_conn: Sends to the server with the fewest active connections.
	// - ip_hash: Ensures requests from the same IP go to the same server.
	// - random: Selects a random server (with optional two-choice algorithm).
	// +kubebuilder:default="round_robin"
	// +kubebuilder:validation:Enum=round_robin;least_conn;ip_hash;random
	// +optional
	Algorithm string `json:"algorithm,omitempty"`

	// RandomTwoChoices enables the "two choices" variant for the random algorithm.
	// When enabled, picks two servers randomly and selects one using least_conn.
	// Only applies when Algorithm is "random".
	// +kubebuilder:default=false
	// +optional
	RandomTwoChoices bool `json:"randomTwoChoices,omitempty"`
}

// NginxHealthCheckSpec defines active health checking.
type NginxHealthCheckSpec struct {
	// Enabled enables active health checking for upstream servers.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Path is the URI to request for health checks (HTTP only).
	// +kubebuilder:default="/"
	// +optional
	Path string `json:"path,omitempty"`

	// Interval defines how often health checks are performed.
	// +kubebuilder:default="30s"
	// +optional
	Interval string `json:"interval,omitempty"`

	// Timeout defines the health check request timeout.
	// +kubebuilder:default="5s"
	// +optional
	Timeout string `json:"timeout,omitempty"`

	// HealthyThreshold is the number of consecutive successes before marking healthy.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +optional
	HealthyThreshold int32 `json:"healthyThreshold,omitempty"`

	// UnhealthyThreshold is the number of consecutive failures before marking unhealthy.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +optional
	UnhealthyThreshold int32 `json:"unhealthyThreshold,omitempty"`

	// ExpectedStatus defines the expected HTTP status code range for a healthy response.
	// +kubebuilder:default=200
	// +optional
	ExpectedStatus int32 `json:"expectedStatus,omitempty"`
}

// NginxServiceDiscoverySpec defines automatic backend discovery from a Kubernetes Service.
type NginxServiceDiscoverySpec struct {
	// Enabled enables service discovery.
	Enabled bool `json:"enabled"`

	// ServiceName is the name of the Kubernetes Service to discover endpoints from.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServiceName string `json:"serviceName"`

	// ServicePort is the port on the Service to use for upstream backends.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ServicePort int32 `json:"servicePort"`

	// Namespace is the namespace of the Service. Defaults to the NginxUpstream's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// NginxUpstreamStatus defines the observed state of NginxUpstream.
type NginxUpstreamStatus struct {
	// Conditions represent the latest available observations of the NginxUpstream's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ActiveBackends is the number of backends currently active (not down or failed).
	// +optional
	ActiveBackends int32 `json:"activeBackends,omitempty"`

	// TotalBackends is the total number of configured backends.
	// +optional
	TotalBackends int32 `json:"totalBackends,omitempty"`

	// ConfigHash is the SHA-256 hash of the generated upstream config.
	// +optional
	ConfigHash string `json:"configHash,omitempty"`

	// LastAppliedTime is the timestamp when the config was last applied.
	// +optional
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DiscoveredEndpoints lists the endpoints discovered via service discovery.
	// Only populated when ServiceDiscovery is enabled.
	// +optional
	DiscoveredEndpoints []string `json:"discoveredEndpoints,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nu;nxu
// +kubebuilder:printcolumn:name="Server",type="string",JSONPath=".spec.serverRef"
// +kubebuilder:printcolumn:name="Backends",type="integer",JSONPath=".status.activeBackends"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NginxUpstream is the Schema for the nginxupstreams API.
// It represents an NGINX upstream {} block configuration that defines
// backend servers for proxying traffic.
type NginxUpstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxUpstreamSpec   `json:"spec,omitempty"`
	Status NginxUpstreamStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NginxUpstreamList contains a list of NginxUpstream resources.
type NginxUpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxUpstream `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxUpstream{}, &NginxUpstreamList{})
}
