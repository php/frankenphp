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
 * before it counts as a failure. max_execution_time applies until that
 * call and not after. It returns false once FrankenPHP drains the
 * worker, on shutdown, reboot or restart, so the script can leave its loop,
 * and true otherwise. It never blocks and never hands out work: the script
 * waits on the stream returned by frankenphp_get_worker_handle() and calls
 * this when it is readable. Whatever FrankenPHP wrote on that stream is
 * consumed here, the script does not have to read it. Only callable from
 * inside a background worker.
 */
function frankenphp_worker_tick(): bool {}

/**
 * Publishes the vars of the current background worker: the array replaces
 * the previous snapshot, atomically for readers, which get copies. Values
 * must be null, scalars, arrays or enums. Only callable from inside a
 * background worker.
 */
function frankenphp_set_vars(array $vars): void {}

/**
 * Returns a copy of the vars last published by the named background worker,
 * resolved within the current php_server, then among global workers. Blocks
 * until that worker reached its ready point. Throws if the worker is
 * unknown, if it is ready but has not published any vars, or if background
 * workers wait on each other in a cycle.
 */
function frankenphp_get_vars(string $name): array {}

/**
 * Hands a task to the named background worker, resolved like
 * frankenphp_get_vars() does, and returns a stream carrying the updates it
 * sends back. Blocks until a thread of the worker picks the task up; throws
 * if none did within $timeout seconds (null waits forever), or if the
 * worker is unknown. Payload values must be null, scalars, arrays or enums.
 * Closing the stream abandons the task.
 *
 * @return resource
 */
function frankenphp_send_task(string $name, array $payload, ?float $timeout = 30.0) {}

/**
 * Returns the next update of a task, blocking until the background worker
 * sends one, or null once it completed the task. Throws if the worker ended
 * its script with the task open. The stream also works with stream_select().
 *
 * @param resource $stream A stream returned by frankenphp_send_task()
 */
function frankenphp_read_task($stream): ?array {}

/**
 * Dequeues a task sent to the current background worker, without blocking:
 * [$stream, $payload], or null when there is none. Each task sent wakes one
 * thread of the worker through its handle, so a script calls this after
 * frankenphp_worker_tick() returned. A wake-up is not a count, and null
 * after one is expected in a pool. $stream reaches EOF when the sender
 * closes its own stream, for stream_select() and feof(). Only callable from
 * inside a background worker.
 *
 * @return array{resource, array}|null
 */
function frankenphp_receive_task(): ?array {}

/**
 * Sends an update, progress or result, to the sender of a task; fclose() on
 * the stream completes the task, before the script ends: a close during
 * request shutdown, from a destructor or a shutdown function included,
 * reports the task as not completed instead. Values must be null, scalars,
 * arrays or enums. At most 16 updates are buffered: past that, blocks until
 * the sender reads. Throws once the sender closed its stream.
 *
 * @param resource $stream A stream returned by frankenphp_receive_task()
 */
function frankenphp_update_task($stream, array $data): void {}
