<?php

// Reports whether the handle is still readable after a tick: the tick
// consumes the wake-ups, including the one sent at start, so a script that
// ticked has nothing left to read until the next one or the drain.
$handle = new \FrankenPHP\WorkerHandle();
$stream = $handle->getStream();
$poll = static function () use ($stream): string {
    $read = [$stream];
    $write = $except = null;

    return stream_select($read, $write, $except, 0) > 0 ? 'readable' : 'quiet';
};

$seen = ['start:' . $poll()];
$handle->tick();
$seen[] = 'after tick:' . $poll();
$handle->tick();
$seen[] = 'after second tick:' . $poll();
file_put_contents($_SERVER['BG_SENTINEL'], implode(' ', $seen));

while ($handle->tick()) {
    $read = [$stream];
    $write = $except = null;
    stream_select($read, $write, $except, null);
}
