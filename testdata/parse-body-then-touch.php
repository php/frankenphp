<?php

try {
    request_parse_body();
    touch($_GET['marker']);
} catch (FrankenPHP\RequestBodyException) {
    touch($_GET['marker'] . '-caught');
}
