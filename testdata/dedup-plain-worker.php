<?php

// Dedicated worker for TestPhpServerWorkerMatchPoolCount (#2477 regression guard).
// Used by no other test so its metric label is hermetic.
while (frankenphp_handle_request(static function (): void {
    echo 'dedup-plain-worker';
})) {
}
