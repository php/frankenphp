<?php

// Runs on a regular thread. Compiles fresh scripts until the opcache hash
// overflows, which schedules a restart, then reports how many restarts have
// been performed so the caller can stop as soon as one lands.
if (!function_exists('opcache_get_status')) {
    echo 'NOOPCACHE';

    return;
}

$dir = sys_get_temp_dir() . '/frankenphp-opcache-restart-test';
@mkdir($dir, 0777, true);

$batch = (int) ($_GET['b'] ?? 0);
for ($i = 0; $i < 120; $i++) {
    $file = $dir . '/g' . $batch . '_' . $i . '.php';
    file_put_contents($file, "<?php\nfunction fn_{$batch}_{$i}() { return {$i}; }\n" . str_repeat("// pad\n", 40));
    @include_once $file;
}

// Discarding scripts grows wasted_shared_memory, which is what lets the
// hash-full path schedule a restart.
foreach (glob($dir . '/*.php') as $file) {
    @opcache_invalidate($file, true);
}

$status = @opcache_get_status(false);
if (!is_array($status) || !isset($status['opcache_statistics'])) {
    echo 'NOOPCACHE';

    return;
}

$s = $status['opcache_statistics'];
echo (int) ($s['oom_restarts'] + $s['hash_restarts'] + $s['manual_restarts']);
