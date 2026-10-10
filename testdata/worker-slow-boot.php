<?php

// simulates a framework bootstrap, so that reloads take a while to boot workers
usleep(300000);

while (frankenphp_handle_request(static function (): void {
    echo 'ok';
})) {
}
