<?php

echo "just executing a script";
echo "argc is $argc\n";
echo "argv is: " . implode(',', $argv) . "\n";
usleep(10 * 1000); // sleep for 10ms
