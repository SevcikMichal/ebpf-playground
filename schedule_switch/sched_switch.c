//go:build ignore

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

#define TASK_COMM_LEN 16

struct sched_switch_args {
    unsigned long long pad;
    char prev_comm[TASK_COMM_LEN];
    int prev_pid;
    int prev_prio;
    long long prev_state;
    char next_comm[TASK_COMM_LEN];
    int next_pid;
    int next_prio;
};

struct event {
    int prev_pid;
    int next_pid;
    char prev_comm[TASK_COMM_LEN];
    char next_comm[TASK_COMM_LEN];
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256 KB ring buffer
} events SEC(".maps");

SEC("tracepoint/sched/sched_switch")
int handle_sched_switch(struct sched_switch_args *ctx) {
    struct event *e;
    
    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) {
        return 0;
    }
    
    e->prev_pid = ctx->prev_pid;
    e->next_pid = ctx->next_pid;
    
    __builtin_memcpy(&e->prev_comm, ctx->prev_comm, TASK_COMM_LEN);
    __builtin_memcpy(&e->next_comm, ctx->next_comm, TASK_COMM_LEN);
    
    bpf_ringbuf_submit(e, 0);
    
    return 0;
}

char __license[] SEC("license") = "Dual MIT/GPL";
