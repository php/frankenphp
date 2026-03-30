<?php

frankenphp_handle_request(function () {
    $vars = frankenphp_get_vars('my-named-worker', 5.0);
    echo ($vars['WORKER_NAME'] ?? 'missing') . '|' . ($vars['AUTOSTART'] ? 'true' : 'false');
});
