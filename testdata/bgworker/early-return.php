<?php

// Broken background worker: returns immediately without ever calling
// frankenphp_worker_tick(). Without a guard this would respawn in a tight
// loop; the guard treats it as a boot failure instead.
