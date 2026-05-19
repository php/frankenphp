<?php

// HTTP fixture: ensure three workers in a single batch call. Each
// catch-all instance writes its own per-name sentinel under
// $_SERVER['BG_SENTINEL_DIR'] (set via WithWorkerEnv at the bg worker
// declaration). The HTTP response just confirms the call did not throw.
frankenphp_ensure_background_worker(['batch-a', 'batch-b', 'batch-c']);
echo "ok\n";
