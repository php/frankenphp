<?php

/** @generate-class-entries */

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
 * @return array{
 *     "frankenphp_version": string,
 *     "current_thread_index": int,
 *     "is_worker_thread": bool,
 *     "threads": array {
 *         "index": int,
 *         "name": string,
 *         "state": string,
 *         "is_waiting": bool,
 *         "waiting_since_milliseconds": int,
 *     },
 * }
 */
function frankenphp_info(): array {}
