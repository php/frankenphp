<?php

try {
    file_get_contents('php://input');
    touch($_GET['marker']);
} catch (FrankenPHP\RequestBodyTooLargeException) {
    touch($_GET['marker'] . '-caught');
}
