package testintegration

// #include <Zend/zend_types.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

// export_php:function my_array_map(array $data, callable $callback): array
func my_array_map(arr *C.zend_array, callback *C.zval) unsafe.Pointer {
	goArray, err := frankenphp.GoPackedArray[any](unsafe.Pointer(arr))
	if err != nil {
		return nil
	}

	result := make([]any, len(goArray))
	for i, item := range goArray {
		callResult := frankenphp.CallPHPCallable(unsafe.Pointer(callback), []any{item})
		result[i] = callResult
	}

	return frankenphp.PHPPackedArray[any](result)
}

// export_php:function my_filter(array $data, ?callable $callback): array
func my_filter(arr *C.zend_array, callback *C.zval) unsafe.Pointer {
	goArray, err := frankenphp.GoPackedArray[any](unsafe.Pointer(arr))
	if err != nil {
		return nil
	}

	if callback == nil {
		//return unsafe.Pointer(arr) // returning the original array requires GC_ADDREF
		return frankenphp.PHPPackedArray[any](goArray)
	}

	result := make([]any, 0)
	for _, item := range goArray {
		callResult := frankenphp.CallPHPCallable(unsafe.Pointer(callback), []any{item})
		if boolResult, ok := callResult.(bool); ok && boolResult {
			result = append(result, item)
		}
	}

	return frankenphp.PHPPackedArray[any](result)
}

// export_php:class Processor
type Processor struct{}

// export_php:method Processor::transform(string $input, callable $transformer): string
func (p *Processor) Transform(input *C.zend_string, callback *C.zval) unsafe.Pointer {
	goInput := frankenphp.GoString(unsafe.Pointer(input))

	callResult := frankenphp.CallPHPCallable(unsafe.Pointer(callback), []any{goInput})

	resultStr, ok := callResult.(string)
	if !ok {
		return unsafe.Pointer(input)
	}

	return frankenphp.PHPString(resultStr, false)
}
