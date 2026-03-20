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

/**
 * @param array<null|scalar|\UnitEnum|array<mixed>> $vars Nested arrays must recursively follow the same type constraints
 */
function frankenphp_worker_set_vars(array $vars): void {}

/**
 * @return array<null|scalar|array<mixed>|\UnitEnum> Nested arrays recursively follow the same type constraints
 */
function frankenphp_worker_get_vars(string|array $name, float $timeout = 30.0): array {}

/** @return resource */
function frankenphp_worker_get_signaling_stream() {}

/**
 * Sends a task to a background worker and returns a readable stream for receiving updates.
 * The background worker is started if not already running (at-most-once).
 * Close the stream with fclose() to cancel the task.
 *
 * @param array<array-key, null|scalar|\UnitEnum|array<mixed>> $payload
 * @return resource A readable stream - use stream_select() to wait for updates, task_read() to dequeue
 * @throws \ValueError If name is empty or payload contains unsupported types
 * @throws \RuntimeException If the background worker cannot be started
 */
function frankenphp_worker_task_send(string $name, array $payload, float $timeout = 30.0) {}

/**
 * Reads the next update from a task stream. Returns null on EOF (task completed).
 *
 * @param resource $stream The stream returned by task_send()
 * @return array|null The update data, or null when the background worker closed the stream cleanly
 * @throws \RuntimeException If the background worker exited without completing the task
 * @throws \ValueError If the argument is not a task stream from task_send()
 */
function frankenphp_worker_task_read(mixed $stream): ?array {}

/**
 * Dequeues the next pending task. Call after reading "task\n" from the signaling stream.
 * Returns [$taskStream, $payload] or false if no task is available (spurious signal,
 * cancelled task, or fan-out contention with num > 1).
 * Can only be called from a background worker.
 *
 * @return array{resource, array}|null [$stream, $payload] or null
 * @throws \RuntimeException If not called from a background worker
 */
function frankenphp_worker_task_receive(): ?array {}

/**
 * Sends an update to the task sender. Blocks if the FIFO is full (backpressure).
 * Can only be called from a background worker with a task stream from task_receive().
 *
 * @param resource $stream The stream from task_receive()
 * @param array<array-key, null|scalar|\UnitEnum|array<mixed>> $data
 * @throws \ValueError If the argument is not a task stream from task_receive()
 * @throws \RuntimeException If not called from a background worker, stream is closed, or cancelled by sender
 */
function frankenphp_worker_task_update(mixed $stream, array $data): void {}

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
 * @param int                  $level   The importance or severity of a log event. The higher the level, the more important or severe the event. For more details, see: https://pkg.go.dev/log/slog#Level
 * @param array<string, mixed> $context Values of the array will be converted to the corresponding Go type (if supported by FrankenPHP) and added to the context of the structured logs using https://pkg.go.dev/log/slog#Attr
 */
function frankenphp_log(string $message, int $level = 0, array $context = []): void {}
