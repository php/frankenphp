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
        echo "WRITING SESSION: id=$id, data=$data\n";
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

do {
    $ok = frankenphp_handle_request(function () use ($handler): void {
        $action = $_GET['action'] ?? 'check';

        switch ($action) {
            case 'put_session':
                session_set_save_handler($handler, true);
                session_start();
                $_SESSION['test'] = 'session value exists';
                echo 'session value set';
                break;
            case 'read_session':
                session_set_save_handler($handler, true);
                session_start();
                echo 'session id:' . session_id();
                echo $_SESSION['test'] ?? 'no session value';
                break;

            case 'check':
            default:
                echo $_SESSION['test'] ?? 'no session value';
                break;
        }
    });
} while ($ok);
