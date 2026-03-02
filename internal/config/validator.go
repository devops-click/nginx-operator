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

// Package config provides NGINX configuration generation and validation utilities.
package config

import (
	"fmt"
	"strings"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
)

// ValidationError represents a configuration validation error with context.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface for ValidationError.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %q: %s", e.Field, e.Message)
}

// ValidationResult holds the combined results of a validation pass.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []string
}

// ValidateNginxServer validates an NginxServer spec for correctness.
//
// Usage:
//
//	result := config.ValidateNginxServer(server)
//	if !result.Valid { /* handle errors */ }
func ValidateNginxServer(server *nginxv1alpha1.NginxServer) ValidationResult {
	result := ValidationResult{Valid: true}

	if server.Spec.Replicas != nil && *server.Spec.Replicas < 0 {
		result.addError("spec.replicas", "replicas must be >= 0")
	}

	if server.Spec.Image == "" {
		result.addError("spec.image", "image is required")
	}

	// Validate TLS
	if server.Spec.TLS != nil && server.Spec.TLS.Enabled {
		if server.Spec.TLS.SecretName == "" {
			result.addError("spec.tls.secretName", "secretName is required when TLS is enabled")
		}
	}

	// Validate autoscaling
	if server.Spec.Autoscaling != nil && server.Spec.Autoscaling.Enabled {
		if server.Spec.Autoscaling.MinReplicas != nil && server.Spec.Autoscaling.MaxReplicas > 0 {
			if *server.Spec.Autoscaling.MinReplicas > server.Spec.Autoscaling.MaxReplicas {
				result.addError("spec.autoscaling", "minReplicas must be <= maxReplicas")
			}
		}
	}

	// Validate PDB
	if server.Spec.PodDisruptionBudget != nil && server.Spec.PodDisruptionBudget.Enabled {
		if server.Spec.PodDisruptionBudget.MinAvailable != nil && server.Spec.PodDisruptionBudget.MaxUnavailable != nil {
			result.addError("spec.podDisruptionBudget", "cannot set both minAvailable and maxUnavailable")
		}
	}

	// Validate service ports
	if server.Spec.Service != nil {
		portNames := make(map[string]bool)
		for i, port := range server.Spec.Service.Ports {
			if port.Name == "" {
				result.addError(fmt.Sprintf("spec.service.ports[%d].name", i), "port name is required")
			}
			if portNames[port.Name] {
				result.addError(fmt.Sprintf("spec.service.ports[%d].name", i), "duplicate port name")
			}
			portNames[port.Name] = true
		}
	}

	// Warnings
	if server.Spec.Replicas != nil && *server.Spec.Replicas == 1 {
		result.Warnings = append(result.Warnings, "single replica provides no high availability")
	}

	if server.Spec.GlobalConfig != nil && server.Spec.GlobalConfig.ServerTokens {
		result.Warnings = append(result.Warnings, "server_tokens is enabled, exposing NGINX version in headers")
	}

	return result
}

// ValidateNginxRoute validates an NginxRoute spec for correctness.
//
// Usage:
//
//	result := config.ValidateNginxRoute(route)
func ValidateNginxRoute(route *nginxv1alpha1.NginxRoute) ValidationResult {
	result := ValidationResult{Valid: true}

	if route.Spec.ServerRef == "" {
		result.addError("spec.serverRef", "serverRef is required")
	}

	if route.Spec.ServerName == "" {
		result.addError("spec.serverName", "serverName is required")
	}

	if len(route.Spec.Locations) == 0 {
		result.addError("spec.locations", "at least one location is required")
	}

	// Validate each location
	for i, loc := range route.Spec.Locations {
		prefix := fmt.Sprintf("spec.locations[%d]", i)

		if loc.Path == "" {
			result.addError(prefix+".path", "path is required")
		}

		// Validate mutual exclusivity
		configCount := 0
		if loc.UpstreamRef != "" {
			configCount++
		}
		if loc.ProxyPass != "" {
			configCount++
		}
		if loc.StaticContent != nil {
			configCount++
		}
		if loc.Return != nil {
			configCount++
		}

		if configCount == 0 {
			result.addError(prefix, "one of upstreamRef, proxyPass, staticContent, or return must be specified")
		}
		if configCount > 1 {
			result.addError(prefix, "upstreamRef, proxyPass, staticContent, and return are mutually exclusive")
		}

		// Validate proxy settings only when proxying
		if loc.ProxySettings != nil && loc.UpstreamRef == "" && loc.ProxyPass == "" {
			result.addError(prefix+".proxySettings", "proxySettings only applies when upstreamRef or proxyPass is set")
		}
	}

	// Validate rate limit
	if route.Spec.RateLimit != nil && route.Spec.RateLimit.Enabled {
		if route.Spec.RateLimit.Rate == "" {
			result.addError("spec.rateLimit.rate", "rate is required when rate limiting is enabled")
		}
	}

	// Validate access control
	if route.Spec.AccessControl != nil {
		for i, cidr := range route.Spec.AccessControl.Allow {
			if !isValidCIDROrIP(cidr) {
				result.addError(fmt.Sprintf("spec.accessControl.allow[%d]", i), fmt.Sprintf("invalid CIDR or IP: %s", cidr))
			}
		}
		for i, cidr := range route.Spec.AccessControl.Deny {
			if !isValidCIDROrIP(cidr) {
				result.addError(fmt.Sprintf("spec.accessControl.deny[%d]", i), fmt.Sprintf("invalid CIDR or IP: %s", cidr))
			}
		}
	}

	return result
}

