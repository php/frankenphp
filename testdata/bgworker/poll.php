<?php

// Bg worker waiting on its handle through the poll API of PHP 8.6: the
// handle goes into the context as is, no stream in sight. It writes the
// backend to BG_SENTINEL once ready, then parks.
set_time_limit(0);
$handle = new \FrankenPHP\WorkerHandle();
$poll = new \Io\Poll\Context();
$poll->add($handle, [\Io\Poll\Event::Read]);

while ($handle->tick()) {
    file_put_contents($_SERVER['BG_SENTINEL'], $poll->getBackend()->name);
    $poll->wait();
}
