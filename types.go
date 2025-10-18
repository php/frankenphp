package frankenphp

/*
#cgo nocallback __zend_new_array__
#cgo noescape __zend_new_array__
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

	return goString((*C.zend_string)(s))
}

func goString(zendStr *C.zend_string) string {
	return C.GoStringN((*C.char)(unsafe.Pointer(&zendStr.val)), C.int(zendStr.len))
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
type AssociativeArray struct {
	Map   map[string]any
	Order []string
}

type Object struct {
	ClassName  string
	Props      map[string]any
	ce         *C.zend_class_entry
	serialized *C.zend_string
}

// EXPERIMENTAL: GoAssociativeArray converts a zend_array to a Go AssociativeArray
func GoAssociativeArray(arr unsafe.Pointer) AssociativeArray {
	entries, order := goArray((*C.zend_array)(arr), true)
	return AssociativeArray{entries, order}
}

// EXPERIMENTAL: GoMap converts a PHP zend_array to an unordered Go map
func GoMap(arr unsafe.Pointer) map[string]any {
	entries, _ := goArray((*C.zend_array)(arr), false)
	return entries
}

func goArray(array *C.zend_array, ordered bool) (map[string]any, []string) {
	if array == nil {
		panic("received a pointer that wasn't a zend_array on array conversion")
	}

	nNumUsed := array.nNumUsed
	entries := make(map[string]any, nNumUsed)
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
			entries[strIndex] = goValue(v)
			if ordered {
				order = append(order, strIndex)
			}
		}

		return entries, order
	}

	buckets := unsafe.Slice(C.get_ht_bucket(array), nNumUsed)
	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := &buckets[i]
		v := goValue(&bucket.val)

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

	return entries, order
}

// EXPERIMENTAL: GoPackedArray converts a PHP zend_array to a Go slice
func GoPackedArray(arr unsafe.Pointer) []any {
	if arr == nil {
		panic("GoPackedArray received a nil pointer")
	}

	array := (*C.zend_array)(arr)

	if array == nil {
		panic("GoPackedArray received a pointer that wasn't a zrnd_array")
	}

	nNumUsed := array.nNumUsed
	result := make([]any, 0, nNumUsed)

	if htIsPacked(array) {
		zvals := unsafe.Slice(C.get_ht_packed_data(array, 0), nNumUsed)
		for i := C.uint32_t(0); i < nNumUsed; i++ {
			v := &zvals[i]
			result = append(result, goValue(v))
		}

		return result
	}

	// fallback if ht isn't packed - equivalent to array_values()
	buckets := unsafe.Slice(C.get_ht_bucket(array), nNumUsed)
	for i := C.uint32_t(0); i < nNumUsed; i++ {
		bucket := &buckets[i]
		result = append(result, goValue(&bucket.val))
	}

	return result
}

// EXPERIMENTAL: PHPMap converts an unordered Go map to a PHP zend_array
func PHPMap(arr map[string]any) unsafe.Pointer {
	return unsafe.Pointer(phpArray(arr, nil))
}

// EXPERIMENTAL: PHPAssociativeArray converts a Go AssociativeArray to a PHP zend_array
func PHPAssociativeArray(arr AssociativeArray) unsafe.Pointer {
	return unsafe.Pointer(phpArray(arr.Map, arr.Order))
}

func phpArray(entries map[string]any, order []string) *C.zend_array {
	var zendArray *C.zend_array

	if len(order) != 0 {
		zendArray = createNewArray((uint32)(len(order)))
		for _, key := range order {
			val := entries[key]
			var zval C.zval
			phpValue(&zval, val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), &zval)
		}
	} else {
		zendArray = createNewArray((uint32)(len(entries)))
		for key, val := range entries {
			var zval C.zval
			phpValue(&zval, val)
			C.zend_hash_str_update(zendArray, toUnsafeChar(key), C.size_t(len(key)), &zval)
		}
	}

	return zendArray
}

// EXPERIMENTAL: PHPPackedArray converts a Go slice to a PHP zend_array.
func PHPPackedArray(slice []any) unsafe.Pointer {
	zendArray := createNewArray((uint32)(len(slice)))
	for _, val := range slice {
		var zval C.zval
		phpValue(&zval, val)
		C.zend_hash_next_index_insert(zendArray, &zval)
	}

	return unsafe.Pointer(zendArray)
}

// EXPERIMENTAL: GoValue converts a PHP zval to a Go value
func GoValue(zval unsafe.Pointer) any {
	return goValue((*C.zval)(zval))
}

func goValue(zval *C.zval) any {
	t := zvalGetType(zval) // TODO: zval->u1.v.type

	switch t {
	case C.IS_NULL:
		return nil
	case C.IS_FALSE:
		return false
	case C.IS_TRUE:
		return true
	case C.IS_OBJECT:
		obj := (*C.zend_object)(extractZvalValue(zval, C.IS_OBJECT))
		if obj != nil {
			return goObject(obj)
		}

		return nil
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

		return goString(str)
	case C.IS_ARRAY:
		array := (*C.zend_array)(extractZvalValue(zval, C.IS_ARRAY))
		if array != nil && htIsPacked(array) {
			return GoPackedArray(unsafe.Pointer(array))
		}

		return GoAssociativeArray(unsafe.Pointer(array))
	default:
		return nil
	}
}

// EXPERIMENTAL: PHPValue converts a Go any to a PHP zval
func PHPValue(value any) unsafe.Pointer {
	var zval C.zval // TODO: emalloc?
	phpValue(&zval, value)
	return unsafe.Pointer(&zval)
}

func phpValue(zval *C.zval, value any) {
	switch v := value.(type) {
	case nil:
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_NULL
	case bool:
		if v {
			*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_TRUE
		} else {
			*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_FALSE
		}
	case int:
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_LONG
		*(*C.zend_long)(unsafe.Pointer(&zval.value)) = C.zend_long(v)
	case int64:
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_LONG
		*(*C.zend_long)(unsafe.Pointer(&zval.value)) = C.zend_long(v)
	case float64:
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_DOUBLE
		*(*C.double)(unsafe.Pointer(&zval.value)) = C.double(v)
	case string:
		if v == "" {
			*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_INTERNED_STRING_EX
			*(**C.zend_string)(unsafe.Pointer(&zval.value)) = C.zend_empty_string
			break
		}
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_STRING_EX
		*(**C.zend_string)(unsafe.Pointer(&zval.value)) = phpString(v, false)
	case AssociativeArray:
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_ARRAY_EX
		*(**C.zend_array)(unsafe.Pointer(&zval.value)) = phpArray(v.Map, v.Order)
	case map[string]any:
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_ARRAY_EX
		*(**C.zend_array)(unsafe.Pointer(&zval.value)) = phpArray(v, nil)
	case []any:
		*(*uint32)(unsafe.Pointer(&zval.u1)) = C.IS_ARRAY_EX
		*(**C.zend_array)(unsafe.Pointer(&zval.value)) = (*C.zend_array)(PHPPackedArray(v))
	case Object:
		phpObject(zval, v)
	default:
		panic(fmt.Sprintf("unsupported Go type %T", v))
	}
}

func GoObject(obj unsafe.Pointer) Object {
	zval := (*C.zval)(obj)
	zObj := (*C.zend_object)(extractZvalValue(zval, C.IS_OBJECT))

	return goObject(zObj)
}

func goObject(obj *C.zend_object) Object {
	if obj == nil {
		panic("received a nil pointer on object conversion")
	}
	classEntry := obj.ce

	if C.is_internal_class(classEntry) {
		return Object{
			ClassName:  goString(classEntry.name),
			serialized: C.__zval_serialize__(obj),
			ce:         classEntry,
		}
	}

	var props map[string]any
	// iterate over the properties
	if obj.properties != nil {
		hashTable := (*C.HashTable)(unsafe.Pointer(obj.properties))
		props, _ = goArray(hashTable, false)
	}

	return Object{
		ClassName: goString(classEntry.name),
		Props:     props,
		ce:        classEntry,
	}
}

func PHPObject(obj Object) unsafe.Pointer {
	var zval C.zval
	phpObject(&zval, obj)

	return unsafe.Pointer(&zval)
}

func phpObject(zval *C.zval, obj Object) {
	if obj.serialized != nil {
		C.__zval_unserialize__(zval, obj.serialized)
		return
	}

	zendObj := C.__php_object_init__(zval, toUnsafeChar(obj.ClassName), C.size_t(len(obj.ClassName)), obj.ce)
	if zendObj == nil {
		panic("class not found: " + obj.ClassName)
	}

	zendObj.properties = phpArray(obj.Props, nil)
	// TODO: wakeup?
}

// createNewArray creates a new zend_array with the specified size.
func createNewArray(size uint32) *C.zend_array {
	return C.__zend_new_array__(C.uint32_t(size))
}

// htIsPacked checks if a zend_array is a list (packed) or hashmap (not packed).
func htIsPacked(ht *C.zend_array) bool {
	flags := *(*C.uint32_t)(unsafe.Pointer(&ht.u[0]))

	return (flags & C.HASH_FLAG_PACKED) != 0
}

// extractZvalValue returns a pointer to the zval value cast to the expected type
func extractZvalValue(zval *C.zval, expectedType C.uint8_t) unsafe.Pointer {
	if zval == nil || zvalGetType(zval) != expectedType {
		return nil
	}

	v := unsafe.Pointer(&zval.value[0])

	switch expectedType {
	case C.IS_LONG:
		return v
	case C.IS_DOUBLE:
		return v
	case C.IS_OBJECT:
		return unsafe.Pointer(*(**C.zend_object)(v))
	case C.IS_STRING:
		return unsafe.Pointer(*(**C.zend_string)(v))
	case C.IS_ARRAY:
		return unsafe.Pointer(*(**C.zend_array)(v))
	default:
		return nil
	}
}

func zvalPtrDtor(p unsafe.Pointer) {
	zv := (*C.zval)(p)
	C.zval_ptr_dtor(zv)
}

func zendStringRelease(p unsafe.Pointer) {
	zs := (*C.zend_string)(p)
	C.zend_string_release(zs)
}

func zendHashDestroy(p unsafe.Pointer) {
	ht := (*C.zend_array)(p)
	C.zend_array_destroy(ht)
}

func zvalGetType(z *C.zval) C.uint8_t {
	// interpret z->u1 as a 32-bit integer, then take lowest byte
	ptr := (*uint32)(unsafe.Pointer(&z.u1))
	typeInfo := *ptr
	return C.uint8_t(typeInfo & 0xFF)
}
