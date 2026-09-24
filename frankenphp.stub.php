<?php

/** @generate-class-entries */

namespace {
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
     * Returns a copy of the vars last published by the named background worker,
     * resolved within the current php_server, then among global workers. Blocks
     * until that worker reached its ready point. Throws if the worker is
     * unknown, if it is ready but has not published any vars, or if background
     * workers wait on each other in a cycle.
     */
    function frankenphp_get_vars(string $name): array {}
}

namespace FrankenPHP {
    /**
     * @strict-properties
     * @not-serializable
     *
     * EXPERIMENTAL: the handle of the current background worker, the one
     * point where the script and FrankenPHP meet. Constructing it outside
     * a background worker throws. It implements Io\Poll\Handle, so an
     * Io\Poll\Context waits on it directly, the one of PHP 8.6 or the one
     * of symfony/polyfill-io-poll below that.
     */
    final class WorkerHandle
    {
        public function __construct() {}

        /**
         * The ready point and liveness check of a background worker, the
         * background analog of frankenphp_handle_request(). The first call
         * of a run marks the worker ready: the server start waits for it,
         * and an exit before it counts as a failure. max_execution_time
         * applies until that call and not after. It returns false once
         * FrankenPHP drains the worker, on shutdown, reboot or restart, so
         * the script can leave its loop, and true otherwise. It never
         * blocks and never hands out work: the script waits on the stream
         * of getStream() and calls this when it is readable, which also
         * consumes whatever FrankenPHP wrote there.
         */
        public function tick(): bool {}

        /**
         * The stream to wait on, alone or with the script's own streams: it
         * becomes readable when FrankenPHP needs the script's attention,
         * its drain included. Only waiting on it is supported, through
         * stream_select() or a blocking read; what it carries is not part
         * of the contract and tick() consumes it. Reading it steals those
         * bytes from tick(), writing to it goes nowhere. Closing it is
         * safe: a handle hands out one stream, a fresh one over the same
         * socket once the script closed it.
         *
         * @return resource
         */
        public function getStream() {}

        /**
         * Publishes the vars of this background worker: the array replaces
         * the previous snapshot, atomically for readers, which get copies.
         * Values must be null, scalars, arrays or enums.
         */
        public function setVars(array $vars): void {}

        /**
         * Dequeues a task sent to this background worker, without blocking,
         * or null when there is none. Each task sent wakes one thread of the
         * worker through its handle, so a script calls this after tick()
         * returned. A wake-up is not a count, and null after one is expected
         * in a pool.
         */
        public function receive(): ?ReceivedTaskHandle {}
    }

    /**
     * EXPERIMENTAL: the sender's side of a task, an Io\Poll\Handle on PHP
     * 8.6. Constructing it sends the
     * task to the named background worker, resolved within the current
     * php_server then among global workers, and waits for one of its threads
     * to pick it up, at most $timeout seconds, forever when null. Payload
     * values must be null, scalars, arrays or enums.
     *
     * @strict-properties
     * @not-serializable
     */
    final class SentTaskHandle
    {
        public function __construct(string $worker, array $payload, ?float $timeout = 30.0) {}

        /**
         * The next update published by the worker, waiting for it, or null
         * once the task is complete. Throws when the worker ended without
         * completing it, and when the wait passes the stream's read timeout.
         */
        public function read(): ?array {}

        /**
         * The stream to wait on, alone or with other tasks: it becomes
         * readable when an update is there and reaches EOF when the task
         * ends. Only waiting on it is supported, read() consumes what it
         * carries and reading it steals that signal. Closing it abandons
         * the task. Past its end a stream the script already took comes
         * back closed, and asking for a first one throws.
         *
         * @return resource
         */
        public function getStream() {}

        /**
         * Gives up on the task: the worker's next update throws. Dropping
         * the handle does the same.
         */
        public function abandon(): void {}
    }

    /**
     * EXPERIMENTAL: the worker's side of a task, handed out by
     * WorkerHandle::receive(), an Io\Poll\Handle on PHP 8.6. Ending the run with a task open aborts it,
     * which the sender is told about.
     *
     * @strict-properties
     * @not-serializable
     */
    final class ReceivedTaskHandle
    {
        private function __construct() {}

        /**
         * The payload the sender passed.
         */
        public function getPayload(): array {}

        /**
         * Publishes an update for the sender, which reads it with read().
         * Values must be null, scalars, arrays or enums. Sixteen updates are
         * buffered per task, past that this waits for the sender to read.
         */
        public function update(array $data): void {}

        /**
         * Completes the task, with a last update when one is given: the
         * sender's read() returns it, then null.
         */
        public function complete(?array $data = null): void {}

        /**
         * The stream to wait on: it reaches EOF when the sender abandons the
         * task, for stream_select() and feof(). Only waiting on it is
         * supported. Closing it completes the task. Past its end a stream
         * the script already took comes back closed, and asking for a
         * first one throws.
         *
         * @return resource
         */
        public function getStream() {}
    }
}
