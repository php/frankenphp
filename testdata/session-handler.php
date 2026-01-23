<?php

require_once __DIR__.'/_executor.php';

// Custom session handler class
class TestSessionHandler implements SessionHandlerInterface
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

return function () {
    $action = $_GET['action'] ?? 'default';

    // Collect output, don't send until end
    $output = [];

    switch ($action) {
        case 'set_handler_and_start':
            // Set custom handler and start session
            $handler = new TestSessionHandler();
            session_set_save_handler($handler, true);
            session_id('test-session-id');
            session_start();
            $_SESSION['value'] = $_GET['value'] ?? 'none';
            session_write_close();
            $output[] = "HANDLER_SET_AND_STARTED";
            $output[] = "session.save_handler=" . ini_get('session.save_handler');
            break;

        case 'start_without_handler':
            // Try to start session without setting handler
            // This should use the default handler (files) but in worker mode
            // the INI session.save_handler might still be "user" from previous request
            $saveHandlerBefore = ini_get('session.save_handler');
            $error = null;
            $exception = null;
            $result = false;

            // Capture any errors
            set_error_handler(function($errno, $errstr) use (&$error) {
                $error = $errstr;
                return true;
            });

            try {
                session_id('test-session-id-2');
                $result = session_start();
                if ($result) {
                    session_write_close();
                }
            } catch (Throwable $e) {
                $exception = $e->getMessage();
            }

            restore_error_handler();

            // Now output everything
            $output[] = "save_handler_before=" . $saveHandlerBefore;
            $output[] = "SESSION_START_RESULT=" . ($result ? "true" : "false");
            if ($error) {
                $output[] = "ERROR:" . $error;
            }
            if ($exception) {
                $output[] = "EXCEPTION:" . $exception;
            }
            break;

        case 'just_start':
            // Simple session start without any custom handler
            // This should always work
            session_id('test-session-id-3');
            session_start();
            $_SESSION['test'] = 'value';
            session_write_close();
            $output[] = "SESSION_STARTED_OK";
            break;

        default:
            $output[] = "UNKNOWN_ACTION";
    }

    echo implode("\n", $output);
};
