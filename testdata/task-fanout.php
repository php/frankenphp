<?php

// A sender that queues all its tasks before reading any, on a worker that
// streams its updates meanwhile, then reads them one after the other, the
// last one first when order is reverse.
try {
    $tasks = [];
    for ($i = 0; $i < (int) $_GET['n']; ++$i) {
        $tasks[] = new \FrankenPHP\SentTaskHandle('echo', ['input' => "t$i", 'steps' => (int) $_GET['steps']], 5.0);
    }
    if ('reverse' === ($_GET['order'] ?? '')) {
        $tasks = array_reverse($tasks);
    }
    $updates = 0;
    foreach ($tasks as $task) {
        while (null !== $task->read()) {
            ++$updates;
        }
    }
    echo count($tasks), ' tasks, ', $updates, ' updates';
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
