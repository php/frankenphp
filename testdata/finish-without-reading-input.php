<?php

frankenphp_finish_request();
usleep(200000); // let the Go handler return before shutdown drains the body
