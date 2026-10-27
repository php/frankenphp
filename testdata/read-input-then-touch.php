<?php

file_get_contents('php://input');
try {
    touch($_GET['marker']);
} finally {
    touch($_GET['marker']);
}
