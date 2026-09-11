<?php

// Bg worker checking the handle cache: two fetches of a run are the same
// stream, a fetch after closing it is a fresh one, and the drain still
// reaches the script through that one. Writes what it saw to BG_SENTINEL,
// then parks.
$first = frankenphp_get_worker_handle();
$second = frankenphp_get_worker_handle();
$result = $first === $second ? 'same' : 'different';

fclose($first);
$third = frankenphp_get_worker_handle();
$result .= $third === $second ? ' then same' : ' then fresh';

file_put_contents($_SERVER['BG_SENTINEL'], $result);
fgets($third);
