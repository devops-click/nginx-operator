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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
)

// TestNewGenerator verifies that the config generator initializes correctly.
func TestNewGenerator(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)
	assert.NotNil(t, gen)
	assert.NotNil(t, gen.mainTemplate)
	assert.NotNil(t, gen.routeTemplate)
	assert.NotNil(t, gen.upstreamTemplate)
}

// TestGenerateMainConfig_Defaults verifies main config generation with default values.
func TestGenerateMainConfig_Defaults(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-server",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxServerSpec{},
	}

	config, err := gen.GenerateMainConfig(server)
	require.NoError(t, err)
	assert.NotEmpty(t, config)

	// Verify defaults are present
	assert.Contains(t, config, "worker_processes auto")
	assert.Contains(t, config, "worker_connections 1024")
	assert.Contains(t, config, "error_log /var/log/nginx/error.log warn")
	assert.Contains(t, config, "server_tokens off")
	assert.Contains(t, config, "keepalive_timeout 65s")
	assert.Contains(t, config, "gzip on")
	assert.Contains(t, config, "sendfile on")
	assert.Contains(t, config, "tcp_nopush on")
	assert.Contains(t, config, "include /etc/nginx/conf.d/*.conf")
}

// TestGenerateMainConfig_CustomValues verifies main config with custom global config.
func TestGenerateMainConfig_CustomValues(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-server",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxServerSpec{
			GlobalConfig: &nginxv1alpha1.NginxGlobalConfig{
				WorkerProcesses:   "4",
				WorkerConnections: 4096,
				ErrorLogLevel:     "error",
				KeepaliveTimeout:  "120s",
				ClientMaxBodySize: "50m",
				ServerTokens:      false,
				GzipEnabled:       true,
				GzipMinLength:     512,
			},
		},
	}

	config, err := gen.GenerateMainConfig(server)
	require.NoError(t, err)

	assert.Contains(t, config, "worker_processes 4")
	assert.Contains(t, config, "worker_connections 4096")
	assert.Contains(t, config, "error_log /var/log/nginx/error.log error")
	assert.Contains(t, config, "keepalive_timeout 120s")
	assert.Contains(t, config, "client_max_body_size 50m")
	assert.Contains(t, config, "gzip_min_length 512")
}

// TestGenerateMainConfig_WithTLS verifies TLS section is generated when enabled.
func TestGenerateMainConfig_WithTLS(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-server",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxServerSpec{
			TLS: &nginxv1alpha1.NginxTLSSpec{
				Enabled:             true,
				Protocols:           []string{"TLSv1.2", "TLSv1.3"},
				PreferServerCiphers: true,
				SessionCache:        "shared:SSL:10m",
				SessionTimeout:      "1d",
			},
		},
	}

	config, err := gen.GenerateMainConfig(server)
	require.NoError(t, err)

	assert.Contains(t, config, "ssl_protocols TLSv1.2 TLSv1.3")
	assert.Contains(t, config, "ssl_prefer_server_ciphers on")
	assert.Contains(t, config, "ssl_session_cache shared:SSL:10m")
	assert.Contains(t, config, "ssl_session_timeout 1d")
}

// TestGenerateMainConfig_WithMonitoring verifies monitoring stub_status is generated.
func TestGenerateMainConfig_WithMonitoring(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-server",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxServerSpec{
			Monitoring: &nginxv1alpha1.NginxMonitoringSpec{
				Enabled: true,
				Port:    9113,
			},
		},
	}

	config, err := gen.GenerateMainConfig(server)
	require.NoError(t, err)

	assert.Contains(t, config, "stub_status")
	assert.Contains(t, config, "127.0.0.1:9113")
	assert.Contains(t, config, "/healthz")
}

// TestGenerateRouteConfig verifies route config generation with a simple location.
func TestGenerateRouteConfig(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "test-server",
			ServerName: "example.com",
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{
					Path:      "/",
					ProxyPass: "http://backend:8080",
				},
			},
		},
	}

	config, err := gen.GenerateRouteConfig(route, map[string]string{})
	require.NoError(t, err)

	assert.Contains(t, config, "server_name example.com")
	assert.Contains(t, config, "listen 80")
	assert.Contains(t, config, "location /")
	assert.Contains(t, config, "proxy_pass http://backend:8080")
	assert.Contains(t, config, "proxy_set_header Host $host")
}

