<?php

// Sends n tasks and leaves each one right after its result, without reading
// to the end.
$result = null;
for ($i = 0, $n = (int) ($_GET['n'] ?? 1); $i < $n; ++$i) {
    $task = new \FrankenPHP\SentTaskHandle('echo', ['input' => (string) $i]);
    $result = $task->read();
    unset($task);
}
echo $result['result'] ?? 'no result';
