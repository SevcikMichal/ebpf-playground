//go:build ignore

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/in.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// Event structure for userspace notifications
struct network_event {
    __u32 saddr;      // Source IP (IPv4)
    __u32 daddr;      // Destination IP (IPv4)
    __u16 sport;      // Source port
    __u16 dport;      // Destination port
    __u8 protocol;    // Protocol (TCP/UDP/etc)
    __u8 blocked;     // Whether packet was blocked
    __u8 found_in_map; // Whether IP was found in blocked_pods map
    __u8 block_flag_value; // Value of block flag if found
    __u32 saddr_lookup; // The IP value used for map lookup (for debugging)
};

// Map: pod IP -> block flag (1 = block, 0 = monitor only)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);   // Pod IP address
    __type(value, __u8);  // Block flag
} blocked_pods SEC(".maps");

// Ring buffer for events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

// Simple TC hook - captures all IPv4 traffic from pods
SEC("tc")
int monitor_egress(struct __sk_buff *skb) {
    void *data_end = (void *)(long)skb->data_end;
    void *data = (void *)(long)skb->data;
    
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) {
        return TC_ACT_OK;
    }
    
    // Only process IPv4
    if (eth->h_proto != bpf_htons(ETH_P_IP)) {
        return TC_ACT_OK;
    }
    
    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end) {
        return TC_ACT_OK;
    }
    
    // Create event for every packet
    struct network_event *event;
    event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) {
        return TC_ACT_OK;
    }
    
    event->saddr = ip->saddr;
    event->daddr = ip->daddr;
    event->protocol = ip->protocol;
    event->sport = 0;
    event->dport = 0;
    event->found_in_map = 0;
    event->block_flag_value = 0;
    event->saddr_lookup = ip->saddr;
    
    // Try to get port numbers for TCP/UDP
    if (ip->protocol == IPPROTO_TCP || ip->protocol == IPPROTO_UDP) {
        void *transport = (void *)ip + (ip->ihl * 4);
        if (transport + 4 <= data_end) {
            __u16 *ports = transport;
            event->sport = bpf_ntohs(ports[0]);
            event->dport = bpf_ntohs(ports[1]);
        }
    }
    
    // Check if we should block this pod's traffic
    __u8 *block_flag = bpf_map_lookup_elem(&blocked_pods, &ip->saddr);
    if (block_flag) {
        event->found_in_map = 1;
        event->block_flag_value = *block_flag;
        if (*block_flag == 1) {
            event->blocked = 1;
            bpf_ringbuf_submit(event, 0);
            return TC_ACT_SHOT; // Drop packet at kernel level
        }
    }
    
    event->blocked = 0;
    bpf_ringbuf_submit(event, 0);
    
    return TC_ACT_OK;
}

char __license[] SEC("license") = "Dual MIT/GPL";