// TestGenerateRouteConfig_WithTLS verifies route config with TLS settings.
func TestGenerateRouteConfig_WithTLS(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-route",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "test-server",
			ServerName: "secure.example.com",
			TLS: &nginxv1alpha1.NginxRouteTLSSpec{
				Enabled:      true,
				SecretName:   "tls-secret",
				RedirectHTTP: true,
			},
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{
					Path: "/",
					Return: &nginxv1alpha1.NginxReturnSpec{
						Code: 200,
						Body: "OK",
					},
				},
			},
		},
	}

	config, err := gen.GenerateRouteConfig(route, map[string]string{})
	require.NoError(t, err)

	assert.Contains(t, config, "listen 443 ssl")
	assert.Contains(t, config, "ssl_certificate /etc/nginx/ssl/tls-secret/tls.crt")
	assert.Contains(t, config, "ssl_certificate_key /etc/nginx/ssl/tls-secret/tls.key")
	assert.Contains(t, config, "return 301 https://$host$request_uri")
}

// TestGenerateRouteConfig_WithUpstreamRef verifies upstream reference resolution.
func TestGenerateRouteConfig_WithUpstreamRef(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "upstream-route",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "test-server",
			ServerName: "api.example.com",
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{
					Path:        "/api",
					UpstreamRef: "backend-upstream",
				},
			},
		},
	}

	upstreamMap := map[string]string{
		"backend-upstream": "default_backend_upstream",
	}

	config, err := gen.GenerateRouteConfig(route, upstreamMap)
	require.NoError(t, err)

	assert.Contains(t, config, "proxy_pass http://default_backend_upstream")
}

// TestGenerateRouteConfig_MissingUpstreamRef verifies error on missing upstream.
func TestGenerateRouteConfig_MissingUpstreamRef(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	route := &nginxv1alpha1.NginxRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-route",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxRouteSpec{
			ServerRef:  "test-server",
			ServerName: "bad.example.com",
			Locations: []nginxv1alpha1.NginxLocationSpec{
				{
					Path:        "/",
					UpstreamRef: "nonexistent",
				},
			},
		},
	}

	_, err = gen.GenerateRouteConfig(route, map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestGenerateUpstreamConfig verifies upstream block generation.
func TestGenerateUpstreamConfig(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "test-server",
			Backends: []nginxv1alpha1.NginxBackendSpec{
				{Address: "10.0.0.1", Port: 8080, Weight: 1, MaxFails: 3, FailTimeout: "10s"},
				{Address: "10.0.0.2", Port: 8080, Weight: 2, MaxFails: 3, FailTimeout: "10s"},
			},
			LoadBalancing: &nginxv1alpha1.NginxLoadBalancingSpec{
				Algorithm: "least_conn",
			},
			Keepalive:         64,
			KeepaliveTimeout:  "120s",
			KeepaliveRequests: 200,
		},
	}

	name, config, err := gen.GenerateUpstreamConfig(upstream)
	require.NoError(t, err)

	assert.Equal(t, "default_backend", name)
	assert.Contains(t, config, "upstream default_backend")
	assert.Contains(t, config, "least_conn")
	assert.Contains(t, config, "server 10.0.0.1:8080")
	assert.Contains(t, config, "server 10.0.0.2:8080 weight=2")
	assert.Contains(t, config, "keepalive 64")
	assert.Contains(t, config, "keepalive_timeout 120s")
	assert.Contains(t, config, "keepalive_requests 200")
}

// TestGenerateUpstreamConfig_IPHash verifies ip_hash algorithm.
func TestGenerateUpstreamConfig_IPHash(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sticky",
			Namespace: "prod",
		},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "test-server",
			Backends: []nginxv1alpha1.NginxBackendSpec{
				{Address: "10.0.0.1", Port: 80, Weight: 1, MaxFails: 3, FailTimeout: "10s"},
			},
			LoadBalancing: &nginxv1alpha1.NginxLoadBalancingSpec{
				Algorithm: "ip_hash",
			},
		},
	}

	_, config, err := gen.GenerateUpstreamConfig(upstream)
	require.NoError(t, err)

	assert.Contains(t, config, "ip_hash")
}

