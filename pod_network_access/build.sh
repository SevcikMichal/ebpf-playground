#!/bin/bash
set -e

echo "Building Pod Network Monitor..."
echo ""

# Generate eBPF code
echo "Generating eBPF code..."
go generate ./...

# Build the Go binary locally
echo "Building Go binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pod_network_access .

echo "Binary built successfully"
echo ""

# Build Docker image (will copy the pre-built binary)
echo "Building Docker image..."
docker build -t pod-network-monitor:latest .

echo "Docker image built successfully"

# Optional: Push to registry
# docker tag pod-network-monitor:latest your-registry/pod-network-monitor:latest
# docker push your-registry/pod-network-monitor:latest

echo ""
echo "To deploy to Kubernetes:"
echo "  kubectl apply -f k8s-daemonset.yaml"
echo ""
echo "To label a pod for monitoring:"
echo "  kubectl label pod <pod-name> monitor=external"
echo ""
echo "To view logs:"
echo "  kubectl logs -n kube-system -l app=pod-network-monitor -f"
