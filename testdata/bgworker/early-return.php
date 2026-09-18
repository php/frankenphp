<?php

// Broken background worker: returns immediately without ever ticking its
// handle. Without a guard this would respawn in a tight loop; the guard
// treats it as a boot failure instead.
