<?php

// Exercises frankenphp_test_persist_roundtrip (only registered when
// FRANKENPHP_TEST_HOOKS is defined at build time). The script runs as a
// plain non-worker request; the Go harness asserts the combined output.

enum Status: string {
    case Active = 'active';
    case Paused = 'paused';
}

function same(mixed $actual, mixed $expected, string $label): void {
    if ($actual !== $expected) {
        echo "FAIL $label: expected ";
        var_export($expected);
        echo ' got ';
        var_export($actual);
        echo "\n";
        return;
    }
    echo "OK $label\n";
}

$rt = 'frankenphp_test_persist_roundtrip';
if (!function_exists($rt)) {
    echo "SKIP frankenphp_test_persist_roundtrip not registered\n";
    return;
}

// Scalars.
same($rt(null), null, 'null');
same($rt(false), false, 'false');
same($rt(true), true, 'true');
same($rt(0), 0, 'int zero');
same($rt(42), 42, 'int');
same($rt(-1), -1, 'int negative');
same($rt(1.5), 1.5, 'float');
same($rt(''), '', 'empty string');
same($rt('hello'), 'hello', 'short string');

// Long (non-interned) string: forces the allocation path.
$long = str_repeat('x', 1024);
same($rt($long), $long, 'long string');

// Nested arrays, mixed keys.
$arr = [
    'name' => 'alice',
    'age' => 30,
    'tags' => ['admin', 'editor'],
    'meta' => ['created' => 1234567890, 'flags' => [true, false, null]],
    0 => 'first',
    1 => 'second',
];
same($rt($arr), $arr, 'nested array');

// Enum roundtrip: identity (===) must be preserved because the enum is
// re-resolved to the same singleton case on the read side.
same($rt(Status::Active), Status::Active, 'enum active');
same($rt(Status::Paused), Status::Paused, 'enum paused');

// Array containing an enum.
same(
    $rt(['status' => Status::Active, 'count' => 7]),
    ['status' => Status::Active, 'count' => 7],
    'array with enum',
);

// Invalid inputs throw LogicException.
try {
    $rt(new stdClass());
    echo "FAIL stdClass should throw\n";
} catch (\LogicException) {
    echo "OK stdClass rejected\n";
}

try {
    $rt(fopen('php://memory', 'r'));
    echo "FAIL resource should throw\n";
} catch (\LogicException) {
    echo "OK resource rejected\n";
}

try {
    $rt(['ok' => 1, 'bad' => new stdClass()]);
    echo "FAIL nested stdClass should throw\n";
} catch (\LogicException) {
    echo "OK nested stdClass rejected\n";
}
