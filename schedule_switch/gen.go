package main

//go:generate go tool bpf2go -tags linux sched_switch sched_switch.c
