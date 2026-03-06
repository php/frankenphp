package testintegration

// #include <Zend/zend_types.h>
import "C"
import (
	"strings"
	"unsafe"

	"github.com/dunglas/frankenphp"
)

// export_php:function test_uppercase(string $str): string
func test_uppercase(s *C.zend_string) unsafe.Pointer {
	str := frankenphp.GoString(unsafe.Pointer(s))
	upper := strings.ToUpper(str)
	return frankenphp.PHPString(upper, false)
}

// export_php:function test_add_numbers(int $a, int $b): int
func test_add_numbers(a int64, b int64) int64 {
	return a + b
}

// export_php:function test_multiply(float $a, float $b): float
func test_multiply(a float64, b float64) float64 {
	return a * b
}

// export_php:function test_is_enabled(bool $flag): bool
func test_is_enabled(flag bool) bool {
	return !flag
}
