package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"os/signal"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

const TaskCommLen = 16

type Event struct {
	PrevPID  int32
	NextPID  int32
	PrevComm [TaskCommLen]byte
	NextComm [TaskCommLen]byte
}

func main() {
	// Remove resource limits for kernels <5.11.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal("Removing memlock:", err)
	}

	// Load the compiled eBPF ELF and load it into the kernel.
	var objs sched_switchObjects
	if err := loadSched_switchObjects(&objs, nil); err != nil {
		log.Fatal("Loading eBPF objects:", err)
	}
	defer objs.Close()

	// Attach the eBPF program to the sched_switch tracepoint.
	tp, err := link.Tracepoint("sched", "sched_switch", objs.HandleSchedSwitch, nil)
	if err != nil {
		log.Fatal("Attaching tracepoint:", err)
	}
	defer tp.Close()

	log.Println("Monitoring sched_switch events... Press Ctrl+C to exit.")

	// Open the ring buffer for reading events.
	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		log.Fatal("Opening ring buffer reader:", err)
	}
	defer rd.Close()

	// Setup signal handler to exit gracefully.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		log.Println("Received signal, exiting...")
		rd.Close()
	}()

	// Read events from the ring buffer.
	for {
		record, err := rd.Read()
		if err != nil {
			if err == ringbuf.ErrClosed {
				log.Println("Ring buffer closed, exiting...")
				return
			}
			log.Printf("Reading from ring buffer: %s", err)
			continue
		}

		var event Event
		if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.LittleEndian, &event); err != nil {
			log.Printf("Parsing event: %s", err)
			continue
		}

		// Convert byte arrays to strings (null-terminated).
		prevComm := string(bytes.TrimRight(event.PrevComm[:], "\x00"))
		nextComm := string(bytes.TrimRight(event.NextComm[:], "\x00"))

		log.Printf("%s (%d) -> %s (%d)", prevComm, event.PrevPID, nextComm, event.NextPID)
	}
}
