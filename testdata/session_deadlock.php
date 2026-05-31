<?php

require_once __DIR__.'/_executor.php';

// Self-contained repro of https://github.com/php/frankenphp/issues/2368
if (!class_exists('StrictSessionHandler', false)) {
    abstract class AbstractSessionHandler implements SessionHandlerInterface, SessionUpdateTimestampHandlerInterface
    {
        private string $sessionName;
        private string $prefetchId;
        private string $prefetchData;
        private ?string $newSessionId = null;

        public function open(string $savePath, string $sessionName): bool
        {
            $this->sessionName = $sessionName;
            return true;
        }

        abstract protected function doRead(string $sessionId): string;
        abstract protected function doWrite(string $sessionId, string $data): bool;
        abstract protected function doDestroy(string $sessionId): bool;

        public function validateId(string $sessionId): bool
        {
            $this->prefetchData = $this->read($sessionId);
            $this->prefetchId = $sessionId;
            return '' !== $this->prefetchData;
        }

        public function read(string $sessionId): string
        {
            if (isset($this->prefetchId)) {
                $prefetchId = $this->prefetchId;
                $prefetchData = $this->prefetchData;
                unset($this->prefetchId, $this->prefetchData);
                if ($prefetchId === $sessionId || '' === $prefetchData) {
                    $this->newSessionId = '' === $prefetchData ? $sessionId : null;
                    return $prefetchData;
                }
            }
            $data = $this->doRead($sessionId);
            $this->newSessionId = '' === $data ? $sessionId : null;
            return $data;
        }

        public function write(string $sessionId, string $data): bool
        {
            $this->newSessionId = null;
            return $this->doWrite($sessionId, $data);
        }

        public function destroy(string $sessionId): bool
        {
            return $this->newSessionId === $sessionId || $this->doDestroy($sessionId);
        }
    }

    class StrictSessionHandler extends AbstractSessionHandler
    {
        public function __construct(private SessionHandlerInterface $handler) {}

        public function open(string $savePath, string $sessionName): bool
        {
            parent::open($savePath, $sessionName);
            return $this->handler->open($savePath, $sessionName);
        }

        public function updateTimestamp(string $sessionId, string $data): bool
        {
            return $this->write($sessionId, $data);
        }

        protected function doRead(string $sessionId): string
        {
            return $this->handler->read($sessionId);
        }

        protected function doWrite(string $sessionId, string $data): bool
        {
            return $this->handler->write($sessionId, $data);
        }

        protected function doDestroy(string $sessionId): bool
        {
            return $this->handler->destroy($sessionId);
        }

        public function close(): bool
        {
            return $this->handler->close();
        }

        public function gc(int $maxlifetime): int|false
        {
            return $this->handler->gc($maxlifetime);
        }
    }
}

return function () {
    // Pre-flock timer: any request that doesn't get the lock immediately will
    // have this fire while blocked in flock(), so when the lock is finally
    // released the request bails out *inside* StrictSessionHandler::doRead —
    // the path that leaks the fd without the fix from 2c6d2b5.
    set_time_limit(1);

    $savePath = sys_get_temp_dir() . '/frankenphp-session-deadlock';
    @mkdir($savePath);
    ini_set('session.use_strict_mode', '1');
    ini_set('session.save_path', $savePath);
    session_set_save_handler(new StrictSessionHandler(new SessionHandler()), true);
    session_id('testsession2368');
    session_start();

    // Reset the timer so the holder (and any waiter that successfully acquires
    // the lock) gets enough headroom to finish.
    set_time_limit(5);

    echo "Done.\n";
    flush();
    // Hold the lock long enough that a 1s timer expires for any concurrent
    // waiter still blocked in flock().
    system('sleep 2');
    echo "ok!\n";
};
