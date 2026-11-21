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
    }
}
