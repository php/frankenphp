<?php

// SidekickOnlyEnum is NOT defined here - get_vars should throw LogicException

frankenphp_handle_request(function () {
    try {
        $vars = frankenphp_worker_get_vars('enum-missing');
        echo 'no_error';
    } catch (\LogicException $e) {
        echo 'LogicException:' . $e->getMessage();
    } catch (\Throwable $e) {
        echo 'other:' . get_class($e);
    }
});
