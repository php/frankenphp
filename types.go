package frankenphp

/*
#include "types.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"unsafe"
)

type toZval interface {
	toZval() *C.zval
}

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
type AssociativeArray[T any] struct {
	Map   map[string]T
	Order []string
}

func (a AssociativeArray[T]) toZval() *C.zval {
	return (*C.zval)(PHPAssociativeArray[T](a))
}

// EXPERIMENTAL: GoAssociativeArray converts a zend_array to a Go AssociativeArray
func GoAssociativeArray[T any](arr unsafe.Pointer) AssociativeArray[T] {
	entries, order, _ := goArray[T](arr, true)

	return AssociativeArray[T]{entries, order}
}

// EXPERIMENTAL: GoMap converts a zval having a zend_array value to an unordered Go map
func GoMap[T any](arr unsafe.Pointer) (map[string]T, error) {
	entries, _, err := goArray[T](arr, false)

	return entries, err
}

func goArray[T any](arr unsafe.Pointer, ordered bool) (map[string]T, []string, error) {
	if arr == nil {
		return nil, nil, errors.New("received a nil pointer on array conversion")
	}

	zval := (*C.zval)(arr)
	v, err := extractZvalValue(zval, C.IS_ARRAY)
	if v == nil || err != nil {
		return nil, nil, fmt.Errorf("received a *zval that wasn't a HashTable on array conversion: %w", err)
	}

	hashTable := (*C.HashTable)(v)

	nNumUsed := hashTable.nNumUsed
	entries := make(map[string]T, nNumUsed)
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
				e, err := goValue[T](v)
				if err != nil {
					return nil, nil, err
				}

				entries[strIndex] = e
				if ordered {
					order = append(order, strIndex)
				}
			}
		}

		return entries, order, nil
	}

	var zeroVal T

	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket == nil || C.zval_get_type(&bucket.val) == C.IS_UNDEF {
			continue
		}

		v, err := goValue[any](&bucket.val)
		if err != nil {
			return nil, nil, err
		}

		if bucket.key != nil {
			keyStr := GoString(unsafe.Pointer(bucket.key))
			if v == nil {
				entries[keyStr] = zeroVal
			} else {
				entries[keyStr] = v.(T)
			}

			if ordered {
				order = append(order, keyStr)
			}

			continue
		}

		// as fallback convert the bucket index to a string key
		strIndex := strconv.Itoa(int(bucket.h))
		entries[strIndex] = v.(T)
		if ordered {
			order = append(order, strIndex)
		}
	}

	return entries, order, nil
}

// EXPERIMENTAL: GoPackedArray converts a zval with a zend_array value to a Go slice
func GoPackedArray[T any](arr unsafe.Pointer) ([]T, error) {
	if arr == nil {
		panic("GoPackedArray received a nil pointer")
	}

	zval := (*C.zval)(arr)
	v, err := extractZvalValue(zval, C.IS_ARRAY)
	if v == nil || err != nil {
		return nil, fmt.Errorf("GoPackedArray received *zval that wasn't a HashTable: %w", err)
	}

	hashTable := (*C.HashTable)(v)

	nNumUsed := hashTable.nNumUsed
	result := make([]T, 0, nNumUsed)

	if htIsPacked(hashTable) {
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(hashTable, i)
			if v != nil && C.zval_get_type(v) != C.IS_UNDEF {
				v, err := goValue[T](v)
				if err != nil {
					return nil, err
				}

				result = append(result, v)
			}
		}

		return result, nil
	}

	// fallback if ht isn't packed - equivalent to array_values()
	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := C.get_ht_bucket_data(hashTable, i)
		if bucket != nil && C.zval_get_type(&bucket.val) != C.IS_UNDEF {
			v, err := goValue[T](&bucket.val)
			if err != nil {
				return nil, err
			}

			result = append(result, v)
		}
	}

	return result, nil
}

// EXPERIMENTAL: PHPMap converts an unordered Go map to a PHP zend_array
func PHPMap[T any](arr map[string]T) unsafe.Pointer {
	return phpArray[T](arr, nil)
}

// EXPERIMENTAL: PHPAssociativeArray converts a Go AssociativeArray to a PHP zval with a zend_array value
func PHPAssociativeArray[T any](arr AssociativeArray[T]) unsafe.Pointer {
	return phpArray[T](arr.Map, arr.Order)
}

func phpArray[T any](entries map[string]T, order []string) unsafe.Pointer {
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
func PHPPackedArray[T any](slice []T) unsafe.Pointer {
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
func GoValue[T any](zval unsafe.Pointer) (T, error) {
	return goValue[T]((*C.zval)(zval))
}

func goValue[T any](zval *C.zval) (res T, err error) {
	var (
		resAny  any
		resZero T
	)
	t := C.zval_get_type(zval)

	switch t {
	case C.IS_NULL:
		resAny = any(nil)
	case C.IS_FALSE:
		resAny = any(false)
	case C.IS_TRUE:
		resAny = any(true)
	case C.IS_LONG:
		v, err := extractZvalValue(zval, C.IS_LONG)
		if err != nil {
			return resZero, err
		}

		if v != nil {
			resAny = any(int64(*(*C.zend_long)(v)))

			break
		}

		resAny = any(int64(0))
	case C.IS_DOUBLE:
		v, err := extractZvalValue(zval, C.IS_DOUBLE)
		if err != nil {
			return resZero, err
		}

		if v != nil {
			resAny = any(float64(*(*C.double)(v)))

			break
		}

		resAny = any(float64(0))
	case C.IS_STRING:
		v, err := extractZvalValue(zval, C.IS_STRING)
		if err != nil {
			return resZero, err
		}

		if v == nil {
			resAny = any("")

			break
		}

		resAny = any(GoString(v))
	case C.IS_ARRAY:
		v, err := extractZvalValue(zval, C.IS_ARRAY)
		if err != nil {
			return resZero, err
		}

		hashTable := (*C.HashTable)(v)
		if hashTable != nil && htIsPacked(hashTable) {
			typ := reflect.TypeOf(res)
			if typ == nil || typ.Kind() == reflect.Interface && typ.NumMethod() == 0 {
				r, e := GoPackedArray[any](unsafe.Pointer(zval))
				if e != nil {
					return resZero, e
				}

				resAny = any(r)

				break
			}

			return resZero, fmt.Errorf("cannot convert packed array to non-any Go type %s", typ.String())
		}

		resAny = any(GoAssociativeArray[T](unsafe.Pointer(zval)))
	default:
		return resZero, fmt.Errorf("unsupported zval type %d", t)
	}

	if resAny == nil {
		return resZero, nil
	}

	if castRes, ok := resAny.(T); ok {
		return castRes, nil
	}

	return resZero, fmt.Errorf("cannot cast value of type %T to type %T", resAny, res)
}

// EXPERIMENTAL: PHPValue converts a Go any to a PHP zval
func PHPValue(value any) unsafe.Pointer {
	return unsafe.Pointer(phpValue(value))
}

func phpValue(value any) *C.zval {
	var zval C.zval

	if toZvalObj, ok := value.(toZval); ok {
		return toZvalObj.toZval()
	}

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
	case map[string]any:
		return (*C.zval)(PHPAssociativeArray[any](AssociativeArray[any]{Map: v}))
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
func extractZvalValue(zval *C.zval, expectedType C.uint8_t) (unsafe.Pointer, error) {
	if zval == nil {
		return nil, nil
	}

	if zType := C.zval_get_type(zval); zType != expectedType {
		return nil, fmt.Errorf("zval type mismatch: expected %d, got %d", expectedType, zType)
	}

	v := unsafe.Pointer(&zval.value[0])

	switch expectedType {
	case C.IS_LONG:
		return v, nil
	case C.IS_DOUBLE:
		return v, nil
	case C.IS_STRING:
		return unsafe.Pointer(*(**C.zend_string)(v)), nil
	case C.IS_ARRAY:
		return unsafe.Pointer(*(**C.zend_array)(v)), nil
	}

	return nil, fmt.Errorf("unsupported zval type %d", expectedType)
}
