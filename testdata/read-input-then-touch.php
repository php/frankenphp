<?php

try {
    file_get_contents('php://input');
    touch($_GET['marker']);
} catch (FrankenPHP\RequestBodyException) {
    touch($_GET['marker'] . '-caught');
}
