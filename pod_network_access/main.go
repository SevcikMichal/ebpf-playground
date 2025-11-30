package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type NetworkEvent struct {
	Saddr          uint32
	Daddr          uint32
	Sport          uint16
	Dport          uint16
	Protocol       uint8
	Blocked        uint8
	FoundInMap     uint8
	BlockFlagValue uint8
	SaddrLookup    uint32
}

var (
	monitoredPods = make(map[string]string) // IP -> Pod Name
	podsMutex     sync.RWMutex
)

func main() {
	log.Println("Starting Simple Pod Network Monitor")

	// Remove resource limits
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal("Removing memlock:", err)
	}

	// Load eBPF objects
	var objs network_monitorObjects
	if err := loadNetwork_monitorObjects(&objs, nil); err != nil {
		log.Fatal("Loading eBPF objects:", err)
	}
	defer objs.Close()

	// Attach TC to all veth interfaces (ingress to capture pod->host traffic)
	links, err := netlink.LinkList()
	if err != nil {
		log.Fatal("Listing links:", err)
	}

	var tcLinks []link.Link
	attached := 0

	for _, iface := range links {
		if iface.Type() != "veth" {
			continue
		}

		l, err := link.AttachTCX(link.TCXOptions{
			Interface: iface.Attrs().Index,
			Program:   objs.MonitorEgress,
			Attach:    ebpf.AttachTCXIngress,
		})
		if err != nil {
			log.Printf("Warning: Could not attach to %s: %v", iface.Attrs().Name, err)
			continue
		}

		tcLinks = append(tcLinks, l)
		attached++
		log.Printf("Attached to veth: %s", iface.Attrs().Name)
	}

	if attached == 0 {
		log.Fatal("No veth interfaces found or all attachments failed")
	}

	log.Printf("Successfully attached to %d veth interface(s)", attached)

	// Setup Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Getting in-cluster config:", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatal("Creating Kubernetes client:", err)
	}

	// Get pod selector from env
	podSelector := getPodSelector()
	blockExternal := os.Getenv("BLOCK_EXTERNAL") == "true"
	log.Printf("Monitoring pods with labels: %v", podSelector)
	log.Printf("Blocking mode: %v", blockExternal)

	// Start pod monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go monitorPods(ctx, clientset, podSelector, &objs)

	// Open ring buffer for events
	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		log.Fatal("Opening ringbuf reader:", err)
	}
	defer rd.Close()

	log.Println("Monitoring network traffic from pods...")

	// Read events in background
	go func() {
		for {
			record, err := rd.Read()
			if err != nil {
				if err == ringbuf.ErrClosed {
					return
				}
				log.Printf("Reading from ringbuf: %v", err)
				continue
			}

			var event NetworkEvent
			if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
				log.Printf("Parsing event: %v", err)
				continue
			}

			// Convert IPs from network byte order
			srcIP := intToIP(event.Saddr)
			dstIP := intToIP(event.Daddr)

			// Check if source IP is from a monitored pod
			podsMutex.RLock()
			podName, isMonitored := monitoredPods[srcIP]
			podsMutex.RUnlock()

			if !isMonitored {
				continue // Skip non-monitored pods
			}

			proto := protoName(event.Protocol)
			action := "ALLOWED"
			if event.Blocked == 1 {
				action = "BLOCKED"
			}

			// Debug info
			debugInfo := fmt.Sprintf("[map_lookup=%v, flag=%d, lookup_ip=0x%08x]",
				event.FoundInMap == 1, event.BlockFlagValue, event.SaddrLookup)

			if event.Sport > 0 && event.Dport > 0 {
				log.Printf("[%s] %s: %s -> %s  [%s] %d -> %d %s",
					podName, action, srcIP, dstIP, proto, event.Sport, event.Dport, debugInfo)
			} else {
				log.Printf("[%s] %s: %s -> %s  [%s] %s", podName, action, srcIP, dstIP, proto, debugInfo)
			}
		}
	}()

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")

	// Cleanup links
	for _, l := range tcLinks {
		l.Close()
	}
}

func intToIP(ip uint32) string {
	// IP is in network byte order (big endian)
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
}

func protoName(proto uint8) string {
	switch proto {
	case 1:
		return "ICMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	default:
		return fmt.Sprintf("proto-%d", proto)
	}
}

func getPodSelector() map[string]string {
	selector := make(map[string]string)
	envSelector := os.Getenv("POD_SELECTOR")
	if envSelector == "" {
		// Default: monitor pods with label monitor=external
		selector["monitor"] = "external"
	} else {
		pairs := strings.Split(envSelector, ",")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				selector[kv[0]] = kv[1]
			}
		}
	}
	return selector
}

func monitorPods(ctx context.Context, clientset *kubernetes.Clientset, selector map[string]string, objs *network_monitorObjects) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Initial update
	updateMonitoredPods(clientset, selector, objs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updateMonitoredPods(clientset, selector, objs)
		}
	}
}

func updateMonitoredPods(clientset *kubernetes.Clientset, selector map[string]string, objs *network_monitorObjects) {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName, _ = os.Hostname()
	}

	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	}

	if len(selector) > 0 {
		listOptions.LabelSelector = labels.SelectorFromSet(selector).String()
	}

	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), listOptions)
	if err != nil {
		log.Printf("Error listing pods: %v", err)
		return
	}

	newPods := make(map[string]string)
	blockExternal := os.Getenv("BLOCK_EXTERNAL") == "true"

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
			continue
		}

		newPods[pod.Status.PodIP] = pod.Name

		// Add IP to eBPF blocking map
		ipAddr := net.ParseIP(pod.Status.PodIP)
		if ipAddr == nil {
			continue
		}
		ipv4 := ipAddr.To4()
		if ipv4 == nil {
			continue
		}
		// Use LittleEndian to match how kernel sees IP in ip->saddr on x86
		ipInt := binary.LittleEndian.Uint32(ipv4)

		// Add to blocked_pods map
		var blockFlag uint8 = 0
		if blockExternal {
			blockFlag = 1
		}
		if err := objs.BlockedPods.Put(&ipInt, &blockFlag); err != nil {
			log.Printf("Error adding pod %s to blocking map: %v", pod.Name, err)
		} else {
			mode := "monitor"
			if blockFlag == 1 {
				mode = "BLOCK"
			}
			log.Printf("Added pod %s (IP: %s, uint32: 0x%08x) in %s mode", pod.Name, pod.Status.PodIP, ipInt, mode)
		}
	}

	podsMutex.Lock()
	monitoredPods = newPods
	podsMutex.Unlock()

	log.Printf("Now monitoring %d pod(s) - Block mode: %v", len(newPods), blockExternal)
}
