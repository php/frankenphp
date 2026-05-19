<?php

// HTTP script that reads the background worker's shared vars and echoes
// them so the Go test can assert the round-trip.
$vars = frankenphp_get_vars('bg-basic');
echo 'message=', $vars['message'] ?? 'MISSING', "\n";
echo 'count=', $vars['count'] ?? 'MISSING', "\n";
echo 'has-ready-at=', isset($vars['ready_at']) ? '1' : '0', "\n";
