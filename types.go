package frankenphp

/*
#include "types.h"
*/
import "C"
import (
	"fmt"
	"strconv"
	"unsafe"
)

// EXPERIMENTAL: GoString copies a zend_string to a Go string.
func GoString(s unsafe.Pointer) string {
	if s == nil {
		return ""
	}

	zendStr := (*C.zend_string)(s)

	return C.GoStringN((*C.char)(unsafe.Pointer(&zendStr.val)), C.int(zendStr.len))
}

// EXPERIMENTAL: PHPString converts a Go string to a zend_string with copy. The string can be
// non-persistent (automatically freed after the request by the ZMM) or persistent. If you choose
// the second mode, it is your repsonsability to free the allocated memory.
func PHPString(s string, persistent bool) unsafe.Pointer {
	if s == "" {
		return nil
	}

	zendStr := C.zend_string_init(
		(*C.char)(unsafe.Pointer(unsafe.StringData(s))),
		C.size_t(len(s)),
		C._Bool(persistent),
	)

	return unsafe.Pointer(zendStr)
}

// AssociativeArray represents a PHP array with ordered key-value pairs
type AssociativeArray struct {
	Map   map[string]any
	Order []string
}

// EXPERIMENTAL: GoAssociativeArray converts a zend_array to a Go AssociativeArray
func GoAssociativeArray(arr unsafe.Pointer) AssociativeArray {
	entries, order := goArray(arr, true)
	return AssociativeArray{entries, order}
}

// EXPERIMENTAL: GoMap converts a zval having a zend_array value to an unordered Go map
func GoMap(arr unsafe.Pointer) map[string]any {
	entries, _ := goArray(arr, false)
	return entries
}

func goArray(arr unsafe.Pointer, ordered bool) (map[string]any, []string) {
	if arr == nil {
		panic("received a nil pointer on array conversion")
	}

	zval := (*C.zval)(arr)
	hashTable := (*C.HashTable)(extractZvalValue(zval, C.IS_ARRAY))

	if hashTable == nil {
		panic("received a *zval that wasn't a HashTable on array conversion")
	}

	nNumUsed := hashTable.nNumUsed
	entries := make(map[string]any, nNumUsed)
	var order []string
	if ordered {
		order = make([]string, 0, nNumUsed)
	}

	if htIsPacked(hashTable) {
		// if the HashTable is packed, convert all integer keys to strings
		// this is probably a bug by the dev using this function
		// still, we'll (inefficiently) convert to an associative array
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(hashTable, i)
			if v != nil && C.zval_get_type(v) != C.IS_UNDEF {
				strIndex := strconv.Itoa(int(i))
				entries[strIndex] = goValue(v)
				if ordered {
					order = append(order, strIndex)
				}
			}
		}

		return entries, order
	}

	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket == nil || C.zval_get_type(&bucket.val) == C.IS_UNDEF {
			continue
		}

		v := goValue(&bucket.val)

		if bucket.key != nil {
			keyStr := GoString(unsafe.Pointer(bucket.key))
			entries[keyStr] = v
			if ordered {
				order = append(order, keyStr)
			}

			continue
		}

		// as fallback convert the bucket index to a string key
		strIndex := strconv.Itoa(int(bucket.h))
		entries[strIndex] = v
		if ordered {
			order = append(order, strIndex)
		}
	}

	return entries, order
}

// EXPERIMENTAL: GoPackedArray converts a zval with a zend_array value to a Go slice
func GoPackedArray(arr unsafe.Pointer) []any {
	if arr == nil {
		panic("GoPackedArray received a nil pointer")
	}

	zval := (*C.zval)(arr)
	hashTable := (*C.HashTable)(extractZvalValue(zval, C.IS_ARRAY))

	if hashTable == nil {
		panic("GoPackedArray received *zval that wasn't a HashTable")
	}

	nNumUsed := hashTable.nNumUsed
	result := make([]any, 0, nNumUsed)

	if htIsPacked(hashTable) {
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(hashTable, i)
			if v != nil && C.zval_get_type(v) != C.IS_UNDEF {
				result = append(result, goValue(v))
			}
		}

		return result
	}

	// fallback if ht isn't packed - equivalent to array_values()
	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket != nil && C.zval_get_type(&bucket.val) != C.IS_UNDEF {
			result = append(result, goValue(&bucket.val))
		}
	}

	return result
}

// EXPERIMENTAL: PHPMap converts an unordered Go map to a PHP zend_array
func PHPMap(arr map[string]any) unsafe.Pointer {
	return phpArray(arr, nil)
}

// EXPERIMENTAL: PHPAssociativeArray converts a Go AssociativeArray to a PHP zval with a zend_array value
func PHPAssociativeArray(arr AssociativeArray) unsafe.Pointer {
	return phpArray(arr.Map, arr.Order)
}

