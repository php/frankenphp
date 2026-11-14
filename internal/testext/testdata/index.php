<?php

$extensions = get_loaded_extensions();
printf("ext1: %s\n", in_array('ext1', $extensions, true) ? 'loaded' : 'missing');
printf("ext2: %s\n", in_array('ext2', $extensions, true) ? 'loaded' : 'missing');
printf("ext2_answer: %s\n", function_exists('ext2_answer') ? ext2_answer() : 'missing');
printf("EXT2_MINIT_CALLS: %s\n", defined('EXT2_MINIT_CALLS') ? EXT2_MINIT_CALLS : 'missing');
