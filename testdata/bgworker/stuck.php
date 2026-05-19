<?php

// Intentionally stuck bg worker fixture: does NOT watch the stop pipe,
// so handler.drain() closing the write end has no effect. The only way
// to make RestartWorkers finish within the grace period is the
// force-kill primitive (pthread_kill on Linux/FreeBSD). Used by
// TestBackgroundWorkerRestartForceKillsStuckThread to prove the
// force-kill path (not just the stop-pipe wake-up) is actually wired.
set_time_limit(0);

if (!empty($_SERVER['BG_SENTINEL'])) {
    @touch($_SERVER['BG_SENTINEL']);
}

// sleep() is interruptible by SIGRTMIN+3 on Linux/FreeBSD; the test
// skips platforms where neither mechanism can interrupt it.
sleep(60);
