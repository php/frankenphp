<?php

require_once __DIR__.'/_executor.php';

return function () {
    // Output $_REQUEST to verify it contains current request data
    // $_REQUEST should be a merge of $_GET and $_POST (based on request_order)
    echo "REQUEST:";
    var_export($_REQUEST);
    echo "\nGET:";
    var_export($_GET);
    echo "\nPOST:";
    var_export($_POST);
};
