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

// Package config provides NGINX configuration generation from CRD resources.
// It renders Go templates into valid NGINX configuration files, computes
// content hashes for change detection, and provides validation helpers.
//
// Usage:
//
//	gen := config.NewGenerator()
//	mainConf, err := gen.GenerateMainConfig(server)
//	routeConf, err := gen.GenerateRouteConfig(route, upstreams)
//	hash := gen.Hash(routeConf)
package config

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"text/template"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
)

// Generator renders NGINX configuration from CRD specs.
// It holds pre-parsed templates for efficient repeated rendering.
type Generator struct {
	mainTemplate     *template.Template
	routeTemplate    *template.Template
	upstreamTemplate *template.Template
}

// NewGenerator creates a new config Generator with pre-parsed templates.
//
// Usage:
//
//	gen := config.NewGenerator()
func NewGenerator() (*Generator, error) {
	funcMap := template.FuncMap{
		"join": func(sep string, s []string) string { return strings.Join(s, sep) },
		"indent":  indentString,
		"default": defaultValue,
	}

	mainTmpl, err := template.New("nginx.conf").Funcs(funcMap).Parse(mainConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse main config template: %w", err)
	}

	routeTmpl, err := template.New("route.conf").Funcs(funcMap).Parse(routeConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse route config template: %w", err)
	}

	upstreamTmpl, err := template.New("upstream.conf").Funcs(funcMap).Parse(upstreamConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse upstream config template: %w", err)
	}

	return &Generator{
		mainTemplate:     mainTmpl,
		routeTemplate:    routeTmpl,
		upstreamTemplate: upstreamTmpl,
	}, nil
}

// mainConfigData holds the data used to render the main nginx.conf template.
type mainConfigData struct {
	WorkerProcesses   string
	WorkerConnections int32
	ErrorLogLevel     string
	ServerTokens      string
	KeepaliveTimeout  string
	KeepaliveRequests int32
	ClientMaxBodySize string
	AccessLogEnabled  bool
	AccessLogFormat   string
	GzipEnabled       bool
	GzipTypes         string
	GzipMinLength     int32
	CustomMainSnippet string
	CustomHTTPSnippet string
	TLS               *nginxv1alpha1.NginxTLSSpec
	Monitoring        *nginxv1alpha1.NginxMonitoringSpec
}

// routeConfigData holds the data used to render a server block template.
type routeConfigData struct {
	ServerName            string
	ListenPort            int32
	HTTPSPort             int32
	ProxyProtocol         bool
	TLS                   *nginxv1alpha1.NginxRouteTLSSpec
	Locations             []locationData
	RateLimit             *nginxv1alpha1.NginxRateLimitSpec
	AccessControl         *nginxv1alpha1.NginxAccessControlSpec
	Headers               *nginxv1alpha1.NginxHeadersSpec
	CORS                  *nginxv1alpha1.NginxCORSSpec
	CustomServerSnippet   string
	RateLimitZoneName     string
}

// locationData holds pre-processed location block data for template rendering.
type locationData struct {
	Path                  string
	UpstreamName          string
	ProxyPass             string
	StaticContent         *nginxv1alpha1.NginxStaticContentSpec
	Return                *nginxv1alpha1.NginxReturnSpec
	ProxySettings         *nginxv1alpha1.NginxProxySettingsSpec
	RateLimit             *nginxv1alpha1.NginxRateLimitSpec
	Headers               *nginxv1alpha1.NginxHeadersSpec
	CustomLocationSnippet string
	RateLimitZoneName     string
}

// upstreamConfigData holds the data used to render an upstream block template.
type upstreamConfigData struct {
	Name              string
	Algorithm         string
	RandomTwoChoices  bool
	Backends          []nginxv1alpha1.NginxBackendSpec
	Keepalive         int32
	KeepaliveTimeout  string
	KeepaliveRequests int32
	CustomSnippet     string
}

