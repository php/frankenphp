package frankenphp

/*
#cgo noescape __zend_new_array__
#include "types.h"
*/
import "C"
import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"unsafe"
)

type toZval interface {
	toZval(*C.zval)
}

// EXPERIMENTAL: GoString copies a zend_string to a Go string.
func GoString(s unsafe.Pointer) string {
	if s == nil {
		return ""
	}

	return goString((*C.zend_string)(s))
}

var internedStrings = sync.Map{}

func goString(zendStr *C.zend_string) string {

	// interned strings can be global or thread-local, but their number is limited
	if isInternedString(zendStr) {
		if v, ok := internedStrings.Load(zendStr); ok {
			return v.(string)
		}
		str := C.GoStringN((*C.char)(unsafe.Pointer(&zendStr.val)), C.int(zendStr.len))
		internedStrings.Store(zendStr, str)

		return str
	}

	return C.GoStringN((*C.char)(unsafe.Pointer(&zendStr.val)), C.int(zendStr.len))
}

// equivalent of ZSTR_IS_INTERNED
// interned strings are global strings used by the zend_engine (like classnames, function names, etc)
func isInternedString(zs *C.zend_string) bool {
	// mirror of zend_refcounted_h struct
	type zendRefcountedH struct {
		refcount uint32
		typeInfo uint32
	}

	gc := (*zendRefcountedH)(unsafe.Pointer(zs))
	return (gc.typeInfo & C.IS_STR_INTERNED) != 0
}

// EXPERIMENTAL: PHPString converts a Go string to a zend_string with copy. The string can be
// non-persistent (automatically freed after the request by the ZMM) or persistent. If you choose
// the second mode, it is your repsonsability to free the allocated memory.
func PHPString(s string, persistent bool) unsafe.Pointer {
	return unsafe.Pointer(phpString(s, persistent))
}

func phpString(s string, persistent bool) *C.zend_string {
	if s == "" {
		return C.zend_empty_string
	}

	return C.zend_string_init(
		toUnsafeChar(s),
		C.size_t(len(s)),
		C.bool(persistent),
	)
}

// AssociativeArray represents a PHP array with ordered key-value pairs
type AssociativeArray[T any] struct {
	Map   map[string]T
	Order []string
}

func (a AssociativeArray[T]) toZval(zval *C.zval) {
	*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_ARRAY_EX
	*(**C.zend_array)(unsafe.Pointer(&zval.value)) = phpArray[T](a.Map, a.Order)
}

// EXPERIMENTAL: GoAssociativeArray converts a zend_array to a Go AssociativeArray
func GoAssociativeArray[T any](arr unsafe.Pointer) (AssociativeArray[T], error) {
	entries, order, err := goArray[T]((*C.zend_array)(arr), true)

	return AssociativeArray[T]{entries, order}, err
}

// EXPERIMENTAL: GoMap converts a zend_array to an unordered Go map
func GoMap[T any](arr unsafe.Pointer) (map[string]T, error) {
	entries, _, err := goArray[T]((*C.zend_array)(arr), false)

	return entries, err
}

func goArray[T any](array *C.zend_array, ordered bool) (map[string]T, []string, error) {
	if array == nil {
		return nil, nil, fmt.Errorf("received a nil pointer on array conversion")
	}

	nNumUsed := array.nNumUsed
	if nNumUsed == 0 {
		return make(map[string]T), nil, nil
	}

	entries := make(map[string]T, nNumUsed)
	var order []string
	if ordered {
		order = make([]string, 0, nNumUsed)
	}

	if htIsPacked(array) {
		// if the array is packed, convert all integer keys to strings
		// this is probably a bug by the dev using this function
		// still, we'll (inefficiently) convert to an associative array
		zvals := unsafe.Slice(C.get_ht_packed_data(array, 0), nNumUsed)
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := &zvals[i]
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

		return entries, order, nil
	}

	buckets := unsafe.Slice(C.get_ht_bucket(array), nNumUsed)
	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := &buckets[i]
		v, err := goValue[T](&bucket.val)
		if err != nil {
			return nil, nil, err
		}

		if bucket.key != nil {
			keyStr := goString(bucket.key)
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

	return entries, order, nil
}

// EXPERIMENTAL: GoPackedArray converts a zend_array to a Go slice
func GoPackedArray[T any](arr unsafe.Pointer) ([]T, error) {
	return goPackedArray[T]((*C.zend_array)(arr))
}

func goPackedArray[T any](array *C.zend_array) ([]T, error) {
	if array == nil {
		return nil, fmt.Errorf("GoPackedArray received nil pointer")
	}

	nNumUsed := array.nNumUsed
	result := make([]T, 0, nNumUsed)

	if htIsPacked(array) {
		zvals := unsafe.Slice(C.get_ht_packed_data(array, 0), nNumUsed)
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := &zvals[i]
			goVal, err := goValue[T](v)
			if err != nil {
				return nil, err
			}

			result = append(result, goVal)
		}

		return result, nil
	}

	// fallback if ht isn't packed - equivalent to array_values()
	buckets := unsafe.Slice(C.get_ht_bucket(array), nNumUsed)
	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := &buckets[i]
		v, err := goValue[T](&bucket.val)
		if err != nil {
			return nil, err
		}

		result = append(result, v)
	}

	return result, nil
}

// EXPERIMENTAL: PHPMap converts an unordered Go map to a zend_array
func PHPMap[T any](arr map[string]T) unsafe.Pointer {
	return unsafe.Pointer(phpArray[T](arr, nil))
}

// EXPERIMENTAL: PHPAssociativeArray converts a Go AssociativeArray to a zend_array
func PHPAssociativeArray[T any](arr AssociativeArray[T]) unsafe.Pointer {
	return unsafe.Pointer(phpArray[T](arr.Map, arr.Order))
}

