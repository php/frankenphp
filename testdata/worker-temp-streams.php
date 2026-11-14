<?php

$tmp = new SplTempFileObject();
$tmp->fwrite('hello');

$handler = static function () use ($tmp) {
    $tmp->rewind();
    echo $tmp->fgets(), ' ', count(get_resources('stream'));
};

while (frankenphp_handle_request($handler)) {
}
