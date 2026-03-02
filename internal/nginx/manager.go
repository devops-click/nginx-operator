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

// Package nginx provides resource management utilities for creating and updating
// Kubernetes resources (Deployments, Services, ConfigMaps) that represent
// NGINX instances managed by the operator.
//
// Usage:
//
//	mgr := nginx.NewResourceManager(client, scheme)
//	deployment := mgr.BuildDeployment(server)
//	service := mgr.BuildService(server)
package nginx

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	nginxv1alpha1 "github.com/devops-click/nginx-operator/api/v1alpha1"
)

const (
	// configVolumeName is the name of the volume that holds NGINX configuration.
	configVolumeName = "nginx-config"

	// configMountPath is where the ConfigMap is mounted in the NGINX container.
	configMountPath = "/etc/nginx"

	// confDMountPath is where server block configs are mounted.
	confDMountPath = "/etc/nginx/conf.d"

	// defaultNginxImage is the default NGINX container image.
	defaultNginxImage = "nginx:1.27-alpine"

	// reloaderImage is the config reloader sidecar image.
	reloaderImage = "ghcr.io/devops-click/nginx-operator-reloader"

	// nginxContainerName is the name of the main NGINX container.
	nginxContainerName = "nginx"

	// reloaderContainerName is the name of the config reloader sidecar.
	reloaderContainerName = "config-reloader"
)

// ResourceManager creates and manages Kubernetes resources for NGINX instances.
type ResourceManager struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewResourceManager creates a new ResourceManager.
//
// Usage:
//
//	mgr := nginx.NewResourceManager(k8sClient, scheme)
func NewResourceManager(c client.Client, s *runtime.Scheme) *ResourceManager {
	return &ResourceManager{
		client: c,
		scheme: s,
	}
}

// BuildDeployment creates a Deployment spec for the given NginxServer.
// The Deployment includes the NGINX container and a config-reloader sidecar.
//
// Usage:
//
//	deployment := mgr.BuildDeployment(server, "v1.0.0")
func (m *ResourceManager) BuildDeployment(server *nginxv1alpha1.NginxServer, reloaderTag string) *appsv1.Deployment {
	labels := buildLabels(server)
	replicas := int32(1)
	if server.Spec.Replicas != nil {
		replicas = *server.Spec.Replicas
	}

	image := defaultNginxImage
	if server.Spec.Image != "" {
		image = server.Spec.Image
	}

	pullPolicy := corev1.PullIfNotPresent
	if server.Spec.ImagePullPolicy != "" {
		pullPolicy = server.Spec.ImagePullPolicy
	}

	// Build NGINX container
	nginxContainer := corev1.Container{
		Name:            nginxContainerName,
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Ports:           buildContainerPorts(server),
		Resources:       server.Spec.Resources,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      configVolumeName,
				MountPath: configMountPath,
				ReadOnly:  true,
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt32(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt32(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
			TimeoutSeconds:      3,
			FailureThreshold:    3,
		},
	}

	// Apply container security context if specified
	if server.Spec.ContainerSecurityContext != nil {
		nginxContainer.SecurityContext = server.Spec.ContainerSecurityContext
	}

	// Add extra volume mounts
	if len(server.Spec.ExtraVolumeMounts) > 0 {
		nginxContainer.VolumeMounts = append(nginxContainer.VolumeMounts, server.Spec.ExtraVolumeMounts...)
	}

	// Add extra env vars
	if len(server.Spec.ExtraEnvVars) > 0 {
		nginxContainer.Env = server.Spec.ExtraEnvVars
	}

	// Build config-reloader sidecar
	reloaderFullImage := fmt.Sprintf("%s:%s", reloaderImage, reloaderTag)
	reloaderContainer := corev1.Container{
		Name:            reloaderContainerName,
		Image:           reloaderFullImage,
		ImagePullPolicy: pullPolicy,
		Resources:       server.Spec.ReloaderResources,
		Args: []string{
			"--watch-dir=/etc/nginx",
			"--nginx-binary=/usr/sbin/nginx",
			"--reload-timeout=30s",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      configVolumeName,
				MountPath: configMountPath,
				ReadOnly:  true,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			// Reloader needs to signal nginx process
			RunAsUser:  int64Ptr(0),
			RunAsGroup: int64Ptr(0),
		},
	}

	// Build volumes
	volumes := []corev1.Volume{
		{
			Name: configVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ConfigMapName(server),
					},
				},
			},
		},
	}

	// Add extra volumes
	if len(server.Spec.ExtraVolumes) > 0 {
		volumes = append(volumes, server.Spec.ExtraVolumes...)
	}

	// Build pod annotations
	podAnnotations := make(map[string]string)
	for k, v := range server.Spec.PodAnnotations {
		podAnnotations[k] = v
	}

	// Build pod labels
	podLabels := make(map[string]string)
	for k, v := range labels {
		podLabels[k] = v
	}
	for k, v := range server.Spec.PodLabels {
		podLabels[k] = v
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName(server),
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels(server),
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 0},
					MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					Containers:                []corev1.Container{nginxContainer, reloaderContainer},
					Volumes:                   volumes,
					ImagePullSecrets:          server.Spec.ImagePullSecrets,
					NodeSelector:              server.Spec.NodeSelector,
					Tolerations:               server.Spec.Tolerations,
					Affinity:                  server.Spec.Affinity,
					TopologySpreadConstraints: server.Spec.TopologySpreadConstraints,
					SecurityContext:           server.Spec.SecurityContext,
					TerminationGracePeriodSeconds: int64Ptr(30),
				},
			},
		},
	}

	return deployment
}

