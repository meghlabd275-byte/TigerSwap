# TigerSwap Kubernetes Configuration

# Kubernetes cluster configuration for multi-region deployment

## Quick Start

```bash
# Create cluster
kind create cluster --name tigerswap

# Apply configurations
kubectl apply -f namespace.yaml
kubectl apply -f configmaps/
kubectl apply -f services/
kubectl apply -f deployments/
kubectl apply -f ingress/
```

## Directory Structure

```
infrastructure/
├── kind-config.yaml          # Local development cluster
├── namespace.yaml          # Namespace definition
├── configmaps/            # ConfigMaps for each service
│   ├── api-gateway.yaml
│   ├── wallet-service.yaml
│   └── indexer-service.yaml
├── services/              # Service definitions
│   ├── api-gateway.yaml
│   ├── wallet-service.yaml
│   ├── matching-engine.yaml
│   └── indexer-service.yaml
├── deployments/           # Deployment manifests
│   ├── api-gateway.yaml
│   ├── wallet-service.yaml
│   ├── matching-engine.yaml
│   └── indexer-service.yaml
├── ingress/               # Ingress for external access
│   └── api-ingress.yaml
├── service-mesh/          # Istio service mesh
│   ├── virtual-service.yaml
│   └── destination-rule.yaml
├── secrets/               # Secret manifests (encrypted)
│   └── .gitkeep
├── monitoring/            # Prometheus/Grafana
│   ├── prometheus.yaml
│   └── grafana.yaml
├── disaster-recovery/       # Backup configs
│   └── backup-cronjob.yaml
└── Makefile
```

## Multi-Region Setup

### Regions
- us-east-1 (Primary)
- us-west-2 (Secondary)
- eu-west-1 (Europe)

### Deployment Strategy
- Active-Passive for disaster recovery
- Cross-region replication for data

## Environment Variables

```yaml
environment: production
log_level: info
tracing_enabled: true
metrics_enabled: true
```

## Resource Limits

```yaml
resources:
  limits:
    cpu: "2000m"
    memory: "4Gi"
  requests:
    cpu: "500m"
    memory: "1Gi"
```

## Health Checks

- Liveness: /health
- Readiness: /ready
- Startup: /startup