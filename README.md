# KubeTag

Track Docker images across your Kubernetes cluster with a simple web interface.

## Features

- Collects container images from Deployments, DaemonSets, and CronJobs
- Clean, dark-mode UI built with ShadCN styling
- Filter by namespace
- Real-time statistics
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

## Configuration

Set the `PORT` environment variable to change the server port (default: 8080).

## License

MIT
