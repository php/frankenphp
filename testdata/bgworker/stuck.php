<?php

// Intentionally stuck bg worker fixture: does NOT watch the stop pipe,
// so handler.drain() closing the write end has no effect. The only way
// to make RestartWorkers finish within the grace period is the
// force-kill primitive (pthread_kill on Linux/FreeBSD,
// CancelSynchronousIo on Windows). Used by
// TestBackgroundWorkerRestartForceKillsStuckThread to prove the
// force-kill path (not just the stop-pipe wake-up) is actually wired.
set_time_limit(0);

// Touch the worker handle so the combined readiness signal (handle +
// set_vars) closes; the stream itself is intentionally ignored — the
// point of this fixture is that we DON'T watch the stop pipe.
frankenphp_get_worker_handle();
frankenphp_set_vars(['ready' => 1]);

// sleep() is interruptible by SIGRTMIN+3 on Linux/FreeBSD and by
// alertable-wait APCs on Windows; the test skips platforms where
// neither mechanism can interrupt it.
sleep(60);
