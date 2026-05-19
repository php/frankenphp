<?php

// Reads the "shared" worker's scope tag. Which worker this resolves to
// depends on the request's BackgroundScope. ensure() waits for the
// scoped worker to publish at least once, removing the race between
// eager start (num=1) and the first request landing here.
frankenphp_ensure_background_worker('shared');
$vars = frankenphp_get_vars('shared');
echo 'scope=', $vars['scope'] ?? 'MISSING', "\n";
