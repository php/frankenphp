<?php

// Custom session handler defined BEFORE the worker loop
class PreLoopSessionHandler implements SessionHandlerInterface
{
    private static array $data = [];

    public function open(string $path, string $name): bool
    {
        return true;
    }

    public function close(): bool
    {
        return true;
    }

    public function read(string $id): string|false
    {
        return self::$data[$id] ?? '';
    }

    public function write(string $id, string $data): bool
    {
        self::$data[$id] = $data;
        return true;
    }

    public function destroy(string $id): bool
    {
        unset(self::$data[$id]);
        return true;
    }

    public function gc(int $max_lifetime): int|false
    {
        return 0;
    }
}

// Set the session handler BEFORE the worker loop
$handler = new PreLoopSessionHandler();
session_set_save_handler($handler, true);

$requestCount = 0;

do {
    $ok = frankenphp_handle_request(function () use (&$requestCount): void {
        $requestCount++;
        $output = [];
        $output[] = "request=$requestCount";

        $action = $_GET['action'] ?? 'check';

        switch ($action) {
            case 'use_session':
                // Try to use the session - should work with pre-loop handler
                $error = null;
                set_error_handler(function ($errno, $errstr) use (&$error) {
                    $error = $errstr;
                    return true;
                });

                try {
                    session_id('test-preloop-' . $requestCount);
                    $result = session_start();
                    if ($result) {
                        $_SESSION['test'] = 'value-' . $requestCount;
                        session_write_close();
                        $output[] = "SESSION_OK";
                    } else {
                        $output[] = "SESSION_START_FAILED";
                    }
                } catch (Throwable $e) {
                    $output[] = "EXCEPTION:" . $e->getMessage();
                }

                restore_error_handler();

                if ($error) {
                    $output[] = "ERROR:" . $error;
                }
                break;

            case 'check':
            default:
                $saveHandler = ini_get('session.save_handler');
                $output[] = "save_handler=$saveHandler";
                if ($saveHandler === 'user') {
                    $output[] = "HANDLER_PRESERVED";
                } else {
                    $output[] = "HANDLER_LOST";
                }
        }

        echo implode("\n", $output);
    });
} while ($ok);
