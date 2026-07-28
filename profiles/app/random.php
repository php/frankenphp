<?php
$random = new \Random\Randomizer(new \Random\Engine\Xoshiro256StarStar());
for ($i = 0; $i < 50; $i++) {
    echo $random->getBytesFromString("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", 1023), "\n";
}
