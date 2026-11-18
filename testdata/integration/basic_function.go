package testintegration

// #include <Zend/zend_types.h>
import "C"
import (
	"strconv"
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

// export_php:function test_nullables(?int $count, ?float $ratio, ?bool $flag): string
func test_nullables(count *int64, ratio *float64, flag *bool) unsafe.Pointer {
	values := []string{"null", "null", "null"}
	if count != nil {
		values[0] = strconv.FormatInt(*count, 10)
	}
	if ratio != nil {
		values[1] = strconv.FormatFloat(*ratio, 'f', -1, 64)
	}
	if flag != nil {
		values[2] = strconv.FormatBool(*flag)
	}

	return frankenphp.PHPString(strings.Join(values, " "), false)
}
