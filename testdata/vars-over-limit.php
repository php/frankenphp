<?php

// Reads a snapshot larger than what the memory_limit of this request leaves:
// the copy is refused before it starts, where PHP would only raise its error
// once it is done, or end the whole process past its hard limit.
ini_set('memory_limit', '8M');
try {
    echo strlen(frankenphp_get_vars('big')['blob']);
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
