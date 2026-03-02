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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
)

// TestValidateNginxServer_Valid verifies a valid NginxServer passes validation.
func TestValidateNginxServer_Valid(t *testing.T) {
	replicas := int32(2)
	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxServerSpec{
			Replicas: &replicas,
			Image:    "nginx:1.27-alpine",
		},
	}

	result := ValidateNginxServer(server)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

// TestValidateNginxServer_EmptyImage verifies validation catches empty image.
func TestValidateNginxServer_EmptyImage(t *testing.T) {
	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxServerSpec{
			Image: "",
		},
	}

	result := ValidateNginxServer(server)
	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "spec.image", result.Errors[0].Field)
}

// TestValidateNginxServer_TLSWithoutSecret verifies TLS requires secretName.
func TestValidateNginxServer_TLSWithoutSecret(t *testing.T) {
	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxServerSpec{
			Image: "nginx:latest",
			TLS: &nginxv1alpha1.NginxTLSSpec{
				Enabled:    true,
				SecretName: "",
			},
		},
	}

	result := ValidateNginxServer(server)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "secretName")
}

// TestValidateNginxServer_AutoscalingMinMax verifies autoscaling min <= max.
func TestValidateNginxServer_AutoscalingMinMax(t *testing.T) {
	min := int32(10)
	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxServerSpec{
			Image: "nginx:latest",
			Autoscaling: &nginxv1alpha1.NginxAutoscalingSpec{
				Enabled:     true,
				MinReplicas: &min,
				MaxReplicas: 5,
			},
		},
	}

	result := ValidateNginxServer(server)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "minReplicas must be <= maxReplicas")
}

// TestValidateNginxServer_PDBConflict verifies PDB mutual exclusivity.
func TestValidateNginxServer_PDBConflict(t *testing.T) {
	minAvail := int32(1)
	maxUnavail := int32(1)
	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxServerSpec{
			Image: "nginx:latest",
			PodDisruptionBudget: &nginxv1alpha1.NginxPDBSpec{
				Enabled:        true,
				MinAvailable:   &minAvail,
				MaxUnavailable: &maxUnavail,
			},
		},
	}

	result := ValidateNginxServer(server)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "cannot set both")
}

// TestValidateNginxServer_SingleReplicaWarning verifies HA warning.
func TestValidateNginxServer_SingleReplicaWarning(t *testing.T) {
	replicas := int32(1)
	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxServerSpec{
			Replicas: &replicas,
			Image:    "nginx:latest",
		},
	}

	result := ValidateNginxServer(server)
	assert.True(t, result.Valid)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "high availability")
}

// TestValidateNginxRoute_Valid verifies a valid NginxRoute passes validation.
func TestValidateNginxRoute_Valid(t *testing.T) {
	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "my-server",
			ServerName: "example.com",
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{Path: "/", ProxyPass: "http://backend:8080"},
			},
		},
	}

	result := ValidateNginxRoute(route)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

// TestValidateNginxRoute_MissingServerRef verifies serverRef is required.
func TestValidateNginxRoute_MissingServerRef(t *testing.T) {
	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerName: "example.com",
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{Path: "/", ProxyPass: "http://backend:8080"},
			},
		},
	}

	result := ValidateNginxRoute(route)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "serverRef")
}

// TestValidateNginxRoute_NoLocations verifies at least one location required.
func TestValidateNginxRoute_NoLocations(t *testing.T) {
	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "my-server",
			ServerName: "example.com",
			Locations:  []nginxv1alpha1.NginxLocationSpec{},
		},
	}

	result := ValidateNginxRoute(route)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "at least one location")
}

// TestValidateNginxRoute_MutuallyExclusiveLocationTypes verifies location type exclusivity.
func TestValidateNginxRoute_MutuallyExclusiveLocationTypes(t *testing.T) {
	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "my-server",
			ServerName: "example.com",
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{
					Path:        "/",
					ProxyPass:   "http://backend:8080",
					UpstreamRef: "some-upstream",
				},
			},
		},
	}

	result := ValidateNginxRoute(route)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "mutually exclusive")
}

// TestValidateNginxRoute_EmptyLocation verifies location requires a handler.
func TestValidateNginxRoute_EmptyLocation(t *testing.T) {
	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "my-server",
			ServerName: "example.com",
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{Path: "/"},
			},
		},
	}

	result := ValidateNginxRoute(route)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "one of upstreamRef")
}

// TestValidateNginxUpstream_Valid verifies a valid NginxUpstream passes validation.
func TestValidateNginxUpstream_Valid(t *testing.T) {
	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "my-server",
			Backends: []nginxv1alpha1.NginxBackendSpec{
				{Address: "10.0.0.1", Port: 8080},
			},
		},
	}

	result := ValidateNginxUpstream(upstream)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

// TestValidateNginxUpstream_NoBackendsNoServiceDiscovery verifies backends required.
func TestValidateNginxUpstream_NoBackendsNoServiceDiscovery(t *testing.T) {
	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "my-server",
			Backends:  []nginxv1alpha1.NginxBackendSpec{},
		},
	}

	result := ValidateNginxUpstream(upstream)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "at least one backend")
}

// TestValidateNginxUpstream_InvalidPort verifies port range validation.
func TestValidateNginxUpstream_InvalidPort(t *testing.T) {
	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "my-server",
			Backends: []nginxv1alpha1.NginxBackendSpec{
				{Address: "10.0.0.1", Port: 0},
			},
		},
	}

	result := ValidateNginxUpstream(upstream)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "port must be between")
}

// TestValidateNginxUpstream_InvalidAlgorithm verifies algorithm validation.
func TestValidateNginxUpstream_InvalidAlgorithm(t *testing.T) {
	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "my-server",
			Backends: []nginxv1alpha1.NginxBackendSpec{
				{Address: "10.0.0.1", Port: 80},
			},
			LoadBalancing: &nginxv1alpha1.NginxLoadBalancingSpec{
				Algorithm: "invalid_algo",
			},
		},
	}

	result := ValidateNginxUpstream(upstream)
	assert.False(t, result.Valid)
	assert.Contains(t, result.ErrorMessages(), "invalid algorithm")
}

// TestValidateNginxUpstream_ServiceDiscoveryWithoutBackends verifies SD skips backend check.
func TestValidateNginxUpstream_ServiceDiscoveryWithoutBackends(t *testing.T) {
	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "my-server",
			Backends:  []nginxv1alpha1.NginxBackendSpec{},
			ServiceDiscovery: &nginxv1alpha1.NginxServiceDiscoverySpec{
				Enabled:     true,
				ServiceName: "my-service",
				ServicePort: 8080,
			},
		},
	}

	result := ValidateNginxUpstream(upstream)
	assert.True(t, result.Valid)
}

// TestIsValidCIDROrIP verifies CIDR/IP validation helper.
func TestIsValidCIDROrIP(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"10.0.0.0/8", true},
		{"192.168.1.1", true},
		{"::1", true},
		{"all", true},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, isValidCIDROrIP(tt.input))
		})
	}
}

// TestValidationResult_ErrorMessages verifies error message formatting.
func TestValidationResult_ErrorMessages(t *testing.T) {
	result := ValidationResult{Valid: true}
	assert.Empty(t, result.ErrorMessages())

	result.addError("field1", "error 1")
	result.addError("field2", "error 2")
	assert.False(t, result.Valid)

	msg := result.ErrorMessages()
	assert.Contains(t, msg, "field1")
	assert.Contains(t, msg, "error 1")
	assert.Contains(t, msg, "field2")
	assert.Contains(t, msg, "error 2")
}
