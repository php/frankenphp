<?php

// Long-lived bg worker: disable PHP max_execution_time so the 30s default
// cannot interrupt the stream_select park. The C side calls
// zend_unset_timeout() too, but the belt-and-suspenders here covers PHP
// builds where that path does not fully disarm the timer.
set_time_limit(0);

// Enum-missing fixture: defines an enum that lives only in the bg worker
// process image (the HTTP reader never loads this file), then publishes
// a case. On the reader side the class is unknown, so the generational
// deserializer must surface a LogicException instead of returning a
// broken zval.
enum WorkerOnlyEnum {
    case Foo;
}

frankenphp_set_vars(['val' => WorkerOnlyEnum::Foo]);

$stream = frankenphp_get_worker_handle();
$read = [$stream];
$write = null;
$except = null;
stream_select($read, $write, $except, null);
