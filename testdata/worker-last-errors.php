<?php

$hasLibxml = function_exists('libxml_use_internal_errors');

// set at worker boot: must survive the reset between requests
if ($hasLibxml) {
	libxml_use_internal_errors(true);
}

require_once __DIR__.'/_executor.php';

return function () use ($hasLibxml) {
	if (isset($_GET['leak'])) {
		@trigger_error("secret {$_GET['leak']}", E_USER_WARNING);
		if (class_exists(DOMDocument::class)) {
			(new DOMDocument())->loadXML("<root>&secret_{$_GET['leak']};</root>");
		}
		output_add_rewrite_var('token', "secret_{$_GET['leak']}");
	}

	echo json_encode([
		'last_error' => error_get_last()['message'] ?? null,
		'rewriter' => isset($_GET['leak']) ? null : output_add_rewrite_var('x', 'y') && in_array('URL-Rewriter', ob_list_handlers(), true),
		'libxml_errors' => $hasLibxml ? array_map(fn ($e) => trim($e->message), libxml_get_errors()) : null,
		'libxml_last_error' => $hasLibxml ? (libxml_get_last_error()->message ?? null) : null,
		'internal_errors' => $hasLibxml ? libxml_use_internal_errors() : null,
	]);
};
