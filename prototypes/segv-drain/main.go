// Prototype of the crash drain proposed in php/frankenphp#2836.
//
// Four C threads serve fake requests and call into Go, like PHP threads do.
// Thread 2 faults during its second request. The handler in drain.c parks
// the thread, Go lets the other requests finish, then releases the thread
// and the process dies by the original signal.
//
// Usage: go build -o drain . && ./drain <mode> <altstack>
//
//	mode 0: NULL dereference in C (exit status 139 after the drain)
//	mode 1: stack overflow in C (exit status 139 after the drain)
//	mode 2: nil dereference in a Go callback (recovered, exit status 0)
//	mode 3: NULL dereference in C called from a Go callback (exit status 139 after the drain)
//	mode 4: NULL dereference in C called from a goroutine (exit status 139, no drain)
//	altstack 1: the C threads install their own alternate signal stack
//
// DRAIN=0 skips the handler, to compare with what happens today: modes 3
// and 4 then print Go's traceback and exit with status 2.
package main

/*
#include "drain.h"
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	mu       sync.Mutex
	draining bool
	inflight = map[int]int{}
	start    = time.Now()
)

func logf(format string, args ...any) {
	fmt.Printf("%6dms  %s\n", time.Since(start).Milliseconds(), fmt.Sprintf(format, args...))
}

//export go_request_start
func go_request_start(idx, req C.int) C.int {
	mu.Lock()
	defer mu.Unlock()
	if draining {
		return 0
	}
	inflight[int(idx)] = int(req)
	return 1
}

//export go_request_done
func go_request_done(idx, req C.int) {
	mu.Lock()
	delete(inflight, int(idx))
	d := draining
	mu.Unlock()
	if d {
		logf("thread %d finished request %d during the drain", idx, req)
	}
}

//export go_thread_exit
func go_thread_exit(idx C.int) {
	logf("thread %d stopped", idx)
}

//export go_nil_deref
func go_nil_deref() (ret C.int) {
	defer func() {
		if r := recover(); r != nil {
			logf("fault in Go code on a PHP thread: recovered as a panic (%v)", r)
		}
	}()
	var p *C.int
	return *p
}

//export go_fault_via_c
func go_fault_via_c() {
	C.fault_in_c()
}

func main() {
	mode, _ := strconv.Atoi(os.Args[1])
	ownAltstack, _ := strconv.Atoi(os.Args[2])

	go func() {
		f := os.NewFile(uintptr(C.notify_fd()), "notify")
		var idx int32
		if err := binary.Read(f, binary.LittleEndian, &idx); err != nil {
			panic(err)
		}
		mu.Lock()
		draining = true
		logf("thread %d faulted in C during request %d; draining %d other requests", idx, inflight[int(idx)], len(inflight)-1)
		delete(inflight, int(idx))
		mu.Unlock()

		runtime.GC()
		logf("GC cycle completed while the faulting thread is parked")

		deadline := time.Now().Add(2 * time.Second)
		for {
			mu.Lock()
			n := len(inflight)
			mu.Unlock()
			if n == 0 || time.Now().After(deadline) {
				logf("drain over, %d requests left; releasing the faulting thread", n)
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		C.release_crashed()
	}()

	C.start_threads(4, 2, C.int(mode), C.int(ownAltstack))

	if mode == 4 {
		time.Sleep(600 * time.Millisecond)
		C.fault_in_c()
	}

	if mode == 2 {
		time.Sleep(1200 * time.Millisecond)
		logf("still alive, exiting normally")
		return
	}
	select {}
}
