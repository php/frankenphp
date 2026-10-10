<?php

// mimics apps (e.g. Nextcloud) that exit during boot when REQUEST_URI is empty
if (($_SERVER['REQUEST_URI'] ?? null) === '') {
    exit(1);
}

$bootRequestUri = $_SERVER['REQUEST_URI'];

require_once __DIR__.'/_executor.php';

return function () use ($bootRequestUri) {
    echo $bootRequestUri;
};
