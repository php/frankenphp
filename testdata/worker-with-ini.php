<?php

// Modify INI values BEFORE the worker loop (simulating framework setup)
$preLoopPrecision = '8';
$preLoopDisplayErrors = '0';

ini_set('precision', $preLoopPrecision);
ini_set('display_errors', $preLoopDisplayErrors);

$requestCount = 0;

do {
    $ok = frankenphp_handle_request(function () use (
        &$requestCount,
        $preLoopPrecision,
        $preLoopDisplayErrors
    ): void {
        $requestCount++;
        $output = [];
        $output[] = "request=$requestCount";

        $action = $_GET['action'] ?? 'check';

        switch ($action) {
            case 'change_ini':
                // Change INI values during the request
                ini_set('precision', '5');
                ini_set('display_errors', '1');
                $output[] = "precision=" . ini_get('precision');
                $output[] = "display_errors=" . ini_get('display_errors');
                $output[] = "INI_CHANGED";
                break;

            case 'check':
            default:
                // Check if pre-loop INI values are preserved
                $precision = ini_get('precision');
                $displayErrors = ini_get('display_errors');

                $output[] = "precision=$precision";
                $output[] = "display_errors=$displayErrors";

                $issues = [];
                if ($precision !== $preLoopPrecision) {
                    $issues[] = "precision mismatch (expected $preLoopPrecision)";
                }
                if ($displayErrors !== $preLoopDisplayErrors) {
                    $issues[] = "display_errors mismatch (expected $preLoopDisplayErrors)";
                }

                if (empty($issues)) {
                    $output[] = "PRELOOP_INI_PRESERVED";
                } else {
                    $output[] = "PRELOOP_INI_LOST";
                    foreach ($issues as $issue) {
                        $output[] = "ISSUE: $issue";
                    }
                }
        }

        echo implode("\n", $output);
    });
} while ($ok);
