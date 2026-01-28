<?php

require_once __DIR__.'/_executor.php';

return function () {
    $action = $_GET['action'] ?? 'check';
    $output = [];

    switch ($action) {
        case 'set':
            // Set a value in session
            session_start();
            $_SESSION['secret'] = $_GET['value'] ?? 'default_secret';
            $_SESSION['client_id'] = $_GET['client_id'] ?? 'unknown';
            session_write_close();
            $output[] = 'SESSION_SET';
            $output[] = 'secret=' . $_SESSION['secret'];
            break;

        case 'get':
            // Read session and return values
            session_start();
            $output[] = 'SESSION_READ';
            $output[] = 'secret=' . ($_SESSION['secret'] ?? 'NOT_FOUND');
            $output[] = 'client_id=' . ($_SESSION['client_id'] ?? 'NOT_FOUND');
            $output[] = 'session_id=' . session_id();
            session_write_close();
            break;

        case 'set_and_exit':
            // Set a value in session and exit without calling session_write_close
            session_start();
            $_SESSION['secret'] = $_GET['value'] ?? 'exit_secret';
            $_SESSION['client_id'] = $_GET['client_id'] ?? 'exit_client';
            // Intentionally NOT calling session_write_close() before exit
            $output[] = 'BEFORE_EXIT';
            echo implode("\n", $output);
            flush();
            exit(1);
            break;

        case 'check_empty':
            // Check that session is empty (no leak from other clients)
            // Note: We intentionally do NOT call session_start() here.
            // $_SESSION should be empty without starting a session.
            // If data leaks from previous requests, this test will catch it.
            $output[] = 'SESSION_CHECK';
            if (empty($_SESSION)) {
                $output[] = 'SESSION_EMPTY=true';
            } else {
                $output[] = 'SESSION_EMPTY=false';
                $output[] = 'LEAKED_DATA=' . json_encode($_SESSION);
            }
            $output[] = 'session_id=' . session_id();
            break;

        default:
            $output[] = 'UNKNOWN_ACTION';
    }

    echo implode("\n", $output);
};
