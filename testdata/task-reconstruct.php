<?php

// Constructs a task handle twice: the second call throws, the first task goes on.
$task = new \FrankenPHP\SentTaskHandle('echo', ['input' => 'first']);
try {
    $task->__construct('echo', ['input' => 'second']);
} catch (\Error $e) {
    echo get_class($e), ': ', $e->getMessage(), "\n";
}
while (null !== $update = $task->read()) {
    echo json_encode($update), "\n";
}
