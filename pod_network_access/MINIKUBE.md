# Running with Minikube

## Prerequisites

```bash
# Install Minikube if not already installed
# See: https://minikube.sigs.k8s.io/docs/start/

# Start Minikube with enough resources
minikube start --cpus=4 --memory=4096
```

## Quick Deploy

```bash
# Make the deploy script executable
chmod +x deploy-minikube.sh

# Build and deploy
./deploy-minikube.sh
```

## Manual Steps

If you prefer to do it manually:

### 1. Build locally

The eBPF code and Go binary must be built on the host (where you have kernel headers):

```bash
# Generate eBPF code
go generate ./...

# Build the binary for Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pod_network_access .
```

### 2. Configure Docker to use Minikube's daemon

```bash
eval $(minikube docker-env)
```

### 3. Build the Docker image

This will package the pre-built binary into a minimal Alpine image:

```bash
docker build -t pod-network-monitor:latest .
```

### 4. Deploy to Kubernetes

```bash
kubectl apply -f k8s-daemonset.yaml
```

### 4. Verify deployment

```bash
kubectl get daemonset -n kube-system pod-network-monitor
kubectl get pods -n kube-system -l app=pod-network-monitor
```

## Testing

### Create a test pod with monitoring enabled

```bash
kubectl run test-nginx --image=nginx --labels=monitor=external
```

### Wait for pod to be running

```bash
kubectl wait --for=condition=ready pod/test-nginx --timeout=60s
```

### Watch the monitor logs

```bash
kubectl logs -n kube-system -l app=pod-network-monitor -f
```

### Generate external traffic from the test pod

In another terminal:

```bash
# DNS lookup (external)
kubectl exec test-nginx -- nslookup google.com

# HTTP request (external)
kubectl exec test-nginx -- curl -I https://google.com

# Check internal cluster DNS (should NOT be logged)
kubectl exec test-nginx -- nslookup kubernetes.default.svc.cluster.local
```

You should see logs like:
```
TRAFFIC: 10.244.0.5:45678 -> 8.8.8.8:53 (UDP) [ALLOWED] [pod: default/test-nginx]
TRAFFIC: 10.244.0.5:52341 -> 142.250.185.46:443 (TCP) [ALLOWED] [pod: default/test-nginx]
```

## Enable Blocking Mode

To test blocking external traffic:

```bash
# Edit the DaemonSet
kubectl edit daemonset -n kube-system pod-network-monitor

# Change BLOCK_EXTERNAL from "false" to "true"
# Save and exit

# Wait for rollout
kubectl rollout status daemonset/pod-network-monitor -n kube-system

# Now external traffic from monitored pods will be blocked
kubectl exec test-nginx -- curl -I https://google.com
# This should timeout or fail
```

## Cleanup

```bash
# Delete test pod
kubectl delete pod test-nginx

# Delete DaemonSet
kubectl delete -f k8s-daemonset.yaml

# Stop Minikube (optional)
minikube stop
```

## Troubleshooting

### Pods not starting

```bash
# Check events
kubectl describe daemonset -n kube-system pod-network-monitor

# Check pod logs
kubectl logs -n kube-system -l app=pod-network-monitor
```

### Build errors

Make sure you have the required tools:
```bash
# Check Go version
go version  # Should be 1.24.0 or higher

# Check for required tools
which clang llvm-strip

# Install if missing (Ubuntu/Debian)
sudo apt-get install clang llvm
```

### Image pull errors

Make sure you're using Minikube's Docker daemon and the binary is built:
```bash
# Check if binary exists
ls -lh pod_network_access

# Verify Docker environment
eval $(minikube docker-env)
docker images | grep pod-network-monitor
```

### No events being logged

1. Verify the pod has the correct label:
   ```bash
   kubectl get pod test-nginx --show-labels
   ```

2. Check the monitor configuration:
   ```bash
   kubectl get daemonset -n kube-system pod-network-monitor -o yaml | grep -A 10 "env:"
   ```

3. Check the pod IPs and verify they're being tracked:
   ```bash
   kubectl get pods -o wide
   kubectl logs -n kube-system -l app=pod-network-monitor | grep "Found monitored pod"
   ```

### Permission issues

The DaemonSet requires privileged mode and specific capabilities. Ensure your Minikube setup supports this:
```bash
minikube start --extra-config=kubelet.allowed-unsafe-sysctls=net.*
```
