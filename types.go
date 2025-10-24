package frankenphp

/*
#cgo nocallback __zend_new_array__
#cgo nocallback __zval_null__
#cgo nocallback __zval_bool__
#cgo nocallback __zval_long__
#cgo nocallback __zval_double__
#cgo nocallback __zval_string__
#cgo nocallback __zval_arr__
#cgo noescape __zend_new_array__
#cgo noescape __zval_null__
#cgo noescape __zval_bool__
#cgo noescape __zval_long__
#cgo noescape __zval_double__
#cgo noescape __zval_string__
#cgo noescape __zval_arr__
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
func GoAssociativeArray[T any](arr unsafe.Pointer) (AssociativeArray[T], error) {
	entries, order, err := goArray[T](arr, true)

	return AssociativeArray[T]{entries, order}, err
}

// EXPERIMENTAL: GoMap converts a zend_array to an unordered Go map
func GoMap[T any](arr unsafe.Pointer) (map[string]T, error) {
	entries, _, err := goArray[T](arr, false)

	return entries, err
}

func goArray[T any](arr unsafe.Pointer, ordered bool) (map[string]T, []string, error) {
	if arr == nil {
		return nil, nil, errors.New("received a nil pointer on array conversion")
	}

	array := (*C.zend_array)(arr)

	if array == nil {
		return nil, nil, fmt.Errorf("received a *zval that wasn't a HashTable on array conversion")
	}

	nNumUsed := array.nNumUsed
	entries := make(map[string]T, nNumUsed)
	var order []string
	if ordered {
		order = make([]string, 0, nNumUsed)
	}

	if htIsPacked(array) {
		// if the array is packed, convert all integer keys to strings
		// this is probably a bug by the dev using this function
		// still, we'll (inefficiently) convert to an associative array
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(array, i)
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
		bucket := C.get_ht_bucket_data(array, i)
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

// EXPERIMENTAL: GoPackedArray converts a zend_array to a Go slice
func GoPackedArray[T any](arr unsafe.Pointer) ([]T, error) {
	if arr == nil {
		return nil, errors.New("GoPackedArray received a nil value")
	}

	array := (*C.zend_array)(arr)

	if array == nil {
		return nil, fmt.Errorf("GoPackedArray received *zval that wasn't a HashTable")
	}

	nNumUsed := array.nNumUsed
	result := make([]T, 0, nNumUsed)

	if htIsPacked(array) {
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := C.get_ht_packed_data(array, i)
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
		bucket := C.get_ht_bucket_data(array, i)
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

// EXPERIMENTAL: PHPMap converts an unordered Go map to a zend_array
func PHPMap[T any](arr map[string]T) unsafe.Pointer {
	return phpArray[T](arr, nil)
}

// EXPERIMENTAL: PHPAssociativeArray converts a Go AssociativeArray to a zend_array
func PHPAssociativeArray[T any](arr AssociativeArray[T]) unsafe.Pointer {
	return phpArray[T](arr.Map, arr.Order)
}

func phpArray[T any](entries map[string]T, order []string) unsafe.Pointer {
	var zendArray *C.zend_array

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
			fmt.Println("adding key", key, "val", val)
			zval := phpValue(val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), zval)
		}
	}

	return unsafe.Pointer(zendArray)
}

// EXPERIMENTAL: PHPPackedArray converts a Go slice to a PHP zval with a zend_array value.
func PHPPackedArray[T any](slice []T) unsafe.Pointer {
	zendArray := createNewArray((uint32)(len(slice)))
	for _, val := range slice {
		zval := phpValue(val)
		C.zend_hash_next_index_insert(zendArray, zval)
	}

	return unsafe.Pointer(zendArray)
}

// EXPERIMENTAL: GoValue converts a PHP zval to a Go value
//
// Zval having the null, bool, long, double, string and array types are currently supported.
// Arrays can curently only be converted to any[] and AssociativeArray[any].
// Any other type will cause an error.
// More types may be supported in the future.
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

		array := (*C.zend_array)(v)
		if array != nil && htIsPacked(array) {
			typ := reflect.TypeOf(res)
			if typ == nil || typ.Kind() == reflect.Interface && typ.NumMethod() == 0 {
				r, e := GoPackedArray[any](unsafe.Pointer(array))
				if e != nil {
					return resZero, e
				}

				resAny = any(r)

				break
			}

			return resZero, fmt.Errorf("cannot convert packed array to non-any Go type %s", typ.String())
		}

		a, err := GoAssociativeArray[T](unsafe.Pointer(array))
		if err != nil {
			return resZero, err
		}

		resAny = any(a)
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
//
// nil, bool, int, int64, float64, string, []any, and map[string]any are currently supported.
// Any other type will cause a panic.
// More types may be supported in the future.
func PHPValue(value any) unsafe.Pointer {
	return unsafe.Pointer(phpValue(value))
}

func phpValue(value any) *C.zval {
	var zval C.zval

	if toZvalObj, ok := value.(toZval); ok {
		fmt.Println("wtf")
		return toZvalObj.toZval()
	}

	fmt.Println("type", reflect.TypeOf(value))

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
		if v == "" {
			C.__zval_empty_string__(&zval)
			break
		}
		str := (*C.zend_string)(PHPString(v, false))
		C.__zval_string__(&zval, str)
	case AssociativeArray[any]:
		fmt.Println("associative arr")
		C.__zval_arr__(&zval, (*C.zend_array)(PHPAssociativeArray[any](v)))
	case map[string]any:
		fmt.Println("map arr")
		C.__zval_arr__(&zval, (*C.zend_array)(PHPMap[any](v)))
	case []any:
		C.__zval_arr__(&zval, (*C.zend_array)(PHPPackedArray[any](v)))
	default:
		panic(fmt.Sprintf("unsupported Go type %T", v))
	}

	return &zval
}

// createNewArray creates a new zend_array with the specified size.
func createNewArray(size uint32) *C.zend_array {
	arr := C.__zend_new_array__(C.uint32_t(size))
	return (*C.zend_array)(unsafe.Pointer(arr))
}

// htIsPacked checks if a zend_array is a list (packed) or hashmap (not packed).
func htIsPacked(ht *C.zend_array) bool {
	flags := *(*C.uint32_t)(unsafe.Pointer(&ht.u[0]))

	return (flags & C.HASH_FLAG_PACKED) != 0
}

// extractZvalValue returns a pointer to the zval value cast to the expected type
func extractZvalValue(zval *C.zval, expectedType C.uint8_t) (unsafe.Pointer, error) {
	if zval == nil {
		if expectedType == C.IS_NULL {
			return nil, nil
		}

		return nil, fmt.Errorf("zval type mismatch: expected %d, got nil", expectedType)
	}

	if zType := C.zval_get_type(zval); zType != expectedType {
		return nil, fmt.Errorf("zval type mismatch: expected %d, got %d", expectedType, zType)
	}

	v := unsafe.Pointer(&zval.value[0])

	switch expectedType {
	case C.IS_LONG, C.IS_DOUBLE:
		return v, nil
	case C.IS_STRING:
		return unsafe.Pointer(*(**C.zend_string)(v)), nil
	case C.IS_ARRAY:
		return unsafe.Pointer(*(**C.zend_array)(v)), nil
	}

	return nil, fmt.Errorf("unsupported zval type %d", expectedType)
}

func zendStringRelease(p unsafe.Pointer) {
	zs := (*C.zend_string)(p)
	C.zend_string_release(zs)
}

func zendHashDestroy(p unsafe.Pointer) {
	ht := (*C.zend_array)(p)
	C.zend_hash_destroy(ht)
}
