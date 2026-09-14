<?php

// Reports whether the handle is still readable after a tick: the tick
// consumes the wake-ups, including the one sent at start, so a script that
// ticked has nothing left to read until the next one or the drain.
$handle = frankenphp_get_worker_handle();
$poll = static function () use ($handle): string {
    $read = [$handle];
    $write = $except = null;

    return stream_select($read, $write, $except, 0) > 0 ? 'readable' : 'quiet';
};

$seen = ['start:' . $poll()];
frankenphp_worker_tick();
$seen[] = 'after tick:' . $poll();
frankenphp_worker_tick();
$seen[] = 'after second tick:' . $poll();
file_put_contents($_SERVER['BG_SENTINEL'], implode(' ', $seen));

while (frankenphp_worker_tick()) {
    $read = [$handle];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
