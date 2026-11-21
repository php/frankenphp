<?php

// Two fibers read a namespace each, and the autoloader of their enums suspends
// them in the middle of the copy: each has to let go of the snapshot it
// holds, whichever resumes first, while the publishers keep replacing them.
spl_autoload_register(function (): void {
    Fiber::suspend();
    require_once __DIR__.'/enum-suit.php';
});
$a = new Fiber(fn () => frankenphp_get_vars('enum-a')['suit']->name);
$b = new Fiber(fn () => frankenphp_get_vars('enum-b')['suit']->name);
$a->start();
$b->start();
$a->resume();
// time for the publisher of the second snapshot to replace it
usleep(100_000);
$b->resume();
echo $a->getReturn(), ' ', $b->getReturn();
