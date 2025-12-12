<?php

require_once __DIR__.'/../_executor.php';

return function() {
    $var = 'MY_VAR_' . ($_GET['var'] ?? '');
    // Setting an environment variable
    $result = putenv("$var=HelloWorld");
    if ($result) {
        echo "Set MY_VAR successfully.\n";
        echo "MY_VAR = " . getenv($var) . "\n";
    } else {
        echo "Failed to set MY_VAR.\n";
    }

    // verify putenv does not affect $_SERVER
    $result = $_SERVER[$var] ?? null;
    if ($result !== null) {
        echo "MY_VAR is in \$_SERVER (not expected)\n";
    } else {
        echo "MY_VAR not found in \$_SERVER.\n";
    }

    // Unsetting the environment variable
    $result = putenv($var);
    if ($result) {
        echo "Unset MY_VAR successfully.\n";
        $value = getenv($var);
        if ($value === false) {
            echo "MY_VAR is unset.\n";
        } else {
            echo "MY_VAR = " . $value . "\n";
        }
    } else {
        echo "Failed to unset MY_VAR.\n";
    }

    $result = putenv("$var=");
    if ($result) {
        echo "MY_VAR set to empty successfully.\n";
        $value = getenv($var);
        if ($value === false) {
            echo "MY_VAR is unset.\n";
        } else {
            echo "MY_VAR = " . $value . "\n";
        }
    } else {
        echo "Failed to set MY_VAR.\n";
    }

    // Attempt to unset a non-existing variable
    $result = putenv('NON_EXISTING_VAR' . ($_GET['var'] ?? ''));
    if ($result) {
        echo "Unset NON_EXISTING_VAR successfully.\n";
    } else {
        echo "Failed to unset NON_EXISTING_VAR.\n";
    }

    // Inserting an invalid variable should fail (null byte in key)
    putenv("INVALID\x0_VAR=value");
    if (getenv("INVALID\x0_VAR")) {
        echo "Invalid value was inserted (unexpected).\n";
    } else {
        echo "Invalid value was not inserted.\n";
    }

    getenv();
};
