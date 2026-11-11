<?php

/**
 * @generate-class-entries
 * @generate-legacy-arginfo 80200
 */

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
     * @param string|string[] $topics The first topic is the canonical one, the others are alternate topics
     */
    function mercure_publish(string|array $topics, string $data = '', bool $private = false, ?string $id = null, ?string $type = null, ?int $retry = null): string {}

    /**
     * @param int $level The importance or severity of a log event. The higher the level, the more important or severe the event. For more details, see: https://pkg.go.dev/log/slog#Level
     * array<string, any> $context Values of the array will be converted to the corresponding Go type (if supported by FrankenPHP) and added to the context of the structured logs using https://pkg.go.dev/log/slog#Attr
     */
    function frankenphp_log(string $message, int $level = 0, array $context = []): void {}
    /**
     * EXPERIMENTAL: returns a copy of the snapshot last published in a vars
     * namespace, resolved within the current php_server, then among global
     * workers. Until the snapshot changes, the copy is handed out again for
     * the rest of the request, or of the run of a worker script. Blocks until
     * the publishing worker reached its ready point. Throws if no background
     * worker publishes the namespace, or if background workers wait on each
     * other in a cycle.
     */
    function frankenphp_get_vars(string $namespace): array {}
}

namespace FrankenPHP {
    /** Thrown when reading a request body larger than the configured limit */
    class RequestBodyTooLargeException extends \Exception {}

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
         * The name of the worker, as declared or generated from the script
         * path: what tells apart the workers running the same script.
         */
        public function getName(): string {}

        /**
         * The ready point and liveness check of a background worker, the
         * background analog of frankenphp_handle_request(). The first call
         * of a run marks the worker ready: the server start waits for it,
         * and an exit before it counts as a failure. max_execution_time
         * applies until that call and not after. It returns false once
         * FrankenPHP drains the worker, on shutdown, reboot or restart, so
         * the script can leave its loop, and true otherwise. It never
         * hands out work. Given a timeout, in seconds, it waits that long
         * at most for FrankenPHP to need the script, without a limit when
         * null: all a script with nothing else to wait on needs. Without
         * one it never waits: the script waits on the handle, or on the
         * stream of getStream(), and calls this when it is readable, which
         * also consumes whatever FrankenPHP wrote there.
         */
        public function tick(?float $timeout = 0.0): bool {}

        /**
         * The stream to wait on, alone or with the script's own streams: it
         * becomes readable when FrankenPHP needs the script's attention,
         * its drain included. Only waiting on it is supported, through
         * stream_select() or a blocking read; what it carries is not part
         * of the contract and tick() consumes it. Reading it steals those
         * bytes from tick(), and throws before the first tick() when the
         * read would block. Writing to it goes nowhere. Closing it is
         * safe: a handle hands out one stream, a fresh one over the same
         * socket once the script closed it.
         *
         * @return resource
         */
        public function getStream() {}

        /**
         * Returns the namespaces this background worker serves, as declared in
         * its configuration.
         *
         * @return array{vars: list<string>, tasks: list<string>}
         */
        public function getNamespaces(): array {}

        /**
         * Publishes a snapshot in one of the vars namespaces of this background
         * worker: the array replaces the previous snapshot of that namespace,
         * atomically for readers, which get copies. Values must be null,
         * scalars, arrays or enums. Every declared namespace is published
         * before the first tick() of a run.
         */
        public function setVars(string $namespace, array $vars): void {}

        /**
         * Dequeues a task sent to this background worker, or null when there
         * is none. It never waits: each task sent wakes one thread of the
         * worker, in tick() when it waits there or through its handle, so a
         * script calls this once tick() returned; a wake-up is not a count,
         * and null after one is expected in a pool.
         */
        public function receive(): ?ReceivedTaskHandle {}
    }

    /**
     * EXPERIMENTAL: the sender's side of a task, an Io\Poll\Handle on PHP
     * 8.6. Constructing it queues the task for the background worker
     * receiving the task namespace, resolved within the current php_server
     * then among global workers, and returns without waiting: the task stays
     * queued for as long as its handle lives, and nothing else bounds the
     * queue. A thread has $pickupTimeout seconds from there to pick the task
     * up, forever when null: past it the task leaves the queue and read()
     * throws. Payload values must be null, scalars, arrays or enums.
     *
     * @strict-properties
     * @not-serializable
     */
    final class SentTaskHandle
    {
        public function __construct(string $namespace, array $payload, ?float $pickupTimeout = 30.0) {}

        /**
         * The next update published by the worker, waiting for it, or null
         * once the task is complete. Throws when no thread picked the task up
         * in time and when the worker let go of it without completing it.
         * The wait has no timeout: one that must end goes through a poll
         * context or stream_select() first, which take one.
         */
        public function read(): ?array {}

        /**
         * The stream to wait on, alone or with other tasks: it becomes
         * readable when an update is there and reaches EOF when the task
         * ends. Only waiting on it is supported: a read on it returns
         * nothing, at once, the updates come through read(). Closing it is
         * safe: the handle hands out a fresh one. Past the end of the task,
         * the stream the script took last comes back closed, and asking for a
         * first one throws.
         *
         * @return resource
         */
        public function getStream() {}

        /**
         * Gives up on the task: it leaves the queue if no thread picked it up
         * yet, and the worker's next update() returns false otherwise.
         * Dropping the handle does the same.
         */
        public function abandon(): void {}
    }

    /**
     * EXPERIMENTAL: the worker's side of a task, handed out by
     * WorkerHandle::receive(), an Io\Poll\Handle on PHP 8.6. Only complete()
     * completes the task: abort(), dropping the handle or ending the run
     * aborts it, which the sender is told about.
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
         * The task namespace the sender addressed, one of those the worker
         * declares.
         */
        public function getNamespace(): string {}

        /**
         * Publishes an update for the sender, which reads it with read().
         * Values must be null, scalars, arrays or enums. It never waits:
         * the updates the sender has not read yet are kept for it, without
         * a bound. False when the sender gave up: the update went nowhere.
         */
        public function update(array $data): bool {}

        /**
         * Completes the task, with a last update when one is given: the
         * sender's read() returns it, then null. It never waits. False when
         * the sender gave up.
         */
        public function complete(?array $data = null): bool {}

        /**
         * Lets go of the task without completing it: the sender's read()
         * throws. Dropping the handle does the same, once nothing holds it
         * anymore.
         */
        public function abort(): void {}

        /**
         * The stream to wait on: it reaches EOF when the sender abandons the
         * task, for stream_select() and feof(). Only waiting on it is
         * supported. Closing it is safe: the handle hands out a fresh one.
         * Past the end of the task, the stream the script took last comes
         * back closed, and asking for a first one throws.
         *
         * @return resource
         */
        public function getStream() {}
    }
}
