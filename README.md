# eBPF Playground

A collection of eBPF (Extended Berkeley Packet Filter) projects for learning and experimenting with Linux kernel observability and networking.

## Projects

### 1. Counter (`counter/`)
A simple eBPF program that demonstrates basic tracepoint hooking and event counting. Good starting point for understanding eBPF fundamentals.

### 2. Schedule Switch Monitor (`schedule_switch/`)
An eBPF program that hooks into the `sched_switch` tracepoint to monitor process context switches in the Linux kernel. Useful for understanding CPU scheduling behavior.

### 3. Pod Network Access Monitor (`pod_network_access/`)
A production-ready Kubernetes daemon that uses eBPF TC (Traffic Control) hooks to:
- Monitor network traffic from specific pods based on label selectors
- Optionally block traffic at kernel level using TC_ACT_SHOT
- Run as a DaemonSet across K8s cluster nodes
- Provide detailed logging with debug information

This is the most complete project with full K8s integration, Docker packaging, and deployment automation.

## Prerequisites

- Linux kernel >= 5.10 with BTF (BPF Type Format) enabled
- Go 1.24.0 or later
- clang/llvm for compiling eBPF code
- Kernel headers installed
- For K8s projects: Minikube or any Kubernetes cluster

## Quick Start

Each project directory contains its own README with specific instructions. Generally:

```bash
cd <project-directory>

# Generate eBPF code
go generate ./...

# Build
go build

# Run (requires root privileges)
sudo ./<binary-name>
```

For the Kubernetes project:
```bash
cd pod_network_access
./deploy-minikube.sh
```

## Technology Stack

- **eBPF/BPF**: Linux kernel programmability
- **cilium/ebpf**: Go library for eBPF (v0.12.3)
- **Kubernetes**: Container orchestration with client-go
- **TC (Traffic Control)**: Network packet processing with TCX hooks
- **Go**: Primary development language

## Learning Path

1. Start with `counter/` to understand basic eBPF concepts
2. Progress to `schedule_switch/` for tracepoint monitoring
3. Explore `pod_network_access/` for production-grade networking and K8s integration

## Disclaimer

**A significant portion of the code in this repository was generated and developed with the assistance of GitHub Copilot and AI coding assistants.** The projects serve as learning examples and experimentation platforms. While the `pod_network_access` project is designed with production patterns in mind, all code should be thoroughly reviewed and tested before use in any critical environment.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

This is a personal learning repository. Feel free to fork and experiment!

## Resources

- [eBPF Documentation](https://ebpf.io/)
- [Cilium eBPF Go Library](https://github.com/cilium/ebpf)
- [BPF and XDP Reference Guide](https://docs.cilium.io/en/stable/bpf/)
- [Linux Kernel Tracepoints](https://www.kernel.org/doc/html/latest/trace/tracepoints.html)
