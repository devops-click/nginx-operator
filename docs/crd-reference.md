# CRD Reference

This document provides the complete API reference for the NGINX Operator custom resource definitions.

## NginxServer

**API Version**: `nginx.devops.click/v1alpha1`
**Kind**: `NginxServer`
**Short Names**: `ns`, `nxs`

Represents a managed NGINX deployment instance. The operator creates and manages a Deployment, Service, ConfigMap, and optionally PDB/HPA.

### Spec Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `replicas` | int32 | `1` | Number of NGINX pod replicas |
| `image` | string | `nginx:1.27-alpine` | NGINX container image |
| `imagePullPolicy` | string | `IfNotPresent` | Image pull policy |
| `resources` | ResourceRequirements | - | CPU/memory resources for NGINX container |
| `reloaderResources` | ResourceRequirements | - | CPU/memory resources for reloader sidecar |
| `service` | NginxServiceSpec | - | Service configuration |
| `tls` | NginxTLSSpec | - | Global TLS settings |
| `globalConfig` | NginxGlobalConfig | - | Global NGINX directives |
| `monitoring` | NginxMonitoringSpec | - | Prometheus metrics settings |
| `autoscaling` | NginxAutoscalingSpec | - | HPA settings |
| `podDisruptionBudget` | NginxPDBSpec | - | PDB settings |
| `nodeSelector` | map[string]string | - | Node selection constraints |
| `tolerations` | []Toleration | - | Pod tolerations |
| `affinity` | Affinity | - | Pod affinity rules |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | []Condition | Current state conditions |
| `readyReplicas` | int32 | Number of ready NGINX pods |
| `configHash` | string | SHA-256 hash of current config |
| `lastReloadTime` | Time | Last successful reload timestamp |
| `routeCount` | int32 | Number of associated NginxRoutes |
| `upstreamCount` | int32 | Number of associated NginxUpstreams |

---

## NginxRoute

**API Version**: `nginx.devops.click/v1alpha1`
**Kind**: `NginxRoute`
**Short Names**: `nr`, `nxr`

Represents a virtual host / server block configuration applied to an NginxServer.

### Spec Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `serverRef` | string | **required** | Name of the NginxServer this route belongs to |
| `serverName` | string | **required** | NGINX server_name directive value |
| `listen` | NginxListenSpec | - | Listen directive configuration |
| `tls` | NginxRouteTLSSpec | - | Per-route TLS settings |
| `locations` | []NginxLocationSpec | **required** | Location blocks (min 1) |
| `rateLimit` | NginxRateLimitSpec | - | Server-level rate limiting |
| `accessControl` | NginxAccessControlSpec | - | IP-based access control |
| `headers` | NginxHeadersSpec | - | Custom HTTP headers |
| `cors` | NginxCORSSpec | - | CORS settings |
| `priority` | int32 | `100` | Order of server blocks (lower = first) |

### Location Types (mutually exclusive)

Each location must have exactly one of:

- `upstreamRef` — Reference to an NginxUpstream for proxy_pass
- `proxyPass` — Direct proxy_pass URL
- `staticContent` — Serve static files
- `return` — Fixed response (redirect, error page)

---

## NginxUpstream

**API Version**: `nginx.devops.click/v1alpha1`
**Kind**: `NginxUpstream`
**Short Names**: `nu`, `nxu`

Represents an NGINX upstream block with backend servers.

### Spec Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `serverRef` | string | **required** | Name of the NginxServer this upstream belongs to |
| `backends` | []NginxBackendSpec | **required** | Backend server list |
| `loadBalancing` | NginxLoadBalancingSpec | `round_robin` | Load balancing algorithm |
| `healthCheck` | NginxHealthCheckSpec | - | Active health checking |
| `keepalive` | int32 | `32` | Max idle keepalive connections |
| `serviceDiscovery` | NginxServiceDiscoverySpec | - | Auto-discover from K8s Service |

### Load Balancing Algorithms

- `round_robin` — Default, even distribution
- `least_conn` — Fewest active connections
- `ip_hash` — Session affinity by client IP
- `random` — Random selection (with optional two-choice)
