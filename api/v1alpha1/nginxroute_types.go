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

// NginxRouteSpec defines the desired state of an NGINX virtual host (server block).
// Each NginxRoute maps to one or more NGINX server {} blocks and must reference
// an NginxServer instance that will serve this route configuration.
type NginxRouteSpec struct {
	// ServerRef is the name of the NginxServer resource this route belongs to.
	// The NginxServer must exist in the same namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServerRef string `json:"serverRef"`

	// ServerName defines the server_name directive (e.g., "example.com", "*.example.com").
	// Multiple hostnames can be specified as a space-separated string.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServerName string `json:"serverName"`

	// Listen defines the listen directive configuration.
	// +optional
	Listen *NginxListenSpec `json:"listen,omitempty"`

	// TLS defines per-route TLS settings. Overrides the NginxServer global TLS if set.
	// +optional
	TLS *NginxRouteTLSSpec `json:"tls,omitempty"`

	// Locations defines the location blocks within this server block.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Locations []NginxLocationSpec `json:"locations"`

	// RateLimit defines rate limiting settings for this virtual host.
	// +optional
	RateLimit *NginxRateLimitSpec `json:"rateLimit,omitempty"`

	// AccessControl defines IP-based access control for this virtual host.
	// +optional
	AccessControl *NginxAccessControlSpec `json:"accessControl,omitempty"`

	// Headers defines custom HTTP headers to add or remove.
	// +optional
	Headers *NginxHeadersSpec `json:"headers,omitempty"`

	// CORS defines Cross-Origin Resource Sharing settings.
	// +optional
	CORS *NginxCORSSpec `json:"cors,omitempty"`

	// CustomServerSnippet allows injecting raw NGINX directives into the server block.
	// Use with caution — no validation is performed on custom snippets.
	// +optional
	CustomServerSnippet string `json:"customServerSnippet,omitempty"`

	// Priority determines the order of server blocks in the NGINX configuration.
	// Lower values are processed first. Default is 100.
	// +kubebuilder:default=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=9999
	// +optional
	Priority int32 `json:"priority,omitempty"`
}

// NginxListenSpec defines the listen directive.
type NginxListenSpec struct {
	// Port is the port to listen on for HTTP traffic.
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// HTTPSPort is the port to listen on for HTTPS traffic (when TLS is enabled).
	// +kubebuilder:default=443
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	HTTPSPort int32 `json:"httpsPort,omitempty"`

	// ProxyProtocol enables PROXY protocol support on the listen directive.
	// +kubebuilder:default=false
	// +optional
	ProxyProtocol bool `json:"proxyProtocol,omitempty"`
}

// NginxRouteTLSSpec defines per-route TLS settings.
type NginxRouteTLSSpec struct {
	// Enabled enables TLS for this route.
	Enabled bool `json:"enabled"`

	// SecretName references a Kubernetes TLS Secret for this route.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// RedirectHTTP automatically redirects HTTP to HTTPS when true.
	// +kubebuilder:default=true
	// +optional
	RedirectHTTP bool `json:"redirectHTTP,omitempty"`
}

// NginxLocationSpec defines a location block within a server block.
type NginxLocationSpec struct {
	// Path is the location path (e.g., "/", "/api", "~ \.php$").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`

	// UpstreamRef references an NginxUpstream resource by name for proxy_pass.
	// Mutually exclusive with StaticContent and Return.
	// +optional
	UpstreamRef string `json:"upstreamRef,omitempty"`

	// ProxyPass sets a direct proxy_pass URL (alternative to UpstreamRef).
	// Mutually exclusive with UpstreamRef, StaticContent, and Return.
	// +optional
	ProxyPass string `json:"proxyPass,omitempty"`

	// StaticContent serves static files from the specified root directory.
	// Mutually exclusive with UpstreamRef, ProxyPass, and Return.
	// +optional
	StaticContent *NginxStaticContentSpec `json:"staticContent,omitempty"`

	// Return sends a fixed response (e.g., redirect, error page).
	// Mutually exclusive with UpstreamRef, ProxyPass, and StaticContent.
	// +optional
	Return *NginxReturnSpec `json:"return,omitempty"`

	// ProxySettings defines proxy_* directives for this location.
	// Only applies when UpstreamRef or ProxyPass is set.
	// +optional
	ProxySettings *NginxProxySettingsSpec `json:"proxySettings,omitempty"`

	// RateLimit defines per-location rate limiting (overrides server-level).
	// +optional
	RateLimit *NginxRateLimitSpec `json:"rateLimit,omitempty"`

	// Headers defines per-location custom headers.
	// +optional
	Headers *NginxHeadersSpec `json:"headers,omitempty"`

	// CustomLocationSnippet allows injecting raw NGINX directives into this location block.
	// +optional
	CustomLocationSnippet string `json:"customLocationSnippet,omitempty"`
}

// NginxStaticContentSpec defines static file serving configuration.
type NginxStaticContentSpec struct {
	// Root is the root directory for serving static files.
	// +kubebuilder:validation:Required
	Root string `json:"root"`

	// Index defines index file names.
	// +kubebuilder:default={"index.html"}
	// +optional
	Index []string `json:"index,omitempty"`

	// TryFiles defines the try_files directive.
	// +optional
	TryFiles string `json:"tryFiles,omitempty"`

	// Autoindex enables directory listing.
	// +kubebuilder:default=false
	// +optional
	Autoindex bool `json:"autoindex,omitempty"`
}

// NginxReturnSpec defines a fixed return response.
type NginxReturnSpec struct {
	// Code is the HTTP status code to return.
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=599
	Code int32 `json:"code"`

	// Body is the response body or redirect URL.
	// +optional
	Body string `json:"body,omitempty"`
}

