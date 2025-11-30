# Makefile for Pod Network Monitor

.PHONY: all generate build docker-build deploy clean test

all: generate build

# Generate eBPF code from C
generate:
	go generate ./...

# Build the Go binary for Linux
build: generate
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o pod_network_access .

# Build Docker image (requires binary to be built first)
docker-build: build
	docker build -t pod-network-monitor:latest .

# Complete build (local binary + Docker image)
build-all: build docker-build

# Deploy to Kubernetes
deploy:
	kubectl apply -f k8s-daemonset.yaml

# Remove deployment
undeploy:
	kubectl delete -f k8s-daemonset.yaml

# Clean generated files
clean:
	rm -f pod_network_access pod-network-monitor
	rm -f network_monitor_bpfeb.go network_monitor_bpfel.go
	rm -f network_monitor_bpfeb.o network_monitor_bpfel.o

# Run tests
test:
	go test -v ./...

# View logs
logs:
	kubectl logs -n kube-system -l app=pod-network-monitor -f

# Get module dependencies
deps:
	go mod download
	go mod tidy
