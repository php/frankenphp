<?php

// Broken background worker: returns immediately without ever waiting on its
// handle from frankenphp_get_worker_handle(). Without a guard this would
// respawn in a tight loop; the guard treats it as a boot failure instead.
