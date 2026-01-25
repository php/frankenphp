package testintegration

// #include <Zend/zend_types.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

// export_php:class Counter
type CounterStruct struct {
	Value int
}

// export_php:method Counter::increment(): void
func (c *CounterStruct) Increment() {
	c.Value++
}

// export_php:method Counter::decrement(): void
func (c *CounterStruct) Decrement() {
	c.Value--
}

// export_php:method Counter::getValue(): int
func (c *CounterStruct) GetValue() int64 {
	return int64(c.Value)
}

// export_php:method Counter::setValue(int $value): void
func (c *CounterStruct) SetValue(value int64) {
	c.Value = int(value)
}

// export_php:method Counter::reset(): void
func (c *CounterStruct) Reset() {
	c.Value = 0
}

// export_php:method Counter::addValue(int $amount): int
func (c *CounterStruct) AddValue(amount int64) int64 {
	c.Value += int(amount)
	return int64(c.Value)
}

// export_php:method Counter::updateWithNullable(?int $newValue): void
func (c *CounterStruct) UpdateWithNullable(newValue *int64) {
	if newValue != nil {
		c.Value = int(*newValue)
	}
}

// export_php:class StringHolder
type StringHolderStruct struct {
	Data string
}

// export_php:method StringHolder::setData(string $data): void
func (sh *StringHolderStruct) SetData(data *C.zend_string) {
	sh.Data = frankenphp.GoString(unsafe.Pointer(data))
}

// export_php:method StringHolder::getData(): string
func (sh *StringHolderStruct) GetData() unsafe.Pointer {
	return frankenphp.PHPString(sh.Data, false)
}

// export_php:method StringHolder::getLength(): int
func (sh *StringHolderStruct) GetLength() int64 {
	return int64(len(sh.Data))
}