func phpArray[T any](entries map[string]T, order []string) *C.zend_array {
	var zendArray *C.zend_array

	if len(order) != 0 {
		zendArray = createNewArray(len(order))
		for _, key := range order {
			val := entries[key]
			var zval C.zval
			phpValue(&zval, val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), &zval)
		}
	} else {
		zendArray = createNewArray(len(entries))
		for key, val := range entries {
			var zval C.zval
			phpValue(&zval, val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), &zval)
		}
	}

	return zendArray
}

// EXPERIMENTAL: PHPPackedArray converts a Go slice to a PHP zend_array.
func PHPPackedArray[T any](slice []T) unsafe.Pointer {
	return unsafe.Pointer(phpPackedArray[T](slice))
}

func phpPackedArray[T any](slice []T) *C.zend_array {
	zendArray := createNewArray(len(slice))
	for _, val := range slice {
		var zval C.zval
		phpValue(&zval, val)
		C.zend_hash_next_index_insert(zendArray, &zval)
	}

	return zendArray
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

	switch zvalGetType(zval) {
	case C.IS_NULL:
		resAny = nil
	case C.IS_FALSE:
		resAny = false
	case C.IS_TRUE:
		resAny = true
	case C.IS_LONG:
		v := (*C.zend_long)(unsafe.Pointer(&zval.value[0]))
		resAny = int64(*v)
	case C.IS_DOUBLE:
		v := (*C.double)(unsafe.Pointer(&zval.value[0]))
		resAny = float64(*v)
	case C.IS_STRING:
		v := *(**C.zend_string)(unsafe.Pointer(&zval.value[0]))
		resAny = goString(v)
	case C.IS_ARRAY:
		array := *(**C.zend_array)(unsafe.Pointer(&zval.value[0]))
		if htIsPacked(array) {
			typ := reflect.TypeOf(res)
			if typ == nil || typ.Kind() == reflect.Interface && typ.NumMethod() == 0 {
				r, e := goPackedArray[any](array)
				if e != nil {
					return resZero, e
				}

				resAny = r

				break
			}

			return resZero, fmt.Errorf("cannot convert packed array to non-any Go type %s", typ.String())
		}

		goMap, order, err := goArray[T](array, true)
		if err != nil {
			return resZero, err
		}

		resAny = AssociativeArray[T]{Map: goMap, Order: order}
	default:
		return resZero, fmt.Errorf("unsupported zval type %d", zvalGetType(zval))
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
	var zval C.zval // TODO: should this be allocated on heap?
	phpValue(&zval, value)
	return unsafe.Pointer(&zval)
}

func phpValue(zval *C.zval, value any) {
	if toZvalObj, ok := value.(toZval); ok {
		toZvalObj.toZval(zval)
		return
	}

	switch v := value.(type) {
	case nil:
		// equvalent of ZVAL_NULL
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_NULL
	case bool:
		// equvalent of ZVAL_BOOL
		if v {
			*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_TRUE
		} else {
			*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_FALSE
		}
	case int:
		// equvalent of ZVAL_LONG
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_LONG
		*(*C.zend_long)(unsafe.Pointer(&zval.value)) = C.zend_long(v)
	case int64:
		// equvalent of ZVAL_LONG
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_LONG
		*(*C.zend_long)(unsafe.Pointer(&zval.value)) = C.zend_long(v)
	case float64:
		// equvalent of ZVAL_DOUBLE
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_DOUBLE
		*(*C.double)(unsafe.Pointer(&zval.value)) = C.double(v)
	case string:
		if v == "" {
			// equivalent ZVAL_EMPTY_STRING
			*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_INTERNED_STRING_EX
			*(**C.zend_string)(unsafe.Pointer(&zval.value)) = C.zend_empty_string
			break
		}
		// equvalent of ZVAL_STRING
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_STRING_EX
		*(**C.zend_string)(unsafe.Pointer(&zval.value)) = phpString(v, false)
	case AssociativeArray[any]:
		// equvalent of ZVAL_ARR
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_ARRAY_EX
		*(**C.zend_array)(unsafe.Pointer(&zval.value)) = phpArray[any](v.Map, v.Order)
	case map[string]any:
		// equvalent of ZVAL_ARR
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_ARRAY_EX
		*(**C.zend_array)(unsafe.Pointer(&zval.value)) = phpArray[any](v, nil)
	case []any:
		// equvalent of ZVAL_ARR
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_ARRAY_EX
		*(**C.zend_array)(unsafe.Pointer(&zval.value)) = phpPackedArray[any](v)
	default:
		panic(fmt.Sprintf("unsupported Go type %T", v))
	}
}

// createNewArray creates a new zend_array with the specified size.
func createNewArray(size int) *C.zend_array {
	return C.__zend_new_array__(C.uint32_t(size))
}

// htIsPacked checks if a zend_array is a list (packed) or hashmap (not packed).
func htIsPacked(ht *C.zend_array) bool {
	flags := *(*C.uint32_t)(unsafe.Pointer(&ht.u[0]))

	return (flags & C.HASH_FLAG_PACKED) != 0
}

// equivalent of Z_TYPE_P
// interpret z->u1 as a 32-bit integer, then take lowest byte
func zvalGetType(z *C.zval) C.uint8_t {
	ptr := (*uint32)(unsafe.Pointer(&z.u1))
	typeInfo := *ptr
	return C.uint8_t(typeInfo & 0xFF)
}

// used in tests for cleanup
func zendStringRelease(p unsafe.Pointer) {
	C.zend_string_release((*C.zend_string)(p))
}

// used in tests for cleanup
func zendHashDestroy(p unsafe.Pointer) {
	C.zend_array_destroy((*C.zend_array)(p))
}