// TestGenerateUpstreamConfig_BackupAndDown verifies backup and down flags.
func TestGenerateUpstreamConfig_BackupAndDown(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	upstream := &nginxv1alpha1.NginxUpstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "with-backup",
			Namespace: "default",
		},
		Spec: nginxv1alpha1.NginxUpstreamSpec{
			ServerRef: "test-server",
			Backends: []nginxv1alpha1.NginxBackendSpec{
				{Address: "10.0.0.1", Port: 80, Weight: 1, MaxFails: 3, FailTimeout: "10s"},
				{Address: "10.0.0.2", Port: 80, Weight: 1, MaxFails: 3, FailTimeout: "10s", Backup: true},
				{Address: "10.0.0.3", Port: 80, Weight: 1, MaxFails: 3, FailTimeout: "10s", Down: true},
			},
		},
	}

	_, config, err := gen.GenerateUpstreamConfig(upstream)
	require.NoError(t, err)

	assert.Contains(t, config, "backup")
	assert.Contains(t, config, "down")
}

// TestHash verifies that hashing is deterministic and changes with content.
func TestHash(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	hash1 := gen.Hash("config content A")
	hash2 := gen.Hash("config content A")
	hash3 := gen.Hash("config content B")

	// Same content = same hash
	assert.Equal(t, hash1, hash2)
	// Different content = different hash
	assert.NotEqual(t, hash1, hash3)
	// SHA-256 produces 64 hex characters
	assert.Len(t, hash1, 64)
}

// TestGenerateFullConfig verifies full config assembly with routes and upstreams.
func TestGenerateFullConfig(t *testing.T) {
	gen, err := NewGenerator()
	require.NoError(t, err)

	server := &nginxv1alpha1.NginxServer{
		ObjectMeta: metav1.ObjectMeta{Name: "prod", Namespace: "default"},
		Spec:       nginxv1alpha1.NginxServerSpec{},
	}

	upstreams := []nginxv1alpha1.NginxUpstream{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "api-backend", Namespace: "default"},
			Spec: nginxv1alpha1.NginxUpstreamSpec{
				ServerRef: "prod",
				Backends: []nginxv1alpha1.NginxBackendSpec{
					{Address: "10.0.1.1", Port: 8080, Weight: 1, MaxFails: 3, FailTimeout: "10s"},
				},
			},
		},
	}

	routes := []nginxv1alpha1.NginxRoute{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "api-route", Namespace: "default"},
			Spec: nginxv1alpha1.NginxRouteSpec{
				ServerRef:  "prod",
				ServerName: "api.example.com",
				Priority:   100,
				Locations: []nginxv1alpha1.NginxLocationSpec{
					{Path: "/", UpstreamRef: "api-backend"},
				},
			},
		},
	}

	fullConfig, err := gen.GenerateFullConfig(server, routes, upstreams)
	require.NoError(t, err)

	// Verify all sections are present
	assert.Contains(t, fullConfig, "worker_processes")
	assert.Contains(t, fullConfig, "upstream default_api_backend")
	assert.Contains(t, fullConfig, "server_name api.example.com")
	assert.Contains(t, fullConfig, "Upstream Blocks")
	assert.Contains(t, fullConfig, "Server Blocks")
}

// TestSanitizeName verifies name sanitization for NGINX identifiers.
func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"default-backend", "default_backend"},
		{"prod.api-service", "prod_api_service"},
		{"ns/name", "ns_name"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIndentString verifies string indentation helper.
func TestIndentString(t *testing.T) {
	input := "line1\nline2\nline3"
	result := indentString(4, input)

	lines := strings.Split(result, "\n")
	assert.Equal(t, "    line1", lines[0])
	assert.Equal(t, "    line2", lines[1])
	assert.Equal(t, "    line3", lines[2])
}
