# Pod Network Access Monitor - Examples

This directory contains example configurations and demo scripts for the Pod Network Monitor.

## Files

### test-pods.yaml
Basic test pods to demonstrate monitoring functionality:
- `test-external-access`: Pod with curl for testing external requests
- `nginx-monitored`: Nginx pod with monitoring enabled
- `nginx-unmonitored`: Nginx pod without monitoring (for comparison)
- `monitored-app`: Deployment with multiple replicas, all monitored

**Usage:**
```bash
kubectl apply -f test-pods.yaml
kubectl get pods -l monitor=external
```

### demo.sh
Interactive demo script that:
1. Deploys test pods
2. Runs various network tests
3. Shows monitor logs for each test
4. Demonstrates monitored vs unmonitored traffic

**Usage:**
```bash
chmod +x demo.sh
./demo.sh
```

### blocking-mode.yaml
Example configuration for blocking external traffic:
- ConfigMap with blocking enabled
- DaemonSet configured to block traffic
- Test pod that will have external access blocked

**Usage:**
```bash
# Deploy blocking configuration
kubectl apply -f blocking-mode.yaml

# Test that external access is blocked
kubectl exec blocked-pod -- curl https://google.com
# Should timeout or fail

# Check logs
kubectl logs -n kube-system -l app=pod-network-monitor-blocking
```

### production-monitoring.yaml
Real-world example for monitoring only production workloads:
- Production deployment (monitored)
- Development deployment (not monitored)
- DaemonSet with label selector for production pods

**Usage:**
```bash
kubectl apply -f production-monitoring.yaml

# Production pods are monitored
kubectl exec -it deployment/production-app -- wget https://example.com

# Development pods are NOT monitored
kubectl exec -it deployment/development-app -- wget https://example.com

# View logs (only production traffic appears)
kubectl logs -n kube-system -l app=pod-network-monitor-prod -f
```

## Quick Test Workflow

1. **Deploy the monitor:**
   ```bash
   cd ..
   ./deploy-minikube.sh
   ```

2. **Run the demo:**
   ```bash
   cd examples
   ./demo.sh
   ```

3. **Watch logs in real-time:**
   ```bash
   kubectl logs -n kube-system -l app=pod-network-monitor -f
   ```

4. **Manually test:**
   ```bash
   # Deploy test pod
   kubectl apply -f test-pods.yaml
   
   # Generate external traffic
   kubectl exec test-external-access -- curl https://google.com
   
   # Check monitor detected it
   kubectl logs -n kube-system -l app=pod-network-monitor --tail=10
   ```

## Expected Output

When traffic is allowed:
```
TRAFFIC: 10.244.0.5:45678 -> 8.8.8.8:53 (UDP) [ALLOWED] [pod: default/test-external-access] [map_lookup=true, flag=0, lookup_ip=0x0af40024]
TRAFFIC: 10.244.0.5:52341 -> 142.250.185.46:443 (TCP) [ALLOWED] [pod: default/test-external-access] [map_lookup=true, flag=0, lookup_ip=0x0af40024]
```

When traffic is blocked:
```
TRAFFIC: 10.244.0.5:45678 -> 8.8.8.8:53 (UDP) [BLOCKED] [pod: default/blocked-pod] [map_lookup=true, flag=1, lookup_ip=0x0af40024]
```

## Cleanup

```bash
# Remove test pods
kubectl delete -f test-pods.yaml

# Remove blocking example
kubectl delete -f blocking-mode.yaml

# Remove production example
kubectl delete -f production-monitoring.yaml
```
