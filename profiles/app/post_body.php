<?php
header('Content-Type: text/plain');
$body = file_get_contents('php://input');
echo "len=" . strlen($body);
