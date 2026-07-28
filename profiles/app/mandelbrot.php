<?php
$start = microtime(true);

$width = 80;
$height = 40;
$maxIter = 100;

$xmin = -2.5;
$xmax = 1.0;
$ymin = -1.0;
$ymax = 1.0;

$chars = ' .:-=+*#%@';

for ($y = 0; $y < $height; $y++) {
    for ($x = 0; $x < $width; $x++) {
        $cx = $xmin + ($x / $width) * ($xmax - $xmin);
        $cy = $ymin + ($y / $height) * ($ymax - $ymin);

        $zx = 0;
        $zy = 0;
        $iter = 0;

        while ($zx * $zx + $zy * $zy < 4 && $iter < $maxIter) {
            $tmp = $zx * $zx - $zy * $zy + $cx;
            $zy = 2 * $zx * $zy + $cy;
            $zx = $tmp;
            $iter++;
        }

        $charIndex = min(strlen($chars) - 1, (int)(($iter / $maxIter) * strlen($chars)));
        echo $chars[$charIndex];
    }
    echo "\n";
}

printf("Mandelbrot %dx%d iter=%d time=%.4fs\n", $width, $height, $maxIter, microtime(true) - $start);
