<?php

require_once __DIR__.'/_executor.php';

return function () {
    require __DIR__ .'/require.php';
    opcache_reset();
    echo "opcache reset done";
};
