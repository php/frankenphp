//go:build linux

package main

/*
#include <pthread.h>
#include <unistd.h>

static void *segfault(void *unused) {
	(void)unused;
	*(volatile int *)0 = 1;
	return NULL;
}

static void trigger_sigsegv_on_c_thread(void) {
	pthread_t thread;
	if (pthread_create(&thread, NULL, segfault, NULL) != 0) {
		_exit(3);
	}
	pthread_join(thread, NULL);
}
*/
import "C"

func triggerSIGSEGVOnCThread() {
	C.trigger_sigsegv_on_c_thread()
}
