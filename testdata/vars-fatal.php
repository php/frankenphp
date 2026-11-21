<?php

// Reads the vars of enum-vars.php and prints the count of publications they
// carry, autoloading the class of their enum. With fatal, the autoloader dies
// with a fatal error instead, which bails out of the copy.
spl_autoload_register(function (): void {
    if (isset($_GET['fatal'])) {
        eval('function strlen() {}');
    }
    require __DIR__.'/enum-suit.php';
});
echo frankenphp_get_vars('enum')['n'];
