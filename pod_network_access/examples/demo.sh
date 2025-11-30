#!/bin/bash
set -e

echo "Running Pod Network Monitor Demo"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if monitor is deployed
if ! kubectl get daemonset -n kube-system pod-network-monitor > /dev/null 2>&1; then
    echo -e "${RED}Pod Network Monitor is not deployed${NC}"
    echo "Run ./deploy-minikube.sh first"
    exit 1
fi

echo -e "${GREEN}Pod Network Monitor is deployed${NC}"
echo ""

# Deploy test pods
echo -e "${BLUE}Deploying test pods...${NC}"
kubectl apply -f examples/test-pods.yaml

echo ""
echo -e "${YELLOW}Waiting for pods to be ready...${NC}"
kubectl wait --for=condition=ready pod/test-external-access --timeout=60s 2>/dev/null || true
kubectl wait --for=condition=ready pod/nginx-monitored --timeout=60s 2>/dev/null || true

echo ""
echo -e "${GREEN}Test pods deployed${NC}"
echo ""

# Show which pods are monitored
echo -e "${BLUE}Pods with monitoring enabled (label: monitor=external):${NC}"
kubectl get pods -l monitor=external -o wide
echo ""

echo -e "${BLUE}Pods without monitoring:${NC}"
kubectl get pods -l '!monitor' -o wide
echo ""

# Function to run test and show logs
run_test() {
    local test_name=$1
    local command=$2
    
    echo ""
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${BLUE}Test: ${test_name}${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    echo -e "${BLUE}Command:${NC} $command"
    echo ""
    
    # Execute command
    eval "$command" &
    local cmd_pid=$!
    
    # Give it a moment to execute
    sleep 2
    
    # Show recent logs
    echo -e "${GREEN}Monitor logs (last 5 entries):${NC}"
    kubectl logs -n kube-system -l app=pod-network-monitor --tail=5 2>/dev/null | grep -E "EXTERNAL ACCESS|Pod=" || echo "No events captured yet"
    
    # Wait for command to finish
    wait $cmd_pid 2>/dev/null || true
    
    sleep 1
}

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Starting Network Monitoring Tests     ${NC}"
echo -e "${GREEN}========================================${NC}"

# Test 1: Ping external IP (Google DNS)
run_test "Ping External IP (8.8.8.8)" \
    "kubectl exec test-external-access -- ping -c 3 8.8.8.8"

# Test 2: DNS lookup to external DNS server
run_test "DNS Lookup to External DNS" \
    "kubectl exec test-external-access -- nslookup google.com 8.8.8.8"

# Test 3: HTTP request to external IP
run_test "HTTP Request to External Site (by IP)" \
    "kubectl exec test-external-access -- curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 http://1.1.1.1"

# Test 4: UDP connection using nc (netcat)
run_test "UDP Connection to External IP:Port (DNS)" \
    "kubectl exec test-external-access -- timeout 3 nc -u -v 8.8.8.8 53 || true"

# Test 5: Internal cluster DNS (should NOT be logged)
run_test "Internal DNS Query (should NOT trigger alert)" \
    "kubectl exec test-external-access -- nslookup kubernetes.default.svc.cluster.local"

# Test 6: Ping internal pod IP (should NOT be logged)
run_test "Ping Internal Pod IP (should NOT trigger alert)" \
    "kubectl exec test-external-access -- ping -c 2 \$(kubectl get pod nginx-unmonitored -o jsonpath='{.status.podIP}') || echo 'Ping completed'"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Testing Blocking Mode                 ${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

echo -e "${YELLOW}Enabling blocking mode...${NC}"
kubectl set env daemonset/pod-network-monitor -n kube-system BLOCK_EXTERNAL=true 2>/dev/null
echo -e "${YELLOW}Waiting for pods to restart...${NC}"
sleep 5
kubectl wait --for=condition=ready pod -n kube-system -l app=pod-network-monitor --timeout=30s 2>/dev/null || true
sleep 2

echo ""
echo -e "${BLUE}Testing with blocking enabled:${NC}"
echo ""

# Test 7: Ping should be BLOCKED
run_test "Ping External IP (Should be BLOCKED)" \
    "kubectl exec test-external-access -- timeout 5 ping -c 3 8.8.8.8 || echo 'Connection blocked/timed out'"

# Test 8: HTTP should be BLOCKED
run_test "HTTP Request (Should be BLOCKED)" \
    "kubectl exec test-external-access -- timeout 5 curl -s -o /dev/null -w '%{http_code}' http://1.1.1.1 || echo 'Connection blocked/timed out'"

echo ""
echo -e "${YELLOW}Disabling blocking mode (restoring monitor-only)...${NC}"
kubectl set env daemonset/pod-network-monitor -n kube-system BLOCK_EXTERNAL=false 2>/dev/null
echo -e "${YELLOW}Waiting for pods to restart...${NC}"
sleep 5
kubectl wait --for=condition=ready pod -n kube-system -l app=pod-network-monitor --timeout=30s 2>/dev/null || true

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Test Summary                          ${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

echo -e "${BLUE}All monitor logs:${NC}"
kubectl logs -n kube-system -l app=pod-network-monitor --tail=20 | grep -E "EXTERNAL ACCESS|Pod=" || echo "No events captured"

echo ""
echo -e "${GREEN}Demo complete!${NC}"
echo ""
echo -e "${YELLOW}Tips:${NC}"
echo "  - Watch logs in real-time: kubectl logs -n kube-system -l app=pod-network-monitor -f"
echo "  - List monitored pods: kubectl get pods -l monitor=external"
echo "  - Cleanup: kubectl delete -f examples/test-pods.yaml"
echo ""