// NginxProxySettingsSpec defines proxy_* directives.
type NginxProxySettingsSpec struct {
	// ConnectTimeout is the timeout for establishing a connection to the upstream.
	// +kubebuilder:default="60s"
	// +optional
	ConnectTimeout string `json:"connectTimeout,omitempty"`

	// SendTimeout is the timeout for transmitting a request to the upstream.
	// +kubebuilder:default="60s"
	// +optional
	SendTimeout string `json:"sendTimeout,omitempty"`

	// ReadTimeout is the timeout for reading a response from the upstream.
	// +kubebuilder:default="60s"
	// +optional
	ReadTimeout string `json:"readTimeout,omitempty"`

	// BufferSize sets the proxy_buffer_size directive.
	// +kubebuilder:default="4k"
	// +optional
	BufferSize string `json:"bufferSize,omitempty"`

	// Buffers sets the proxy_buffers directive (number and size).
	// +kubebuilder:default="8 4k"
	// +optional
	Buffers string `json:"buffers,omitempty"`

	// SetHeaders defines headers to pass to the upstream.
	// +optional
	SetHeaders map[string]string `json:"setHeaders,omitempty"`

	// WebSocket enables WebSocket proxying (adds Upgrade and Connection headers).
	// +kubebuilder:default=false
	// +optional
	WebSocket bool `json:"webSocket,omitempty"`

	// NextUpstream defines conditions under which the request is passed to the next upstream server.
	// +kubebuilder:default="error timeout"
	// +optional
	NextUpstream string `json:"nextUpstream,omitempty"`

	// NextUpstreamTries limits the number of possible tries for passing a request to the next server.
	// +kubebuilder:default=3
	// +optional
	NextUpstreamTries int32 `json:"nextUpstreamTries,omitempty"`
}

// NginxRateLimitSpec defines rate limiting configuration.
type NginxRateLimitSpec struct {
	// Enabled enables rate limiting.
	Enabled bool `json:"enabled"`

	// Zone defines the shared memory zone name and size (e.g., "10m").
	// +kubebuilder:default="10m"
	// +optional
	Zone string `json:"zone,omitempty"`

	// Rate defines the request rate limit (e.g., "10r/s", "100r/m").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\d+r/[sm]$`
	Rate string `json:"rate"`

	// Burst allows bursting above the rate limit up to this many requests.
	// +kubebuilder:default=20
	// +optional
	Burst int32 `json:"burst,omitempty"`

	// NoDelay processes burst requests without delay.
	// +kubebuilder:default=true
	// +optional
	NoDelay bool `json:"noDelay,omitempty"`

	// Key defines what the rate limit is keyed on.
	// +kubebuilder:default="$binary_remote_addr"
	// +optional
	Key string `json:"key,omitempty"`
}

// NginxAccessControlSpec defines IP-based access control.
type NginxAccessControlSpec struct {
	// Allow is a list of CIDR blocks to allow access.
	// +optional
	Allow []string `json:"allow,omitempty"`

	// Deny is a list of CIDR blocks to deny access.
	// +optional
	Deny []string `json:"deny,omitempty"`
}

// NginxHeadersSpec defines custom HTTP headers.
type NginxHeadersSpec struct {
	// Add defines headers to add to responses.
	// +optional
	Add map[string]string `json:"add,omitempty"`

	// Remove defines headers to remove from responses.
	// +optional
	Remove []string `json:"remove,omitempty"`

	// SecurityHeaders adds common security headers (X-Frame-Options, X-Content-Type-Options, etc.).
	// +kubebuilder:default=true
	// +optional
	SecurityHeaders bool `json:"securityHeaders,omitempty"`
}

// NginxCORSSpec defines Cross-Origin Resource Sharing settings.
type NginxCORSSpec struct {
	// Enabled enables CORS handling.
	Enabled bool `json:"enabled"`

	// AllowOrigins defines allowed origins. Use "*" for any origin.
	// +optional
	AllowOrigins []string `json:"allowOrigins,omitempty"`

	// AllowMethods defines allowed HTTP methods.
	// +kubebuilder:default={"GET","POST","PUT","DELETE","OPTIONS"}
	// +optional
	AllowMethods []string `json:"allowMethods,omitempty"`

	// AllowHeaders defines allowed request headers.
	// +kubebuilder:default={"Content-Type","Authorization"}
	// +optional
	AllowHeaders []string `json:"allowHeaders,omitempty"`

	// ExposeHeaders defines response headers exposed to the browser.
	// +optional
	ExposeHeaders []string `json:"exposeHeaders,omitempty"`

	// MaxAge defines how long preflight results can be cached (in seconds).
	// +kubebuilder:default=86400
	// +optional
	MaxAge int32 `json:"maxAge,omitempty"`

	// AllowCredentials indicates whether credentials are supported.
	// +kubebuilder:default=false
	// +optional
	AllowCredentials bool `json:"allowCredentials,omitempty"`
}

// NginxRouteStatus defines the observed state of NginxRoute.
type NginxRouteStatus struct {
	// Conditions represent the latest available observations of the NginxRoute's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ConfigHash is the SHA-256 hash of the generated config for this route.
	// +optional
	ConfigHash string `json:"configHash,omitempty"`

	// LastAppliedTime is the timestamp when the config was last applied.
	// +optional
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=nr;nxr
// +kubebuilder:printcolumn:name="Server",type="string",JSONPath=".spec.serverRef"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".spec.serverName"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NginxRoute is the Schema for the nginxroutes API.
// It represents a virtual host / server block configuration that is applied
// to a referenced NginxServer instance.
type NginxRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxRouteSpec   `json:"spec,omitempty"`
	Status NginxRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NginxRouteList contains a list of NginxRoute resources.
type NginxRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NginxRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NginxRoute{}, &NginxRouteList{})
}