// ValidateNginxUpstream validates an NginxUpstream spec for correctness.
//
// Usage:
//
//	result := config.ValidateNginxUpstream(upstream)
func ValidateNginxUpstream(upstream *nginxv1alpha1.NginxUpstream) ValidationResult {
	result := ValidationResult{Valid: true}

	if upstream.Spec.ServerRef == "" {
		result.addError("spec.serverRef", "serverRef is required")
	}

	// Validate backends (required unless service discovery is enabled)
	if upstream.Spec.ServiceDiscovery == nil || !upstream.Spec.ServiceDiscovery.Enabled {
		if len(upstream.Spec.Backends) == 0 {
			result.addError("spec.backends", "at least one backend is required when service discovery is disabled")
		}
	}

	// Validate each backend
	for i, backend := range upstream.Spec.Backends {
		prefix := fmt.Sprintf("spec.backends[%d]", i)

		if backend.Address == "" {
			result.addError(prefix+".address", "address is required")
		}
		if backend.Port < 1 || backend.Port > 65535 {
			result.addError(prefix+".port", "port must be between 1 and 65535")
		}
	}

	// Validate service discovery
	if upstream.Spec.ServiceDiscovery != nil && upstream.Spec.ServiceDiscovery.Enabled {
		if upstream.Spec.ServiceDiscovery.ServiceName == "" {
			result.addError("spec.serviceDiscovery.serviceName", "serviceName is required")
		}
		if upstream.Spec.ServiceDiscovery.ServicePort < 1 || upstream.Spec.ServiceDiscovery.ServicePort > 65535 {
			result.addError("spec.serviceDiscovery.servicePort", "servicePort must be between 1 and 65535")
		}
	}

	// Validate load balancing
	if upstream.Spec.LoadBalancing != nil {
		validAlgorithms := map[string]bool{
			"round_robin": true,
			"least_conn":  true,
			"ip_hash":     true,
			"random":      true,
		}
		if !validAlgorithms[upstream.Spec.LoadBalancing.Algorithm] {
			result.addError("spec.loadBalancing.algorithm",
				fmt.Sprintf("invalid algorithm %q, must be one of: round_robin, least_conn, ip_hash, random",
					upstream.Spec.LoadBalancing.Algorithm))
		}
	}

	return result
}

// addError adds a validation error and marks the result as invalid.
func (r *ValidationResult) addError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
}

// ErrorMessages returns all validation error messages as a single string.
func (r *ValidationResult) ErrorMessages() string {
	if len(r.Errors) == 0 {
		return ""
	}
	var msgs []string
	for _, e := range r.Errors {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "; ")
}

// isValidCIDROrIP performs a basic check on whether a string looks like a valid
// CIDR notation or IP address. This is a simplified check — full validation
// happens when NGINX processes the config.
func isValidCIDROrIP(s string) bool {
	if s == "" {
		return false
	}
	// Allow "all" keyword
	if s == "all" {
		return true
	}
	// Basic format check for CIDR (x.x.x.x/y) or IP (x.x.x.x) or IPv6
	if strings.Contains(s, ":") || strings.Contains(s, ".") {
		return true
	}
	return false
}
