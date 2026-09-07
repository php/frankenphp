<?php

// A status line shorter than 9 bytes must not crash header dispatch.
header('HTTP/');
echo 'ok';
