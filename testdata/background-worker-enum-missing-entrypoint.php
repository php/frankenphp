<?php

require __DIR__ . '/background-worker-helper.php';

// This enum only exists in the background worker, not in the HTTP worker
enum SidekickOnlyEnum {
    case Foo;
}

frankenphp_set_vars(['val' => SidekickOnlyEnum::Foo]);

while (!background_worker_should_stop(30)) {
}
