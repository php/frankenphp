<?php

require_once __DIR__ . '/../_executor.php';

return function () {
    $var = 'MY_VAR_' . ($_GET['var'] ?? '');
    // Setting an environment variable
    $result = putenv("$var=HelloWorld");
    echo $result ? "Set MY_VAR successfully.\nMY_VAR = " . getenv($var) . "\n" : "Failed to set MY_VAR.\n";

    // putenv should not affect $_ENV
    $result = $_ENV[$var] ?? null;
    echo $result === null ? "MY_VAR not found in \$_ENV.\n" : "MY_VAR is in \$_ENV (not expected)\n";

    // putenv should not affect $_SERVER
    $result = $_SERVER[$var] ?? null;
    echo $result === null ? "MY_VAR not found in \$_SERVER.\n" : "MY_VAR is in \$_SERVER (not expected)\n";

    // Unsetting the environment variable
    $result = putenv($var);
    if ($result) {
        echo "Unset MY_VAR successfully.\n";
        $value = getenv($var);
        echo $value === false ? "MY_VAR is unset.\n" : "MY_VAR = $value\n";
    } else {
        echo "Failed to unset MY_VAR.\n";
    }

    $result = putenv("$var=");
    if ($result) {
        echo "MY_VAR set to empty successfully.\n";
        $value = getenv($var);
        echo $value === false ? "MY_VAR is unset.\n" : "MY_VAR = $value\n";
    } else {
        echo "Failed to set MY_VAR.\n";
    }

    // Attempt to unset a non-existing variable
    $result = putenv('NON_EXISTING_VAR' . ($_GET['var'] ?? ''));
    echo $result ? "Unset NON_EXISTING_VAR successfully.\n" : "Failed to unset NON_EXISTING_VAR.\n";

    // Inserting an invalid variable should fail (null byte in key)
    $result = putenv("INVALID\x0_VAR=value");
    if (getenv("INVALID\x0_VAR")) {
        echo "Invalid value was inserted (unexpected).\n";
    } else if ($result) {
        echo "Invalid value was not inserted.\n";
    } else {
        echo "Invalid value was not inserted, but regular PHP should still return 'true' here.\n";
    }

    getenv();
};
