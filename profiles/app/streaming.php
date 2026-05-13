<?php
header('Content-Type: text/plain; charset=utf-8');
header('X-Accel-Buffering: no');

while (ob_get_level() > 0) {
    ob_end_flush();
}

for ($i = 0; $i < 32; $i++) {
    echo "chunk-{$i} ";
    echo str_repeat('x', 64);
    echo "\n";
    flush();
}
