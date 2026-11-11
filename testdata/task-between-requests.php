<?php

// HTTP worker sending a task to "jobs" in each request and after it, where
// a script does its work once the response is sent: each response tells
// which worker took the task of its request and the one sent after the
// previous request.
$send = static function (): string {
    try {
        return (new \FrankenPHP\SentTaskHandle('jobs', []))->read()['tag'];
    } catch (\Throwable $e) {
        return get_class($e).': '.$e->getMessage();
    }
};
$after = null;
while (frankenphp_handle_request(static function () use ($send, &$after): void {
    echo json_encode(['request' => $send(), 'after' => $after]);
})) {
    $after = $send();
}
