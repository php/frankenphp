<?php

// Bg worker checking the stream of a handle: asking one handle twice gives
// the same stream, closing it and asking again gives a fresh one, another
// handle has its own, and the drain reaches the script through any of
// them. Writes what it saw to BG_SENTINEL, then parks.
$handle = new \FrankenPHP\WorkerHandle();
$first = $handle->getStream();
$again = $handle->getStream();
$result = $first === $again ? 'same' : 'different';

fclose($first);
$third = $handle->getStream();
$result .= $third === $first ? ' then same' : ' then fresh';

$other = (new \FrankenPHP\WorkerHandle())->getStream();
$result .= $other === $third ? ' then shared' : ' then its own';

file_put_contents($_SERVER['BG_SENTINEL'], $result);
$handle->tick();
fgets($third);
