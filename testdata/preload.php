<?php

// verify ENV can be accessed during preload
$_ENV['TEST'] = '123';
function preloaded_function(): string
{
    return 'I am preloaded';
}
