# Pod Network Access Monitor

An eBPF-based Kubernetes daemon that monitors and optionally blocks network traffic from specific pods.

## Features

- **Monitor Pod Traffic**: Captures all network traffic from labeled pods
- **Label-based Selection**: Monitor specific pods using Kubernetes label selectors
- **Optional Blocking**: Can block traffic at kernel level while traffic is being monitored
- **High Performance**: Uses eBPF TC hooks for minimal overhead
- **Detailed Logging**: Logs source/destination IPs, ports, protocols, and blocking status

## How It Works

1. **Pod Selection**: The daemon watches for pods matching specified labels (default: `monitor=external`)
2. **Pod IP Tracking**: Tracks pod IPs and stores them in eBPF map with blocking flags
3. **TC Hook Attachment**: Attaches TC ingress eBPF programs to all veth interfaces
4. **Traffic Capture**: Monitors all IPv4 packets from tracked pod IPs
5. **Optional Blocking**: Drops packets at kernel level when blocking mode is enabled
6. **Event Reporting**: Sends events to userspace for logging with debug information

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
2024/11/29 20:50:57 Starting Pod Network Access Monitor...
2024/11/29 20:50:57 Pod monitoring enabled with selector: monitor=external
2024/11/29 20:50:57 External blocking: DISABLED (monitoring only)
2024/11/29 20:50:57 Pod Network Monitor started successfully
2024/11/29 20:51:02 Found 2 pods matching selector
2024/11/29 20:51:02 Found monitored pod: default/nginx-pod (10.244.1.5)
2024/11/29 20:51:15 TRAFFIC: 10.244.1.5:45678 -> 8.8.8.8:53 (UDP) [ALLOWED] [pod: default/nginx-pod] [map_lookup=true, flag=0, lookup_ip=0x0af40024]
2024/11/29 20:51:16 TRAFFIC: 10.244.1.5:52341 -> 142.250.185.46:443 (TCP) [ALLOWED] [pod: default/nginx-pod] [map_lookup=true, flag=0, lookup_ip=0x0af40024]
```

## Architecture

### Components

1. **network_monitor.c**: eBPF program that hooks into TC egress
2. **main.go**: Go daemon that manages eBPF programs and Kubernetes integration
3. **k8s-daemonset.yaml**: Kubernetes DaemonSet for deployment

### Traffic Flow

```
Pod → veth interface → TC ingress hook (eBPF) → Check blocked_pods map → Log/Block
```

### How Blocking Works

1. **Pod Discovery**: Daemon watches Kubernetes API for pods matching label selector
2. **IP Tracking**: Pod IPs stored in eBPF hash map with blocking flag (0=allow, 1=block)
3. **TC Hook**: Attached to all veth interfaces using TCX ingress (captures pod egress traffic)
4. **Packet Processing**: eBPF program looks up source IP in map, drops if flag=1
5. **Event Logging**: All packets generate events with debug info sent via ring buffer

### Byte Order Note

The implementation uses little-endian byte order for IP addresses to match x86 kernel representation. This ensures map lookups succeed between kernel eBPF code and Go userspace.

## Security Considerations

- Requires privileged DaemonSet (for eBPF and network access)
- Uses host network and PID namespace
- Minimal RBAC permissions (read-only pod access)
- No credential storage required
- Blocking happens at kernel level before routing

## Limitations

- Currently supports IPv4 only (IPv6 support can be added)
- TC hooks attached to all veth interfaces (captures all pod traffic on node)
- Requires kernel >= 5.10 with BTF support

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

### Debug information shows wrong IPs

Check the `lookup_ip` field in logs - it shows the IP in hex as seen by kernel. On x86, IPs are little-endian.

### Blocking not working

1. Verify `BLOCK_EXTERNAL=true` environment variable is set
2. Check map lookups succeed: `map_lookup=true` in logs
3. Verify blocking flag: `flag=1` when blocking is enabled
4. Ensure TC hooks are attached: check daemon startup logs

## Future Enhancements

- [ ] IPv6 support
- [ ] Per-destination IP blocking rules
- [ ] Prometheus metrics export
- [ ] Network policy integration
- [ ] Webhook for dynamic configuration
- [ ] Time-based blocking windows

## License

Dual MIT/GPL
