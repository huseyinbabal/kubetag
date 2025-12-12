# KubeTag

Track Docker images across your Kubernetes cluster with a simple web interface.

## Features

- Collects container images from Deployments, DaemonSets, and CronJobs
- Clean, dark-mode UI built with ShadCN styling
- Filter by namespace
- Real-time statistics
- Prometheus metrics endpoint for monitoring
- Image version history tracking
- No React - pure vanilla JavaScript

## Quick Start

### Prerequisites

- Go 1.21+
- Access to a Kubernetes cluster
- `kubectl` configured

### Run Locally

```bash
# Clone the repository
git clone https://github.com/huseyinbabal/kubetag.git
cd kubetag

# Install dependencies
go mod download

# Run the server
go run cmd/server/main.go
```

Visit `http://localhost:8080` in your browser.

### Docker

```bash
docker build -t kubetag .
docker run -p 8080:8080 -v ~/.kube/config:/root/.kube/config kubetag
```

### Deploy to Kubernetes

```bash
kubectl apply -f k8s/
```

## API

### GET `/api/images`

Fetch all container images from the cluster.

**Query Parameters:**
- `namespace` (optional) - Filter by namespace

**Response:**
```json
{
  "images": [
    {
      "name": "nginx",
      "tag": "1.21",
      "resourceType": "Deployment",
      "resourceName": "my-app",
      "namespace": "default",
      "containers": ["web"]
    }
  ],
  "total": 1
}
```

### GET `/api/images/:name/history`

Get the version history for a specific image.

**Response:**
```json
{
  "image_name": "nginx",
  "tags": [
    {
      "tag": "1.21",
      "first_seen": "2024-01-01T00:00:00Z",
      "last_seen": "2024-01-02T00:00:00Z",
      "resource_type": "Deployment",
      "resource_name": "my-app",
      "namespace": "default",
      "container": "web",
      "active": true
    }
  ]
}
```

## Prometheus Metrics

KubeTag exposes Prometheus metrics at `/metrics` endpoint.

### Available Metrics

- `kubetag_image_info` - Information about container images running in the cluster
  - Labels: `image_name`, `tag`, `repository`, `resource_type`, `resource_name`, `namespace`, `container`
  
- `kubetag_image_tag_info` - Detailed information about image tags
  - Labels: `image_name`, `tag`, `resource_type`, `resource_name`, `namespace`
  
- `kubetag_image_version_count` - Count of different versions per image
  - Labels: `image_name`, `namespace`

### Prometheus Configuration

Add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'kubetag'
    static_configs:
      - targets: ['kubetag:8080']
    metrics_path: '/metrics'
```

For Kubernetes ServiceMonitor (if using Prometheus Operator):

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kubetag
  namespace: default
spec:
  selector:
    matchLabels:
      app: kubetag
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

## Configuration

Environment variables:
- `PORT` - Server port (default: 8080)
- `WATCH_NAMESPACES` - Namespaces to watch, comma-separated or "*" for all (default: "*")

## License

MIT
