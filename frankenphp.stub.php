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
 * Declares a dependency on one or more background workers. Lazy-starts each
 * worker that isn't already running, then blocks until every named worker
 * has reached combined readiness — meaning it has called both
 * frankenphp_get_worker_handle() AND frankenphp_set_vars() at least once
 * — aborts after exhausting its max_consecutive_failures cap, or the
 * shared timeout expires. Throws RuntimeException if no background
 * worker is configured for any given name, if a worker fails to reach
 * readiness within the timeout, or if a worker aborts during boot.
 *
 * The array form rejects empty arrays (ValueError), non-string elements
 * (TypeError), empty strings, and duplicate names (ValueError) before
 * any worker is started.
 *
 * @param string|string[] $name
 * @param float|null $timeout deadline in seconds. null (the default) falls
 *                            back to FrankenPHP's internal default
 *                            timeout. A value <= 0 raises ValueError.
 */
function frankenphp_ensure_background_worker(string|array $name, ?float $timeout = null): void {}

/**
 * Publishes the given vars from a background worker. Only callable from a
 * worker started with the `background` flag. Values must be null, scalars,
 * arrays of allowed values, or enum cases.
 */
function frankenphp_set_vars(array $vars): void {}

/**
 * Reads the shared vars published by the named background worker. Throws if
 * the worker is not declared, not running, or has not yet called set_vars.
 */
function frankenphp_get_vars(string $name): array {}

/**
 * Returns the stop-signal stream for the current background worker. The
 * stream closes when FrankenPHP is draining the worker so the script can
 * exit its loop gracefully. Only callable from inside a background worker.
 *
 * @return resource
 */
function frankenphp_get_worker_handle(): mixed {}
