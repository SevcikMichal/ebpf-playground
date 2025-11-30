#!/bin/bash
set -e

echo "Building and deploying Pod Network Monitor to Minikube"
echo ""

# Check if minikube is running
if ! minikube status > /dev/null 2>&1; then
    echo "Minikube is not running. Start it with: minikube start"
    exit 1
fi

echo "Minikube is running"
echo ""

# Generate eBPF code and build locally
echo "Generating eBPF code..."
go generate ./...

echo "Building Go binary locally..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pod_network_access .

echo "Binary built successfully"
echo ""

# Set Docker environment to use Minikube's Docker daemon
echo "Setting up Minikube Docker environment..."
eval $(minikube docker-env)

# Build the Docker image inside Minikube (will copy pre-built binary)
echo "Building Docker image..."
docker build -t pod-network-monitor:latest .

echo "Docker image built successfully"
echo ""

# Deploy to Kubernetes
echo "Deploying to Kubernetes..."
kubectl apply -f k8s-daemonset.yaml

echo "Deployed successfully"
echo ""

# Wait for pods to be ready
echo "Waiting for DaemonSet to be ready..."
kubectl rollout status daemonset/pod-network-monitor -n kube-system --timeout=60s

echo ""
echo "Deployment complete!"
echo ""
echo "Next steps:"
echo ""
echo "1. Label a pod to monitor:"
echo "   kubectl label pod <pod-name> monitor=external"
echo ""
echo "2. View logs:"
echo "   kubectl logs -n kube-system -l app=pod-network-monitor -f"
echo ""
echo "3. Create a test pod:"
echo "   kubectl run test-nginx --image=nginx --labels=monitor=external"
echo ""
echo "4. Test external access from the pod:"
echo "   kubectl exec test-nginx -- curl -I https://google.com"
echo ""
