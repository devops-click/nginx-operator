# NGINX Operator

[![CI](https://github.com/devops-click/nginx-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/devops-click/nginx-operator/actions/workflows/ci.yaml)
[![Security](https://github.com/devops-click/nginx-operator/actions/workflows/security.yaml/badge.svg)](https://github.com/devops-click/nginx-operator/actions/workflows/security.yaml)
[![codecov](https://codecov.io/gh/devops-click/nginx-operator/branch/main/graph/badge.svg)](https://codecov.io/gh/devops-click/nginx-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/devops-click/nginx-operator)](https://goreportcard.com/report/github.com/devops-click/nginx-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/devops-click/nginx-operator/badge)](https://securityscorecards.dev/viewer/?uri=github.com/devops-click/nginx-operator)

**Production-grade Kubernetes operator for managing NGINX server deployments with declarative CRDs.**

Designed for high-performance, security-first environments with multi-replica safety, config validation before reload, and full observability.

---

## Features

- **Declarative CRDs** — Manage NGINX instances, routes, and upstreams as Kubernetes resources
- **Safe Config Reloads** — SHA-256 change detection, `nginx -t` validation, atomic updates
- **High Availability** — Leader election, PodDisruptionBudgets, topology spread constraints
- **Multi-Replica Safe** — Only one operator replica reconciles at a time via Kubernetes Lease
- **Service Discovery** — Auto-discover upstream backends from Kubernetes Services
- **Full TLS Support** — Per-route TLS, HTTP-to-HTTPS redirect, modern cipher defaults
- **Rate Limiting** — Per-server and per-location rate limiting with configurable zones
- **CORS & Security Headers** — Built-in CORS handling and security header injection
- **Prometheus Metrics** — Built-in metrics endpoint with optional ServiceMonitor
- **Config Reloader Sidecar** — Lightweight sidecar for zero-downtime config updates
- **Helm Chart** — OCI-based distribution via GitHub Container Registry

## Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│                    Operator Deployment                       │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Controller Manager (with Leader Election)             │  │
│  │  ┌──────────────┐ ┌─────────────┐ ┌────────────────┐   │  │
│  │  │ NginxServer  │ │ NginxRoute  │ │ NginxUpstream  │   │  │
│  │  │ Reconciler   │ │ Reconciler  │ │ Reconciler     │   │  │
│  │  └──────┬───────┘ └──────┬──────┘ └───────┬────────┘   │  │
│  └─────────┼────────────────┼────────────────┼────────────┘  │
└────────────┼────────────────┼────────────────┼───────────────┘
             │                │                │
             ▼                ▼                ▼
    ┌─────────────┐  ┌──────────────┐  ┌───────────────┐
    │ Deployment  │  │  ConfigMap   │  │   Service     │
    │ (NGINX +    │  │ (nginx.conf  │  │ (ClusterIP/   │
    │  Reloader)  │  │  + routes)   │  │  LoadBalancer)│
    └─────────────┘  └──────────────┘  └───────────────┘
```

## Quick Start

### Prerequisites

- Kubernetes cluster v1.26+
- Helm v3.8+
- `kubectl` configured to your cluster

### Install with Helm

```bash
# Install from OCI registry
helm install nginx-operator oci://ghcr.io/devops-click/charts/nginx-operator \
  --namespace nginx-operator-system \
  --create-namespace

# Or install from local chart (development)
helm install nginx-operator charts/nginx-operator \
  --namespace nginx-operator-system \
  --create-namespace
```

### Create Your First NGINX Instance

```yaml
# 1. Create an NginxServer (deploys NGINX)
apiVersion: nginx.devops.click/v1alpha1
kind: NginxServer
metadata:
  name: my-nginx
spec:
  replicas: 2
  image: nginx:1.27-alpine
  service:
    type: ClusterIP
    ports:
      - name: http
        port: 80
  monitoring:
    enabled: true
---
# 2. Create an NginxUpstream (define backends)
apiVersion: nginx.devops.click/v1alpha1
kind: NginxUpstream
metadata:
  name: api-backend
spec:
  serverRef: my-nginx
  backends:
    - address: api-service.default.svc.cluster.local
      port: 8080
      weight: 1
  loadBalancing:
    algorithm: least_conn
  keepalive: 32
---
# 3. Create an NginxRoute (configure virtual host)
apiVersion: nginx.devops.click/v1alpha1
kind: NginxRoute
metadata:
  name: api-route
spec:
  serverRef: my-nginx
  serverName: api.example.com
  headers:
    securityHeaders: true
  locations:
    - path: /
      upstreamRef: api-backend
      proxySettings:
        connectTimeout: "5s"
        readTimeout: "30s"
```

```bash
kubectl apply -f examples/
kubectl get nginxservers,nginxroutes,nginxupstreams
```

## CRD Reference

| CRD | Short Names | Description |
|-----|------------|-------------|
| **NginxServer** | `ns`, `nxs` | Manages an NGINX Deployment with Service, ConfigMap, and optional PDB/HPA |
| **NginxRoute** | `nr`, `nxr` | Defines virtual host / server block configuration |
| **NginxUpstream** | `nu`, `nxu` | Defines upstream backend servers with load balancing |

See [docs/crd-reference.md](docs/crd-reference.md) for the full API specification.

## Configuration

### Helm Values

Key configuration options (see `charts/nginx-operator/values.yaml` for all options):

| Parameter | Default | Description |
|-----------|---------|-------------|
| `replicaCount` | `2` | Number of operator replicas (HA) |
| `operator.leaderElection.enabled` | `true` | Enable leader election |
| `operator.debug` | `false` | Enable debug logging |
| `metrics.enabled` | `true` | Enable Prometheus metrics |
| `metrics.serviceMonitor.enabled` | `false` | Create ServiceMonitor for Prometheus |
| `podDisruptionBudget.enabled` | `true` | Enable PDB for operator pods |

## Development

```bash
# Clone the repository
git clone https://github.com/devops-click/nginx-operator.git
cd nginx-operator

# Install dependencies
go mod download

# Run tests
make test

# Run linter
make lint

# Build binaries
make build-all

# Build Docker images
make docker-build-all

# Run operator locally against current cluster
make run
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## Versioning

This project follows [Semantic Versioning](https://semver.org):

| Bump | When |
|------|------|
| **MAJOR** | Breaking CRD schema change, removed fields |
| **MINOR** | New CRD kind, new feature, new values.yaml keys |
| **PATCH** | Bug fix, performance improvement, no API change |

The Helm chart version and operator version (appVersion) are versioned **independently**.

## Security

See [SECURITY.md](SECURITY.md) for our security policy and how to report vulnerabilities.

## License

This project is licensed under the Apache License 2.0 — see the [LICENSE](LICENSE) file for details.
