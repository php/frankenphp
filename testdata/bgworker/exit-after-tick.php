<?php

// Reaches the ready point, the first frankenphp_worker_tick(), then returns
// at once: a clean exit past the tick is re-run, paced like a crash so it
// does not spin. Runs are counted in BG_COUNT_FILE.
file_put_contents($_SERVER['BG_COUNT_FILE'], "run\n", FILE_APPEND);
frankenphp_worker_tick();