func phpArray(entries map[string]any, order []string) unsafe.Pointer {
	var zendArray *C.HashTable

	if len(order) != 0 {
		zendArray = createNewArray((uint32)(len(order)))
		for _, key := range order {
			val := entries[key]
			zval := phpValue(val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), zval)
		}
	} else {
		zendArray = createNewArray((uint32)(len(entries)))
		for key, val := range entries {
			zval := phpValue(val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), zval)
		}
	}

	var zval C.zval
	C.__zval_arr__(&zval, zendArray)

	return unsafe.Pointer(&zval)
}

// EXPERIMENTAL: PHPPackedArray converts a Go slice to a PHP zval with a zend_array value.
func PHPPackedArray(slice []any) unsafe.Pointer {
	zendArray := createNewArray((uint32)(len(slice)))
	for _, val := range slice {
		zval := phpValue(val)
		C.zend_hash_next_index_insert(zendArray, zval)
	}

	var zval C.zval
	C.__zval_arr__(&zval, zendArray)

	return unsafe.Pointer(&zval)
}

// EXPERIMENTAL: GoValue converts a PHP zval to a Go value
func GoValue(zval unsafe.Pointer) any {
	return goValue((*C.zval)(zval))
}

func goValue(zval *C.zval) any {
	t := C.zval_get_type(zval)

	switch t {
	case C.IS_NULL:
		return nil
	case C.IS_FALSE:
		return false
	case C.IS_TRUE:
		return true
	case C.IS_LONG:
		longPtr := (*C.zend_long)(extractZvalValue(zval, C.IS_LONG))
		if longPtr != nil {
			return int64(*longPtr)
		}

		return int64(0)
	case C.IS_DOUBLE:
		doublePtr := (*C.double)(extractZvalValue(zval, C.IS_DOUBLE))
		if doublePtr != nil {
			return float64(*doublePtr)
		}

		return float64(0)
	case C.IS_STRING:
		str := (*C.zend_string)(extractZvalValue(zval, C.IS_STRING))
		if str == nil {
			return ""
		}

		return GoString(unsafe.Pointer(str))
	case C.IS_ARRAY:
		hashTable := (*C.HashTable)(extractZvalValue(zval, C.IS_ARRAY))
		if hashTable != nil && htIsPacked(hashTable) {
			return GoPackedArray(unsafe.Pointer(zval))
		}

		return GoAssociativeArray(unsafe.Pointer(zval))
	default:
		return nil
	}
}

// EXPERIMENTAL: PHPValue converts a Go any to a PHP zval
func PHPValue(value any) unsafe.Pointer {
	return unsafe.Pointer(phpValue(value))
}

func phpValue(value any) *C.zval {
	var zval C.zval

	switch v := value.(type) {
	case nil:
		C.__zval_null__(&zval)
	case bool:
		C.__zval_bool__(&zval, C._Bool(v))
	case int:
		C.__zval_long__(&zval, C.zend_long(v))
	case int64:
		C.__zval_long__(&zval, C.zend_long(v))
	case float64:
		C.__zval_double__(&zval, C.double(v))
	case string:
		str := (*C.zend_string)(PHPString(v, false))
		C.__zval_string__(&zval, str)
	case AssociativeArray:
		return (*C.zval)(PHPAssociativeArray(v))
	case map[string]any:
		return (*C.zval)(PHPAssociativeArray(AssociativeArray{Map: v}))
	case []any:
		return (*C.zval)(PHPPackedArray(v))
	default:
		panic(fmt.Sprintf("unsupported Go type %T", v))
	}

	return &zval
}

// createNewArray creates a new zend_array with the specified size.
func createNewArray(size uint32) *C.HashTable {
	arr := C.__zend_new_array__(C.uint32_t(size))
	return (*C.HashTable)(unsafe.Pointer(arr))
}

// htIsPacked checks if a HashTable is a list (packed) or hashmap (not packed).
func htIsPacked(ht *C.HashTable) bool {
	flags := *(*C.uint32_t)(unsafe.Pointer(&ht.u[0]))

	return (flags & C.HASH_FLAG_PACKED) != 0
}

// extractZvalValue returns a pointer to the zval value cast to the expected type
func extractZvalValue(zval *C.zval, expectedType C.uint8_t) unsafe.Pointer {
	if zval == nil || C.zval_get_type(zval) != expectedType {
		return nil
	}

	v := unsafe.Pointer(&zval.value[0])

	switch expectedType {
	case C.IS_LONG:
		return v
	case C.IS_DOUBLE:
		return v
	case C.IS_STRING:
		return unsafe.Pointer(*(**C.zend_string)(v))
	case C.IS_ARRAY:
		return unsafe.Pointer(*(**C.zend_array)(v))
	default:
		return nil
	}
}
