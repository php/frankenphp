<?php

echo "<pre>\n";
foreach ([
             'CONTENT_LENGTH',
             'HTTP_CONTENT_LENGTH',
             'CONTENT_TYPE',
             'HTTP_CONTENT_TYPE',
             'HTTP_SPECIAL_CHARS',
             'DOCUMENT_ROOT',
             'DOCUMENT_URI',
             'GATEWAY_INTERFACE',
             'HTTP_HOST',
             'HTTPS',
             'PATH_INFO',
             'DOCUMENT_ROOT',
             'REMOTE_ADDR',
             'PHP_SELF',
             'REMOTE_HOST',
             'REQUEST_SCHEME',
             'SCRIPT_FILENAME',
             'SCRIPT_NAME',
             'SERVER_NAME',
             'SERVER_PORT',
             'SERVER_PROTOCOL',
             'SERVER_SOFTWARE',
             'SSL_PROTOCOL',
             'AUTH_TYPE',
             'REMOTE_IDENT',
             'PATH_TRANSLATED',
             'QUERY_STRING',
             'REMOTE_USER',
             'REQUEST_METHOD',
             'REQUEST_URI',
             'HTTP_X_EMPTY_HEADER',
         ] as $name) {
    echo "$name:" . $_SERVER[$name] . "\n";
}
echo "</pre>";
