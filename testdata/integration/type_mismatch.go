package testintegration

// #include <Zend/zend_types.h>
import "C"

// export_php:function mismatched_param_type(int $value): int
func mismatched_param_type(value string) int64 {
	return 0
}

// export_php:class BadClass
type BadClassStruct struct {
	Value int
}

// export_php:method BadClass::wrongReturnType(): string
func (bc *BadClassStruct) WrongReturnType() int {
	return bc.Value
}