// BuildService creates a Service spec for the given NginxServer.
//
// Usage:
//
//	service := mgr.BuildService(server)
func (m *ResourceManager) BuildService(server *nginxv1alpha1.NginxServer) *corev1.Service {
	labels := buildLabels(server)

	servicePorts := []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt32(80),
			Protocol:   corev1.ProtocolTCP,
		},
	}

	serviceType := corev1.ServiceTypeClusterIP
	serviceAnnotations := make(map[string]string)

	if server.Spec.Service != nil {
		if server.Spec.Service.Type != "" {
			serviceType = server.Spec.Service.Type
		}

		// Override ports if specified
		if len(server.Spec.Service.Ports) > 0 {
			servicePorts = nil
			for _, p := range server.Spec.Service.Ports {
				sp := corev1.ServicePort{
					Name:     p.Name,
					Port:     p.Port,
					Protocol: p.Protocol,
				}
				if p.TargetPort > 0 {
					sp.TargetPort = intstr.FromInt32(p.TargetPort)
				}
				if sp.Protocol == "" {
					sp.Protocol = corev1.ProtocolTCP
				}
				servicePorts = append(servicePorts, sp)
			}
		}

		for k, v := range server.Spec.Service.Annotations {
			serviceAnnotations[k] = v
		}
	}

	// Add HTTPS port if TLS is enabled
	if server.Spec.TLS != nil && server.Spec.TLS.Enabled {
		hasHTTPS := false
		for _, p := range servicePorts {
			if p.Port == 443 {
				hasHTTPS = true
				break
			}
		}
		if !hasHTTPS {
			servicePorts = append(servicePorts, corev1.ServicePort{
				Name:       "https",
				Port:       443,
				TargetPort: intstr.FromInt32(443),
				Protocol:   corev1.ProtocolTCP,
			})
		}
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceName(server),
			Namespace:   server.Namespace,
			Labels:      labels,
			Annotations: serviceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: selectorLabels(server),
			Ports:    servicePorts,
		},
	}

	if server.Spec.Service != nil {
		if server.Spec.Service.LoadBalancerIP != "" {
			service.Spec.LoadBalancerIP = server.Spec.Service.LoadBalancerIP
		}
		if server.Spec.Service.ExternalTrafficPolicy != "" {
			service.Spec.ExternalTrafficPolicy = server.Spec.Service.ExternalTrafficPolicy
		}
	}

	return service
}

