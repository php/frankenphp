<?php

printf("argv0=%s argc=%d args=%s\n", $argv[0], $argc, json_encode(array_slice($argv, 1)));
