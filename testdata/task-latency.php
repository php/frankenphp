<?php

// Sends tasks one after the other and prints the median of their round trips,
// in microseconds: each task has to wake a thread by itself, the watcher of
// the queue only looks at what waits there every 10 ms.
$times = [];
for ($i = 0; $i < 21; ++$i) {
    $start = hrtime(true);
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => "$i"]);
    while (null !== $task->read()) {
    }
    $times[] = intdiv(hrtime(true) - $start, 1000);
}
sort($times);
echo $times[10];
