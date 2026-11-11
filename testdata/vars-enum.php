<?php

// Reads the enum of enum-vars.php through an autoloader that keeps the class
// name it is given, then samples that name while the publications go on.
spl_autoload_register(function (string $class): void {
    $GLOBALS['kept'] = $class;
    require __DIR__.'/enum-suit.php';
});
$suit = frankenphp_get_vars('enum')['suit'];
$seen = [];
for ($i = 0; $i < 50; ++$i) {
    usleep(3000);
    $seen[bin2hex($GLOBALS['kept'])] = true;
}
echo $suit->name, ' kept:', implode(',', array_map('hex2bin', array_keys($seen)));
