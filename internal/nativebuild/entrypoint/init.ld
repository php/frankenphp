/* Go's library initializer requires argc/argv, which musl's constructor loop
 * does not supply (https://github.com/golang/go/issues/13492). Save only the Go
 * object's initializer for main() to call; leave C/C++ constructors alone. */
SECTIONS
{
  .frankenphp_go_init :
  {
    __frankenphp_go_init_start = .;
    KEEP(*frankenphp.a:go.o(.init_array))
    __frankenphp_go_init_end = .;
  }
}
INSERT BEFORE .init_array;
ASSERT(SIZEOF(.frankenphp_go_init) > 0, "Go archive initializer not found")
