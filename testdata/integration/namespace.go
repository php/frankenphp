package testintegration

// export_php:namespace TestIntegration\Extension

// #include <Zend/zend_types.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

// export_php:const
const NAMESPACE_VERSION = "1.0.0"

// export_php:function greet(string $name): string
func greet(name *C.zend_string) unsafe.Pointer {
	str := frankenphp.GoString(unsafe.Pointer(name))
	result := "Hello, " + str + "!"
	return frankenphp.PHPString(result, false)
}

// export_php:class Person
type PersonStruct struct {
	Name string
	Age  int
}

// export_php:method Person::setName(string $name): void
func (p *PersonStruct) SetName(name *C.zend_string) {
	p.Name = frankenphp.GoString(unsafe.Pointer(name))
}

// export_php:method Person::getName(): string
func (p *PersonStruct) GetName() unsafe.Pointer {
	return frankenphp.PHPString(p.Name, false)
}

// export_php:method Person::setAge(int $age): void
func (p *PersonStruct) SetAge(age int64) {
	p.Age = int(age)
}

// export_php:method Person::getAge(): int
func (p *PersonStruct) GetAge() int64 {
	return int64(p.Age)
}

// export_php:classconst Person
const DEFAULT_AGE = 18
