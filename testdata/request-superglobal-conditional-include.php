<?php
// This file tests if $_REQUEST is properly re-initialized in worker mode
// The key test is: does $_REQUEST contain ONLY the current request's data?

// Static counter to track how many times this file is executed (not compiled)
static $execCount = 0;
$execCount++;

echo "EXEC_COUNT:" . $execCount;
echo "\nREQUEST:";
var_export($_REQUEST);
echo "\nREQUEST_COUNT:" . count($_REQUEST);

// Check if $_REQUEST was properly initialized for this request
// If stale, it might have data from a previous request
if (isset($_GET['val'])) {
    $expected_val = $_GET['val'];
    $actual_val = $_REQUEST['val'] ?? 'MISSING';
    echo "\nVAL_CHECK:" . ($expected_val === $actual_val ? "MATCH" : "MISMATCH(expected=$expected_val,actual=$actual_val)");
}
