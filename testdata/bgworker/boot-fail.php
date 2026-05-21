<?php

// Bootstrap-failure worker: throws before ever calling frankenphp_set_vars.
// Used by ensure() bootstrap-mode tests to verify the captured PHP error
// surfaces in the timeout message.
throw new RuntimeException("intentional boot failure for test");
