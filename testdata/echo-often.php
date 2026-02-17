<?php

header('Content-Type: text/plain');

for ($i = 0; $i < $_GET['count']; $i++) {
    echo ",";
}
