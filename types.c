#include <Zend/zend_alloc.h>
#include "types.h"

zval* get_ht_packed_data(HashTable *ht, uint32_t index) {
    if (ht->u.flags & HASH_FLAG_PACKED) {
        return &ht->arPacked[index];
    }
    return NULL;
}

Bucket* get_ht_bucket_data(HashTable *ht, uint32_t index) {
    if (!(ht->u.flags & HASH_FLAG_PACKED)) {
        return &ht->arData[index];
    }
    return NULL;
}
