<?php

// asks for a reboot of every thread while booting, then fails before
// reaching frankenphp_handle_request()
opcache_reset();

exit(1);
