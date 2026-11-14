<?php

// HTTP worker reading the vars of "cfg" in each request and after it, where
// a script does its work once the response is sent: each response tells both
// what its request read and what the script read after the previous one.
$read = static function (): string {
    try {
        return frankenphp_get_vars('cfg')['value'];
    } catch (\Throwable $e) {
        return get_class($e).': '.$e->getMessage();
    }
};
$after = null;
while (frankenphp_handle_request(static function () use ($read, &$after): void {
    echo json_encode(['request' => $read(), 'after' => $after]);
})) {
    $after = $read();
}
