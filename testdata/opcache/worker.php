<?php

// The token changes each time the worker script starts. The literal is an
// interned string and the const array is immutable: both live in opcache's
// shared memory, which a restart reuses.
const OPCACHE_CANARY = ['a' => 1, 'b' => [2, 3], 'c' => 'literal'];

$token = bin2hex(random_bytes(4));
$canary = 'opcache_canary_interned_string';
$expected = crc32($canary.serialize(OPCACHE_CANARY));

$handler = static function () use ($token, $canary, $expected) {
    if (!function_exists('opcache_get_status')) {
        echo '{}';

        return;
    }

    // compile and invalidate scripts until opcache schedules a restart
    if ($batch = (int) ($_GET['batch'] ?? 0)) {
        $dir = $_GET['dir'];
        for ($i = 0; $i < 120 && !opcache_get_status(false)['restart_pending']; ++$i) {
            file_put_contents("$dir/f{$batch}_$i.php", "<?php\nfunction f_{$batch}_{$i}() { return ".implode(' + ', range(1, 200))."; }\n");
            include "$dir/f{$batch}_$i.php";
        }

        // invalidated scripts are wasted memory, which opcache needs to restart
        foreach (glob("$dir/*.php") as $file) {
            opcache_invalidate($file, true);
        }
    }

    $status = opcache_get_status(false);
    $s = $status['opcache_statistics'];

    echo json_encode([
        'token' => $token,
        'cached' => $status['opcache_enabled'],
        'pending' => $status['restart_pending'],
        'restarts' => $s['oom_restarts'] + $s['hash_restarts'] + $s['manual_restarts'],
        'corrupt' => crc32($canary.serialize(OPCACHE_CANARY)) !== $expected,
    ]);
};

while (frankenphp_handle_request($handler));
