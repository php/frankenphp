<?php

// Reads the enum of enum-vars.php with a class of its own: EnumSuit has no
// Hearts case here, ?constant makes Hearts a plain constant instead.
if (isset($_GET['constant'])) {
    eval('enum EnumSuit: string { case Clubs = "c"; const Hearts = self::Clubs; }');
} else {
    eval('enum EnumSuit: string { case Clubs = "c"; }');
}

try {
    var_export(frankenphp_get_vars('enum'));
} catch (\Throwable $e) {
    echo get_class($e), ': ', $e->getMessage();
}
