<?php

// simulate Symfony's dd() behavior
class Dumper
{
    private string $message;

    public function dump(string $message): void
    {
        http_response_code(500);
        $this->message = $message;
    }

    public function __destruct()
    {
        if (isset($this->message)) {
            echo $this->message;
        }
    }
}

$dumper = new Dumper();

while (frankenphp_handle_request(function () use ($dumper) {
    $dumper->dump($_GET['output']);
    exit(1);
})) {
    // noop
}