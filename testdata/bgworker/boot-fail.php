<?php

// Bg worker fixture that throws on its very first line. Used by
// TestEnsureBackgroundWorkerBootFailure to prove ensure() surfaces the
// crash metadata (entrypoint, exit status, attempt count) when a worker
// keeps crashing during boot before reaching the readiness boundary.
throw new RuntimeException('intentional boot failure for test');
