<?php
// trigger encoder middleware in Caddy
header('Content-Type: text/plain; charset=utf-8');

$chunk = str_repeat('Lorem ipsum dolor sit amet, consectetur adipiscing elit. ', 64);
$payload = str_repeat($chunk, 64);
echo $payload;
