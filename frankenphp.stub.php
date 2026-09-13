<?php

/** @generate-class-entries */

/** @var int */
const FRANKENPHP_LOG_LEVEL_DEBUG = -4;

/** @var int */
const FRANKENPHP_LOG_LEVEL_INFO = 0;

/** @var int */
const FRANKENPHP_LOG_LEVEL_WARN = 4;

/** @var int */
const FRANKENPHP_LOG_LEVEL_ERROR = 8;

function frankenphp_handle_request(callable $callback): bool {}

function headers_send(int $status = 200): int {}

function frankenphp_finish_request(): bool {}

/**
 * @alias frankenphp_finish_request
 */
function fastcgi_finish_request(): bool {}

function frankenphp_request_headers(): array {}

/**
 * @alias frankenphp_request_headers
 */
function apache_request_headers(): array {}

/**
 * @alias frankenphp_request_headers
*/
function getallheaders(): array {}

function frankenphp_response_headers(): array|bool {}

/**
 * @alias frankenphp_response_headers
 */
function apache_response_headers(): array|bool {}

/**
 * @param string|string[] $topics
 */
function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}

/**
 * @param int $level The importance or severity of a log event. The higher the level, the more important or severe the event. For more details, see: https://pkg.go.dev/log/slog#Level
 * array<string, any> $context Values of the array will be converted to the corresponding Go type (if supported by FrankenPHP) and added to the context of the structured logs using https://pkg.go.dev/log/slog#Attr
 */
function frankenphp_log(string $message, int $level = 0, array $context = []): void {}

/**
 * EXPERIMENTAL: returns the handle of the current background worker, a
 * stream to wait on, alone or with the script's own streams: it becomes
 * readable when FrankenPHP needs the script's attention, its drain
 * included, and frankenphp_worker_tick() then tells whether the worker
 * still runs. Every call of a run returns the same stream, a fresh one over
 * the same socket once the script closed it. Only callable from inside a
 * background worker.
 *
 * @return resource
 */
function frankenphp_get_worker_handle() {}

/**
 * EXPERIMENTAL: the ready point and liveness check of a background worker,
 * the background analog of frankenphp_handle_request(). The first call of a
 * run marks the worker ready: the server start waits for it, and an exit
 * before it counts as a failure. It returns false once FrankenPHP drains the
 * worker, on shutdown, reboot or restart, so the script can leave its loop,
 * and true otherwise. It never blocks and never hands out work: the script
 * waits on the stream returned by frankenphp_get_worker_handle() and calls
 * this when it is readable. Whatever FrankenPHP wrote on that stream is
 * consumed here, the script does not have to read it. Only callable from
 * inside a background worker.
 */
function frankenphp_worker_tick(): bool {}
