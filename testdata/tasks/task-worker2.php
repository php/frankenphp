<?php

$handleFunc = function ($task) {
    echo "$task";
};

while(frankenphp_handle_task($handleFunc)) {
    // Keep handling tasks until there are no more tasks or the max limit is reached
}