<?php

require_once __DIR__.'/_executor.php';

return function () {
    // Only access $_REQUEST on requests where use_request=1 is passed
    // This tests the "re-arm" scenario where $_REQUEST might be accessed
    // for the first time during a later request
    if (isset($_GET['use_request']) && $_GET['use_request'] === '1') {
        include 'request-superglobal-conditional-include.php';
    } else {
        echo "SKIPPED";
    }
    echo "\nGET:";
    var_export($_GET);
};
