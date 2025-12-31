package testintegration

// #include <Zend/zend_types.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

// export_php:const
const TEST_MAX_RETRIES = 100

// export_php:const
const TEST_API_VERSION = "2.0.0"

// export_php:const
const TEST_ENABLED = true

// export_php:const
const TEST_PI = 3.14159

// export_php:const
const STATUS_PENDING = iota

// export_php:const
const STATUS_PROCESSING = iota

// export_php:const
const STATUS_COMPLETED = iota

// export_php:class Config
type ConfigStruct struct {
	Mode int
}

// export_php:classconst Config
const MODE_DEBUG = 1

// export_php:classconst Config
const MODE_PRODUCTION = 2

// export_php:classconst Config
const DEFAULT_TIMEOUT = 30

// export_php:method Config::setMode(int $mode): void
func (c *ConfigStruct) SetMode(mode int64) {
	c.Mode = int(mode)
}

// export_php:method Config::getMode(): int
func (c *ConfigStruct) GetMode() int64 {
	return int64(c.Mode)
}

// export_php:function test_with_constants(int $status): string
func test_with_constants(status int64) unsafe.Pointer {
	var result string
	switch status {
	case STATUS_PENDING:
		result = "pending"
	case STATUS_PROCESSING:
		result = "processing"
	case STATUS_COMPLETED:
		result = "completed"
	default:
		result = "unknown"
	}
	return frankenphp.PHPString(result, false)
}
