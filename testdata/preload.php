<?php

function preloaded_function(): string {
    $_SERVER['TEST'] = '123';
    putenv('TEST=123');
    return 'I am preloaded';
}
