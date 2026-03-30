<?php

$vars = frankenphp_get_vars('caddy-bg-worker', 5.0);
echo $vars['CADDY_TEST'] ?? 'NOT_SET';
