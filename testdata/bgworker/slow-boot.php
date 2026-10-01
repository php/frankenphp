<?php

// Boots slower than max_execution_time allows and never reaches its first
// tick: the limit ends the run, a boot failure. Counts its boots in
// BG_COUNT_FILE. A busy loop rather than a sleep, the limit is checked
// between opcodes.
file_put_contents($_SERVER['BG_COUNT_FILE'], "boot\n", FILE_APPEND);
$until = microtime(true) + 5;
while (microtime(true) < $until) {
}
(new \FrankenPHP\WorkerHandle())->tick();
