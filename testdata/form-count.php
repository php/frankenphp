<?php
require_once __DIR__.'/_executor.php';

return function () {
    echo count($_POST, COUNT_RECURSIVE), ' ', count($_FILES);
};
