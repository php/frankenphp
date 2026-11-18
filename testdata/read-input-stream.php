<?php

$input = fopen('php://input', 'r');
$start = fread($input, 8192);
touch($_GET['marker']);
echo 'read=' . strlen($start . stream_get_contents($input));
