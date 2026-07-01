<?php

// Worker for TestWorkerTimeout_InterruptsBlockingSocketRead. It connects to the
// address in the Upstream-Addr header and blocks in fread() waiting for data the
// server never sends - the same blocking socket read (ppoll) a slow DB query
// parks on. The worker timeout must abort the fread via the fd shutdown; the
// "read returned" output after it proves whether the read was interrupted.

$fn = static function (): void {
    $addr = $_SERVER['HTTP_UPSTREAM_ADDR'] ?? '';
    $sock = @stream_socket_client("tcp://$addr", $errno, $errstr, 5);
    if ($sock === false) {
        echo "connect failed: $errstr\n";
        return;
    }

    $data = fread($sock, 8192); // blocks: the server never writes
    echo 'read returned: ' . strlen((string) $data) . "\n";
};

while (frankenphp_handle_request($fn)) {
}
