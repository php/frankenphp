<?php

// Bg worker checking the stream of a handle: two handles of a run share
// one stream, closing it and asking again gives a fresh one, and the drain
// still reaches the script through that one. Writes what it saw to
// BG_SENTINEL, then parks.
$first = (new \FrankenPHP\WorkerHandle())->getStream();
$second = (new \FrankenPHP\WorkerHandle())->getStream();
$result = $first === $second ? 'same' : 'different';

fclose($first);
$handle = new \FrankenPHP\WorkerHandle();
$third = $handle->getStream();
$result .= $third === $second ? ' then same' : ' then fresh';

file_put_contents($_SERVER['BG_SENTINEL'], $result);
$handle->tick();
fgets($third);
