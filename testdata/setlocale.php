<?php

require_once __DIR__.'/_executor.php';

return function () {
    $initial = setlocale(LC_NUMERIC, '0');
    if ($_GET['locale'] === '0') {
        echo $initial;

        return;
    }

    // de-DE and fr-FR are the names on Windows
    $set = $_GET['locale'] === 'C'
        ? setlocale(LC_NUMERIC, 'C')
        : setlocale(LC_NUMERIC, ['de_DE.UTF-8', 'de_DE.utf8', 'de-DE', 'fr_FR.UTF-8', 'fr_FR.utf8', 'fr-FR', 'C.UTF-8']);
    // let concurrent requests change their locale meanwhile
    usleep(1000);
    $current = setlocale(LC_NUMERIC, '0');
    $float = sprintf('%.1f', 1.5);
    $decimalPoint = localeconv()['decimal_point'];

    // the composite name of all categories restores them
    $all = setlocale(LC_ALL, '0');
    setlocale(LC_ALL, 'C');
    $restored = setlocale(LC_ALL, $all) === $all && setlocale(LC_ALL, '0') === $all;

    echo implode('|', [$initial, $set, $current, $float, $decimalPoint, $restored ? 'restored' : 'not restored']);
};