// GenerateMainConfig renders the main nginx.conf from an NginxServer spec.
//
// Usage:
//
//	config, err := gen.GenerateMainConfig(server)
func (g *Generator) GenerateMainConfig(server *nginxv1alpha1.NginxServer) (string, error) {
	gc := server.Spec.GlobalConfig
	data := mainConfigData{
		WorkerProcesses:   "auto",
		WorkerConnections: 1024,
		ErrorLogLevel:     "warn",
		ServerTokens:      "off",
		KeepaliveTimeout:  "65s",
		KeepaliveRequests: 100,
		ClientMaxBodySize: "1m",
		AccessLogEnabled:  true,
		GzipEnabled:       true,
		GzipTypes:         "text/plain text/css application/json application/javascript text/xml application/xml image/svg+xml",
		GzipMinLength:     256,
		TLS:               server.Spec.TLS,
		Monitoring:        server.Spec.Monitoring,
	}

	// Override defaults with user-specified values
	if gc != nil {
		if gc.WorkerProcesses != "" {
			data.WorkerProcesses = gc.WorkerProcesses
		}
		if gc.WorkerConnections > 0 {
			data.WorkerConnections = gc.WorkerConnections
		}
		if gc.ErrorLogLevel != "" {
			data.ErrorLogLevel = gc.ErrorLogLevel
		}
		if gc.KeepaliveTimeout != "" {
			data.KeepaliveTimeout = gc.KeepaliveTimeout
		}
		if gc.KeepaliveRequests > 0 {
			data.KeepaliveRequests = gc.KeepaliveRequests
		}
		if gc.ClientMaxBodySize != "" {
			data.ClientMaxBodySize = gc.ClientMaxBodySize
		}
		if gc.ServerTokens {
			data.ServerTokens = "on"
		}
		data.AccessLogEnabled = gc.AccessLogEnabled
		if gc.AccessLogFormat != "" {
			data.AccessLogFormat = gc.AccessLogFormat
		}
		data.GzipEnabled = gc.GzipEnabled
		if len(gc.GzipTypes) > 0 {
			data.GzipTypes = strings.Join(gc.GzipTypes, " ")
		}
		if gc.GzipMinLength > 0 {
			data.GzipMinLength = gc.GzipMinLength
		}
		data.CustomMainSnippet = gc.CustomMainSnippet
		data.CustomHTTPSnippet = gc.CustomHTTPSnippet
	}

	var buf bytes.Buffer
	if err := g.mainTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render main config: %w", err)
	}

	return buf.String(), nil
}

// GenerateRouteConfig renders a server block configuration from an NginxRoute spec.
// The upstreamMap maps upstream names to their rendered upstream block names.
//
// Usage:
//
//	config, err := gen.GenerateRouteConfig(route, map[string]string{"backend": "ns-backend"})
func (g *Generator) GenerateRouteConfig(route *nginxv1alpha1.NginxRoute, upstreamMap map[string]string) (string, error) {
	data := routeConfigData{
		ServerName:          route.Spec.ServerName,
		ListenPort:          80,
		HTTPSPort:           443,
		TLS:                 route.Spec.TLS,
		RateLimit:           route.Spec.RateLimit,
		AccessControl:       route.Spec.AccessControl,
		Headers:             route.Spec.Headers,
		CORS:                route.Spec.CORS,
		CustomServerSnippet: route.Spec.CustomServerSnippet,
	}

	// Set listen configuration
	if route.Spec.Listen != nil {
		if route.Spec.Listen.Port > 0 {
			data.ListenPort = route.Spec.Listen.Port
		}
		if route.Spec.Listen.HTTPSPort > 0 {
			data.HTTPSPort = route.Spec.Listen.HTTPSPort
		}
		data.ProxyProtocol = route.Spec.Listen.ProxyProtocol
	}

	// Generate rate limit zone name
	if data.RateLimit != nil && data.RateLimit.Enabled {
		data.RateLimitZoneName = sanitizeName(fmt.Sprintf("%s-%s", route.Namespace, route.Name))
	}

	// Process locations
	for _, loc := range route.Spec.Locations {
		ld := locationData{
			Path:                  loc.Path,
			ProxyPass:             loc.ProxyPass,
			StaticContent:         loc.StaticContent,
			Return:                loc.Return,
			ProxySettings:         loc.ProxySettings,
			RateLimit:             loc.RateLimit,
			Headers:               loc.Headers,
			CustomLocationSnippet: loc.CustomLocationSnippet,
		}

		// Resolve upstream reference
		if loc.UpstreamRef != "" {
			if upstreamName, ok := upstreamMap[loc.UpstreamRef]; ok {
				ld.UpstreamName = upstreamName
			} else {
				return "", fmt.Errorf("upstream reference %q not found in upstream map", loc.UpstreamRef)
			}
		}

		// Generate per-location rate limit zone name
		if ld.RateLimit != nil && ld.RateLimit.Enabled {
			ld.RateLimitZoneName = sanitizeName(fmt.Sprintf("%s-%s-%s",
				route.Namespace, route.Name, strings.ReplaceAll(loc.Path, "/", "_")))
		}

		data.Locations = append(data.Locations, ld)
	}

	var buf bytes.Buffer
	if err := g.routeTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render route config for %s/%s: %w", route.Namespace, route.Name, err)
	}

	return buf.String(), nil
}

