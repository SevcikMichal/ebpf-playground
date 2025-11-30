# Pod Network Access Monitor

An eBPF-based Kubernetes daemon that monitors and optionally blocks external network access from specific pods.

## Features

- **Monitor External Access**: Detects when pods try to reach external networks (internet)
- **Label-based Selection**: Monitor specific pods using Kubernetes label selectors
- **Optional Blocking**: Can block external traffic while allowing internal cluster communication
- **High Performance**: Uses eBPF for minimal overhead
- **Detailed Logging**: Logs source/destination IPs, ports, and protocols

## How It Works

1. **Pod Selection**: The daemon watches for pods matching specified labels on each node
2. **Network Namespace Tracking**: Identifies each pod's network namespace
3. **Traffic Classification**: Uses configurable CIDR blocks to distinguish internal vs external traffic
4. **eBPF Hooks**: Attaches TC (Traffic Control) eBPF programs to pod network interfaces
5. **Event Reporting**: Logs external access attempts with full details

## Prerequisites

- Kubernetes cluster with kernel >= 5.10
- BTF (BPF Type Format) enabled kernel
- Privileged DaemonSet permissions
- Go 1.24.0+ and clang/llvm for building

## Quick Start with Minikube

```bash
# Start Minikube
minikube start --cpus=4 --memory=4096

# Build and deploy (automatically builds locally, then creates Docker image)
./deploy-minikube.sh

# Run interactive demo
cd examples
./demo.sh
```

See [MINIKUBE.md](MINIKUBE.md) for detailed instructions.

## Installation

### 1. Build Locally and Create Docker Image

The build process compiles the eBPF code and Go binary locally (where you have kernel headers), then packages it into a Docker image:

```bash
cd pod_network_access

# Option 1: Use the build script
./build.sh

# Option 2: Manual build
go generate ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pod_network_access .
docker build -t pod-network-monitor:latest .
```

### 2. Deploy to Kubernetes

```bash
kubectl apply -f k8s-daemonset.yaml
```

### 3. Configure Monitoring

Edit the `POD_SELECTOR` environment variable in `k8s-daemonset.yaml`:

```yaml
- name: POD_SELECTOR
  value: "monitor=external,environment=production"
```

### 4. Label Pods to Monitor

```bash
kubectl label pod <pod-name> monitor=external
```

## Configuration

### Environment Variables

- **`NODE_NAME`**: Auto-populated with the node name (required)
- **`POD_SELECTOR`**: Comma-separated label selector (e.g., `"monitor=external,env=prod"`)
- **`BLOCK_EXTERNAL`**: Set to `"true"` to block external traffic (default: `"false"`)
- **`INTERNAL_CIDRS`**: Comma-separated list of internal network CIDRs (default: `"10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"`)

### Example: Monitor All Pods with Specific Label

```yaml
env:
- name: POD_SELECTOR
  value: "app=web"
- name: BLOCK_EXTERNAL
  value: "false"
```

### Example: Block External Access

```yaml
env:
- name: POD_SELECTOR
  value: "security=strict"
- name: BLOCK_EXTERNAL
  value: "true"
```

## Usage Examples

### Monitor Specific Pods

```bash
# Label a pod for monitoring
kubectl label pod nginx-pod monitor=external

# Check logs
kubectl logs -n kube-system -l app=pod-network-monitor
```

### Expected Output

```
2024/11/29 20:50:57 Starting Pod Network Monitor on node: worker-1
2024/11/29 20:50:57 Monitoring pods with labels: map[monitor:external]
2024/11/29 20:50:57 Block external traffic: false
2024/11/29 20:50:57 Internal CIDRs: [10.0.0.0/8 172.16.0.0/12 192.168.0.0/16]
2024/11/29 20:50:57 Added internal network: 10.0.0.0/8
2024/11/29 20:50:57 Added internal network: 172.16.0.0/12
2024/11/29 20:50:57 Added internal network: 192.168.0.0/16
2024/11/29 20:50:57 Pod Network Monitor started successfully
2024/11/29 20:51:02 Found 2 pods matching selector
2024/11/29 20:51:02 Now monitoring pod: nginx-pod (netns: 4026532545)
2024/11/29 20:51:15 ⚠️  EXTERNAL ACCESS ALLOWED: Pod=nginx-pod Src=10.244.1.5:45678 Dst=8.8.8.8:53 Proto=UDP
2024/11/29 20:51:16 ⚠️  EXTERNAL ACCESS ALLOWED: Pod=nginx-pod Src=10.244.1.5:52341 Dst=142.250.185.46:443 Proto=TCP
```

## Architecture

### Components

1. **network_monitor.c**: eBPF program that hooks into TC egress
2. **main.go**: Go daemon that manages eBPF programs and Kubernetes integration
3. **k8s-daemonset.yaml**: Kubernetes DaemonSet for deployment

### Traffic Flow

```
Pod → veth interface → TC egress hook (eBPF) → Check if external → Log/Block
```

### Internal vs External Detection

Traffic is considered **internal** if the destination IP matches any configured CIDR:
- Default: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`
- Configurable via `INTERNAL_CIDRS`

All other traffic is considered **external** and triggers alerts.

## Security Considerations

- Requires privileged DaemonSet (for eBPF and network namespace access)
- Uses host network and PID namespace
- Minimal RBAC permissions (read-only pod access)
- No credential storage required

## Limitations

- Currently supports IPv4 only (IPv6 support can be added)
- TC attachment requires finding correct veth interface (simplified in current implementation)
- Pod PID discovery requires container runtime integration (placeholder implementation)

## Development

### Generate eBPF Code

```bash
go generate ./...
```

### Build Locally

```bash
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pod_network_access .
```

### Test Locally (requires root)

```bash
sudo ./pod_network_access
```

**Note:** The binary is built locally because it requires kernel headers and BPF tooling. The Dockerfile simply packages the pre-built binary into a minimal Alpine image.

## Troubleshooting

### No events appearing

1. Check pod labels match selector: `kubectl get pods --show-labels`
2. Verify DaemonSet is running: `kubectl get ds -n kube-system pod-network-monitor`
3. Check logs: `kubectl logs -n kube-system -l app=pod-network-monitor`

### Permission errors

Ensure the DaemonSet has `privileged: true` and required capabilities.

### Cannot find veth interface

The current implementation uses a simplified veth discovery. In production, integrate with container runtime (containerd/CRI-O) to get actual veth pairs.

## Future Enhancements

- [ ] IPv6 support
- [ ] Container runtime integration for accurate veth discovery
- [ ] Prometheus metrics export
- [ ] Network policy integration
- [ ] Webhook for dynamic configuration
- [ ] Support for allow-listing specific external IPs

## License

Dual MIT/GPL
