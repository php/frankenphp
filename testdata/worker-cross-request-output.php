<?php

final class Secret
{
    /** @var list<string> */
    public static array $destroyed = [];

    public function __construct(public string $value)
    {
    }

    public function __destruct()
    {
        self::$destroyed[] = $this->value;
        echo "destroyed {$this->value}\n";
    }
}

final class ThrowingSecret
{
    public function __construct(public string $value)
    {
    }

    public function __destruct()
    {
        throw new Exception("destroyed {$this->value}");
    }
}

// its destructor puts request data back into superglobals released before it
final class Mover
{
    public function __construct(public string $value)
    {
    }

    public function __destruct()
    {
        $_GET['moved'] = new Secret("moved {$this->value}");
        $_SESSION = ['tenant' => $this->value];
    }
}

final class TenantSessionHandler implements SessionHandlerInterface, SessionIdInterface, SessionUpdateTimestampHandlerInterface
{
    public function open(string $path, string $name): bool
    {
        return true;
    }

    public function close(): bool
    {
        return true;
    }

    public function read(string $id): string
    {
        return '';
    }

    public function write(string $id, string $data): bool
    {
        echo 'write tenant: ', $_SESSION['tenant'] ?? 'none', "\n";

        return true;
    }

    public function destroy(string $id): bool
    {
        return true;
    }

    public function gc(int $max_lifetime): int
    {
        return 0;
    }

    public function create_sid(): string
    {
        return bin2hex(random_bytes(16));
    }

    public function validateId(string $id): bool
    {
        return true;
    }

    public function updateTimestamp(string $id, string $data): bool
    {
        return true;
    }
}

$between = null;

while (frankenphp_handle_request(static function () use (&$between): void {
    if (isset($_GET['session'])) {
        session_start();
        $_SESSION['secret'] = new Secret($_GET['session']);
    }

    if (isset($_GET['tenant'])) {
        session_set_save_handler(new TenantSessionHandler(), false);
        session_start();
        $_SESSION['tenant'] = $_GET['tenant'];
    }

    if (isset($_GET['superglobals'])) {
        Secret::$destroyed = [];
        $_SERVER['secret'] = new Secret('server');
        $_GET['secret'] = new Secret('get');
        $_POST['secret'] = new Secret('post');
        $_COOKIE['secret'] = new Secret('cookie');
        $_FILES['secret'] = new Secret('files');
        $_REQUEST['secret'] = new Secret('request');
        $_ENV['secret'] = new Secret('env');
    }

    if (isset($_GET['throw'])) {
        $_GET['secret'] = new ThrowingSecret($_GET['throw']);
    }

    if (isset($_GET['move'])) {
        $_FILES['mover'] = new Mover($_GET['move']);
    }

    if (isset($_GET['between'])) {
        Secret::$destroyed = [];
        $between = $_GET['between'];
    }

    if (isset($_GET['destroyed'])) {
        $destroyed = Secret::$destroyed;
        sort($destroyed);
        echo 'destroyed: ', implode(',', $destroyed), "\n";
        echo 'session: ', isset($_SESSION) ? 'set' : 'unset', "\n";

        // $_ENV is worker state: its objects outlive the request
        $env = $_ENV['secret'] ?? null;
        unset($_ENV['secret']);
        echo 'env: ', $env?->value ?? 'none', "\n";
    }

    echo "done\n";
})) {
    // code running between two requests, like a framework's terminate()
    if ($between !== null) {
        $_SESSION = ['secret' => new Secret($between)];
        $between = null;
    }
}