// GenerateUpstreamConfig renders an upstream block configuration from an NginxUpstream spec.
// The returned upstream name follows the format: namespace-name.
//
// Usage:
//
//	name, config, err := gen.GenerateUpstreamConfig(upstream)
func (g *Generator) GenerateUpstreamConfig(upstream *nginxv1alpha1.NginxUpstream) (string, string, error) {
	name := sanitizeName(fmt.Sprintf("%s-%s", upstream.Namespace, upstream.Name))

	data := upstreamConfigData{
		Name:              name,
		Algorithm:         "round_robin",
		Backends:          upstream.Spec.Backends,
		Keepalive:         upstream.Spec.Keepalive,
		KeepaliveTimeout:  upstream.Spec.KeepaliveTimeout,
		KeepaliveRequests: upstream.Spec.KeepaliveRequests,
		CustomSnippet:     upstream.Spec.CustomUpstreamSnippet,
	}

	if upstream.Spec.LoadBalancing != nil {
		data.Algorithm = upstream.Spec.LoadBalancing.Algorithm
		data.RandomTwoChoices = upstream.Spec.LoadBalancing.RandomTwoChoices
	}

	// Set defaults
	if data.Keepalive == 0 {
		data.Keepalive = 32
	}
	if data.KeepaliveTimeout == "" {
		data.KeepaliveTimeout = "60s"
	}
	if data.KeepaliveRequests == 0 {
		data.KeepaliveRequests = 100
	}

	var buf bytes.Buffer
	if err := g.upstreamTemplate.Execute(&buf, data); err != nil {
		return "", "", fmt.Errorf("failed to render upstream config for %s/%s: %w", upstream.Namespace, upstream.Name, err)
	}

	return name, buf.String(), nil
}

// GenerateFullConfig assembles a complete NGINX configuration from an NginxServer,
// its associated NginxRoutes, and NginxUpstreams. Routes are sorted by priority.
//
// Usage:
//
//	fullConfig, err := gen.GenerateFullConfig(server, routes, upstreams)
func (g *Generator) GenerateFullConfig(
	server *nginxv1alpha1.NginxServer,
	routes []nginxv1alpha1.NginxRoute,
	upstreams []nginxv1alpha1.NginxUpstream,
) (string, error) {
	// Generate main config
	mainConf, err := g.GenerateMainConfig(server)
	if err != nil {
		return "", fmt.Errorf("failed to generate main config: %w", err)
	}

	// Generate upstream configs and build the name map
	upstreamMap := make(map[string]string)
	var upstreamConfigs []string
	for i := range upstreams {
		name, conf, err := g.GenerateUpstreamConfig(&upstreams[i])
		if err != nil {
			return "", fmt.Errorf("failed to generate upstream config for %s/%s: %w",
				upstreams[i].Namespace, upstreams[i].Name, err)
		}
		upstreamMap[upstreams[i].Name] = name
		upstreamConfigs = append(upstreamConfigs, conf)
	}

	// Sort routes by priority (lower first)
	sortedRoutes := make([]nginxv1alpha1.NginxRoute, len(routes))
	copy(sortedRoutes, routes)
	sort.Slice(sortedRoutes, func(i, j int) bool {
		return sortedRoutes[i].Spec.Priority < sortedRoutes[j].Spec.Priority
	})

	// Generate route configs
	var routeConfigs []string
	for i := range sortedRoutes {
		conf, err := g.GenerateRouteConfig(&sortedRoutes[i], upstreamMap)
		if err != nil {
			return "", fmt.Errorf("failed to generate route config for %s/%s: %w",
				sortedRoutes[i].Namespace, sortedRoutes[i].Name, err)
		}
		routeConfigs = append(routeConfigs, conf)
	}

	// Assemble full configuration
	var fullConfig strings.Builder
	fullConfig.WriteString(mainConf)
	fullConfig.WriteString("\n")

	if len(upstreamConfigs) > 0 {
		fullConfig.WriteString("# --- Upstream Blocks ---\n")
		for _, uc := range upstreamConfigs {
			fullConfig.WriteString(uc)
			fullConfig.WriteString("\n")
		}
	}

	if len(routeConfigs) > 0 {
		fullConfig.WriteString("# --- Server Blocks ---\n")
		for _, rc := range routeConfigs {
			fullConfig.WriteString(rc)
			fullConfig.WriteString("\n")
		}
	}

	return fullConfig.String(), nil
}

// Hash computes a SHA-256 hash of the given configuration content.
// Used for change detection to avoid unnecessary NGINX reloads.
//
// Usage:
//
//	hash := gen.Hash(configContent)
func (g *Generator) Hash(content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
}

// sanitizeName converts a string into a valid NGINX identifier by replacing
// dots, slashes, and other special characters with underscores.
func sanitizeName(name string) string {
	replacer := strings.NewReplacer(
		".", "_",
		"/", "_",
		"-", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

// indentString indents each line of a string by the specified number of spaces.
func indentString(spaces int, s string) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

// defaultValue returns the default value if the given value is empty.
func defaultValue(def, val string) string {
	if val == "" {
		return def
	}
	return val
}
