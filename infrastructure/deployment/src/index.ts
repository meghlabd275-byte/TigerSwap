/**
 * TigerSwap Infrastructure - Kubernetes Deployment
 * 
 * Kubernetes configurations for TigerSwap ecosystem.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

// ============================================================================
// Kubernetes Manifests
// ============================================================================

export const K8S_MANIFESTS = {
  // Namespace
  namespace: {
    apiVersion: 'v1',
    kind: 'Namespace',
    metadata: { name: 'tigerswap' },
  },

  // API Deployment
  apiDeployment: {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: {
      name: 'tigerswap-api',
      namespace: 'tigerswap',
    },
    spec: {
      replicas: 3,
      selector: { matchLabels: { app: 'tigerswap-api' } },
      template: {
        metadata: { labels: { app: 'tigerswap-api' } },
        spec: {
          containers: [{
            name: 'api',
            image: 'tigerswap/api:latest',
            ports: [{ containerPort: 8080 }],
            env: [
              { name: 'NODE_ENV', value: 'production' },
              { name: 'PORT', value: '8080' },
            ],
            resources: {
              requests: { memory: '256Mi', cpu: '250m' },
              limits: { memory: '512Mi', cpu: '500m' },
            },
          }],
        },
      },
    },
  },

  // API Service
  apiService: {
    apiVersion: 'v1',
    kind: 'Service',
    metadata: {
      name: 'tigerswap-api',
      namespace: 'tigerswap',
    },
    spec: {
      selector: { app: 'tigerswap-api' },
      ports: [{ port: 80, targetPort: 8080 }],
      type: 'ClusterIP',
    },
  },

  // Ingress
  ingress: {
    apiVersion: 'networking.k8s.io/v1',
    kind: 'Ingress',
    metadata: {
      name: 'tigerswap-ingress',
      namespace: 'tigerswap',
      annotations: {
        'nginx.ingress.kubernetes.io/rewrite-target': '/',
      },
    },
    spec: {
      rules: [{
        host: 'api.tigerswap.com',
        http: {
          paths: [{
            path: '/',
            pathType: 'Prefix',
            backend: {
              service: {
                name: 'tigerswap-api',
                port: { number: 80 },
              },
            },
          }],
        },
      }],
    },
  },

  // ConfigMap
  configMap: {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: {
      name: 'tigerswap-config',
      namespace: 'tigerswap',
    },
    data: {
      'database.json': JSON.stringify({
        host: 'postgres.tigerswap.svc.cluster.local',
        port: 5432,
        database: 'tigerswap',
      }),
      'redis.json': JSON.stringify({
        host: 'redis.tigerswap.svc.cluster.local',
        port: 6379,
      }),
    },
  },

  // Secret
  secret: {
    apiVersion: 'v1',
    kind: 'Secret',
    metadata: {
      name: 'tigerswap-secrets',
      namespace: 'tigerswap',
    },
    type: 'Opaque',
    data: {
      'db-password': '', // Base64 encoded
      'api-key': '',    // Base64 encoded
    },
  },

  // Horizontal Pod Autoscaler
  hpa: {
    apiVersion: 'autoscaling/v2',
    kind: 'HorizontalPodAutoscaler',
    metadata: {
      name: 'tigerswap-api-hpa',
      namespace: 'tigerswap',
    },
    spec: {
      scaleTargetRef: {
        apiVersion: 'apps/v1',
        kind: 'Deployment',
        name: 'tigerswap-api',
      },
      minReplicas: 3,
      maxReplicas: 10,
      metrics: [{
        type: 'Resource',
        resource: {
          name: 'cpu',
          target: { type: 'Utilization', averageUtilization: 70 },
        },
      }],
    },
  },

  // PodDisruptionBudget
  pdb: {
    apiVersion: 'policy/v1',
    kind: 'PodDisruptionBudget',
    metadata: {
      name: 'tigerswap-api-pdb',
      namespace: 'tigerswap',
    },
    spec: {
      minAvailable: 2,
      selector: {
        matchLabels: { app: 'tigerswap-api' },
      },
    },
  },

  // ServiceMonitor (Prometheus)
  serviceMonitor: {
    apiVersion: 'monitoring.coreos.com/v1',
    kind: 'ServiceMonitor',
    metadata: {
      name: 'tigerswap-api-monitor',
      namespace: 'tigerswap',
    },
    spec: {
      selector: { matchLabels: { app: 'tigerswap-api' } },
      endpoints: [{
        port: 'metrics',
        path: '/metrics',
      }],
    },
  },
};

// ============================================================================
// Docker Compose
// ============================================================================

export const DOCKER_COMPOSE = {
  version: '3.8',
  services: {
    postgres: {
      image: 'postgres:15',
      environment: {
        POSTGRES_USER: 'tigerswap',
        POSTGRES_PASSWORD: 'tigerswap',
        POSTGRES_DB: 'tigerswap',
      },
      volumes: ['postgres_data:/var/lib/postgresql/data'],
      ports: ['5432:5432'],
    },
    redis: {
      image: 'redis:7-alpine',
      ports: ['6379:6379'],
      volumes: ['redis_data:/data'],
    },
    api: {
      build: './api',
      ports: ['8080:8080'],
      environment: {
        DATABASE_URL: 'postgresql://tigerswap:tigerswap@postgres:5432/tigerswap',
        REDIS_URL: 'redis://redis:6379',
      },
      depends_on: ['postgres', 'redis'],
    },
    frontend: {
      build: './frontend',
      ports: ['3000:3000'],
      environment: {
        NEXT_PUBLIC_API_URL: 'http://localhost:8080',
      },
    },
  },
  volumes: {
    postgres_data: {},
    redis_data: {},
  },
};

// ============================================================================
// Terraform
// ============================================================================

export const TERRAFORM_PROVIDER = `
provider "aws" {
  region = var.region
}

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

variable "region" {
  default = "us-east-1"
}

variable "vpc_cidr" {
  default = "10.0.0.0/16"
}

resource "aws_vpc" "tigerswap" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true
  tags                 = { Name = "tigerswap-vpc" }
}

resource "aws_subnet" "public" {
  count             = 2
  vpc_id            = aws_vpc.tigerswap.id
  cidr_block        = "10.0.${count.index + 1}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true
  tags              = { Name = "tigerswap-public-${count.index + 1}" }
}

resource "aws_instance" "tigerswap" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t3.medium"
  subnet_id     = aws_subnet.public[0].id
  tags          = { Name = "tigerswap-server" }
}
`;

// ============================================================================
// GitHub Actions CI/CD
// ============================================================================

export const GITHUB_WORKFLOW = `
name: CI/CD

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '20'
      - run: npm ci
      - run: npm test

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: docker/setup-buildx-action@v2
      - uses: actions/build-push-action@v4
        with:
          context: .
          push: false

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3
      - uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-access-key-id: \${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: \${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: us-east-1
      - run: |
          kubectl apply -f k8s/
`;

// ============================================================================
// Export
// ============================================================================

export default {
  K8S_MANIFESTS,
  DOCKER_COMPOSE,
  TERRAFORM_PROVIDER,
  GITHUB_WORKFLOW,
};