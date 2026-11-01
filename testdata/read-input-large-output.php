<?php

$input = file_get_contents('php://input');
echo str_repeat('a', 8192), 'read=', strlen($input);
