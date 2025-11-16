<?php

// NOTE: use CGO frankenphp_log method.
// The message and it's optional arguments are expected to be logged by go' std slog system.
// The log level should be respected out of the box by the std' slog.
//
// ac[0] expect the log message as string
// ac[1] expect the slog.Level, from -8 to +8
// ac[2] is an optional php map, which will be converted to a []slog.Attr

frankenphp_log("some debug message", -4, [
    "key int"    => 1,
]);

frankenphp_log("some info message", 0, [
    "key string" => "string",
]);

frankenphp_log("some warn message", 4);

frankenphp_log("some error message", 8, [
    "err" => ["a", "v"],
]);