// BuildConfigMap creates a ConfigMap for the NGINX configuration.
//
// Usage:
//
//	cm := mgr.BuildConfigMap(server, mainConfig, serverConfigs)
func (m *ResourceManager) BuildConfigMap(server *nginxv1alpha1.NginxServer, mainConfig string, serverConfigs map[string]string) *corev1.ConfigMap {
	labels := buildLabels(server)

	data := make(map[string]string)
	data["nginx.conf"] = mainConfig

	for name, content := range serverConfigs {
		data[name] = content
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(server),
			Namespace: server.Namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

// SetOwnerReference sets the owner reference on a controlled object to the NginxServer.
//
// Usage:
//
//	err := mgr.SetOwnerReference(server, deployment)
func (m *ResourceManager) SetOwnerReference(owner *nginxv1alpha1.NginxServer, controlled metav1.Object) error {
	return controllerutil.SetControllerReference(owner, controlled, m.scheme)
}

// --- Naming Helpers ---

// DeploymentName returns the name for the NGINX Deployment.
func DeploymentName(server *nginxv1alpha1.NginxServer) string {
	return fmt.Sprintf("%s-nginx", server.Name)
}

// ServiceName returns the name for the NGINX Service.
func ServiceName(server *nginxv1alpha1.NginxServer) string {
	return fmt.Sprintf("%s-nginx", server.Name)
}

// ConfigMapName returns the name for the NGINX ConfigMap.
func ConfigMapName(server *nginxv1alpha1.NginxServer) string {
	return fmt.Sprintf("%s-nginx-config", server.Name)
}

// --- Internal Helpers ---

// buildLabels creates the standard set of labels for managed resources.
func buildLabels(server *nginxv1alpha1.NginxServer) map[string]string {
	return map[string]string{
		nginxv1alpha1.LabelManagedBy: nginxv1alpha1.LabelManagedByValue,
		nginxv1alpha1.LabelInstance:  server.Name,
		nginxv1alpha1.LabelComponent: "nginx",
		nginxv1alpha1.LabelPartOf:    "nginx-operator",
		"app.kubernetes.io/name":     "nginx",
		"app.kubernetes.io/version":  extractImageTag(server.Spec.Image),
	}
}

// selectorLabels returns the subset of labels used for pod selection.
func selectorLabels(server *nginxv1alpha1.NginxServer) map[string]string {
	return map[string]string{
		nginxv1alpha1.LabelInstance:  server.Name,
		nginxv1alpha1.LabelComponent: "nginx",
		"app.kubernetes.io/name":     "nginx",
	}
}

// buildContainerPorts creates the list of container ports from the NginxServer spec.
func buildContainerPorts(server *nginxv1alpha1.NginxServer) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          "http",
			ContainerPort: 80,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	if server.Spec.TLS != nil && server.Spec.TLS.Enabled {
		ports = append(ports, corev1.ContainerPort{
			Name:          "https",
			ContainerPort: 443,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	if server.Spec.Monitoring != nil && server.Spec.Monitoring.Enabled {
		metricsPort := int32(9113)
		if server.Spec.Monitoring.Port > 0 {
			metricsPort = server.Spec.Monitoring.Port
		}
		ports = append(ports, corev1.ContainerPort{
			Name:          "metrics",
			ContainerPort: metricsPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	return ports
}

// extractImageTag extracts the tag from a container image string.
func extractImageTag(image string) string {
	if image == "" {
		return "latest"
	}
	parts := strings.SplitN(image, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return "latest"
}

// int64Ptr returns a pointer to an int64 value.
func int64Ptr(i int64) *int64 {
	return &i
}
