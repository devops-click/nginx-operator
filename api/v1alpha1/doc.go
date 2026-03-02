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

// Package v1alpha1 contains API schema definitions for the nginx.devops.click API group.
//
// The following CRD types are defined:
//   - NginxServer: Represents an NGINX deployment instance managed by the operator.
//   - NginxRoute: Represents a virtual host / server block configuration.
//   - NginxUpstream: Represents upstream backend configuration.
//
// +kubebuilder:object:generate=true
// +groupName=nginx.devops.click
package v1alpha1
