package testintegration

// #include <Zend/zend_types.h>
import "C"

// export_php:function invalid_return_type(string $str): unsupported_type
func invalid_return_type(s *C.zend_string) int {
	return 42
}
