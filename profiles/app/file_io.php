<?php
$file = '/tmp/benchmark_' . bin2hex(random_bytes(16)) . '.txt';

$data = str_repeat('Lorem ipsum dolor sit amet, consectetur adipiscing elit. ', 1000);
file_put_contents($file, $data);

$content = file_get_contents($file);

$lines = explode(' ', $content);
$filtered = array_filter($lines, fn($line) => strlen($line) > 5);
sort($filtered);

unlink($file);

echo "OK\n";
