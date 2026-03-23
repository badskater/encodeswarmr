# EncodeSwarmr Helm Chart

Deploys the EncodeSwarmr controller and agent on Kubernetes.

## Quick start

```bash
helm install encodeswarmr ./deploy/helm/encodeswarmr \
  --set postgres.dsn="postgresql://user:pass@postgres:5432/encodeswarmr" \
  --set auth.sessionSecret="$(openssl rand -base64 32)"
```

## Prerequisites

- Kubernetes 1.25+
- Helm 3.x
- A PostgreSQL 14+ database reachable from the cluster
- A ReadWriteMany storage class for shared media (e.g. NFS, AzureFile)

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Controller replicas | `1` |
| `image.controller.repository` | Controller image | `ghcr.io/badskater/encodeswarmr-controller` |
| `image.agent.repository` | Agent image | `ghcr.io/badskater/encodeswarmr-agent` |
| `postgres.dsn` | PostgreSQL DSN | `""` (required) |
| `auth.sessionSecret` | Session signing secret (32+ bytes) | `""` (required) |
| `persistence.enabled` | Mount shared media PVC | `true` |
| `persistence.size` | PVC size | `500Gi` |
| `persistence.storageClass` | StorageClass (must support RWX) | `""` |
| `persistence.existingClaim` | Use existing PVC | `""` |
| `agent.kind` | `Deployment` or `DaemonSet` | `Deployment` |
| `agent.replicaCount` | Agent replica count (Deployment only) | `2` |
| `hpa.enabled` | Enable HPA for agents | `true` |
| `hpa.maxReplicas` | Maximum agent replicas | `10` |
| `ingress.enabled` | Enable Ingress for web UI | `false` |
| `tracing.enabled` | Enable OpenTelemetry tracing | `false` |
| `tracing.endpoint` | OTLP endpoint | `jaeger-collector:4317` |

## TLS / mTLS

Supply base64-encoded PEM certificates in `tls.caCert`, `tls.cert`, and
`tls.key`.  The controller mounts them from the generated Secret.

## Upgrading

```bash
helm upgrade encodeswarmr ./deploy/helm/encodeswarmr -f myvalues.yaml
```
