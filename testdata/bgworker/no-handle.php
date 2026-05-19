<?php

// Long-lived bg worker fixture that intentionally does NOT call
// frankenphp_get_worker_handle(). Used by TestEnsureBackgroundWorkerTimeout
// to prove ensure() times out when the readiness boundary is never crossed.
// The 0 limit lets sleep() run as long as the test's drain takes.
set_time_limit(0);

sleep(60);
