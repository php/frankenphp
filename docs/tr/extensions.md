# Go ile PHP Eklentileri Yazma

FrankenPHP ile **Go dilinde PHP eklentileri yazabilir**, böylece doğrudan PHP'den çağrılabilen **yüksek performanslı yerel fonksiyonlar** oluşturabilirsiniz. Uygulamalarınız mevcut veya yeni herhangi bir Go kütüphanesini ve ayrıca PHP kodunuzdan doğrudan **gorutinlerin** meşhur eşzamanlılık modelini kullanabilir.

PHP eklentileri genellikle C ile yazılır, ancak biraz ek çabayla başka dillerde de yazmak mümkündür. PHP eklentileri, PHP'nin işlevselliğini genişletmek için düşük seviyeli dillerin gücünden yararlanmanıza olanak tanır, örneğin yerel fonksiyonlar ekleyerek veya belirli işlemleri optimize ederek.

Caddy modülleri sayesinde, Go dilinde PHP eklentileri yazabilir ve bunları FrankenPHP'ye çok hızlı bir şekilde entegre edebilirsiniz.

## İki Yaklaşım

FrankenPHP, Go dilinde PHP eklentileri oluşturmanın iki yolunu sunar:

1. **Eklenti Üreticisini Kullanma** - Çoğu kullanım durumu için gerekli tüm temel yapıyı üreten, Go kodunuzu yazmaya odaklanmanızı sağlayan önerilen yaklaşım
2. **Manuel Uygulama** - Gelişmiş kullanım durumları için eklenti yapısı üzerinde tam kontrol

Başlamak için en kolay yol olduğu için üretici yaklaşımıyla başlayacak, ardından tam kontrole ihtiyaç duyanlar için manuel uygulamayı göstereceğiz.

## Eklenti Üreticisini Kullanma

FrankenPHP, yalnızca Go kullanarak **bir PHP eklentisi oluşturmanıza** olanak tanıyan bir araçla birlikte gelir. **C kodu yazmaya** veya doğrudan CGO kullanmaya gerek yok: FrankenPHP ayrıca **PHP/C ve Go arasındaki tür dengelemesi (type juggling)** konusunda endişelenmenize gerek kalmadan uzantılarınızı Go'da yazmanıza yardımcı olacak bir **genel türler API'si** içerir.

> [!TIP]
> Eklentilerin Go dilinde baştan sona nasıl yazılabileceğini anlamak istiyorsanız, aşağıda üretici kullanmadan Go dilinde bir PHP eklentisi yazmayı gösteren manuel uygulama bölümünü okuyabilirsiniz.

Bu aracın **tam teşekküllü bir eklenti üreticisi olmadığını** unutmayın. Amacı, Go'da basit eklentiler yazmanıza yardımcı olmaktır, ancak PHP eklentilerinin en gelişmiş özelliklerini sağlamaz. Daha **karmaşık ve optimize edilmiş** bir eklenti yazmanız gerekiyorsa, doğrudan C kodu yazmanız veya CGO kullanmanız gerekebilir.

### Önkoşullar

Aşağıdaki manuel uygulama bölümünde de belirtildiği gibi, [PHP kaynaklarını edinmeniz](https://www.php.net/downloads.php) ve yeni bir Go modülü oluşturmanız gerekir.

#### Yeni Bir Modül Oluşturun ve PHP Kaynaklarını Edinin

Go'da bir PHP eklentisi yazmanın ilk adımı yeni bir Go modülü oluşturmaktır. Bunun için şu komutu kullanabilirsiniz:

```console
go mod init example.com/example
```

İkinci adım, sonraki adımlar için [PHP kaynaklarını edinmektir](https://www.php.net/downloads.php). Kaynakları edindikten sonra, Go modülünüzün içine değil, istediğiniz bir dizine açın:

```console
tar xf php-*
```

### Eklentiyi Yazma

Yerel fonksiyonunuzu Go'da yazmak için her şey hazır. `stringext.go` adında yeni bir dosya oluşturun. İlk fonksiyonumuz argüman olarak bir dize, onu tekrarlama sayısı, dizenin ters çevrilip çevrilmeyeceğini belirten bir boole değeri alacak ve sonuç dizesini döndürecektir. Şöyle görünmelidir:

```go
package example

// #include <Zend/zend_types.h>
import "C"
import (
    "strings"
	"unsafe"

	"github.com/dunglas/frankenphp"
)

//export_php:function repeat_this(string $str, int $count, bool $reverse): string
func repeat_this(s *C.zend_string, count int64, reverse bool) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(s))

    result := strings.Repeat(str, int(count))
    if reverse {
        runes := []rune(result)
        for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
            runes[i], runes[j] = runes[j], runes[i]
        }
        result = string(runes)
    }

    return frankenphp.PHPString(result, false)
}
```

Burada dikkat edilmesi gereken iki önemli nokta var:

- `//export_php:function` yönerge yorumu, PHP'deki fonksiyon imzasını tanımlar. Üretici, PHP fonksiyonunu doğru parametreler ve dönüş tipiyle nasıl üreteceğini bu şekilde bilir;
- Fonksiyon bir `unsafe.Pointer` döndürmelidir. FrankenPHP, C ve Go arasındaki tür dengelemesi konusunda size yardımcı olacak bir API sağlar.

İlk nokta kendi kendini açıklasa da, ikincisi kavranması daha zor olabilir. Bir sonraki bölümde tür dengelemesine daha derinlemesine bakalım.

### Tür Dengeleme (Type Juggling)

Bazı değişken türlerinin C/PHP ve Go arasında aynı bellek temsiline sahip olmasına rağmen, bazı türlerin doğrudan kullanılabilmesi için daha fazla mantık gereklidir. Bu, eklenti yazarken belki de en zor kısımdır çünkü Zend Motoru'nun iç işleyişini ve değişkenlerin PHP'de dahili olarak nasıl saklandığını anlamayı gerektirir.
Bu tablo bilmeniz gerekenleri özetlemektedir:

| PHP type           | Go type                       | Direct conversion | C to Go helper                    | Go to C helper                     | Class Methods Support |
| :----------------- | :---------------------------- | :---------------- | :-------------------------------- | :--------------------------------- | :-------------------- |
| `int`              | `int64`                       | ✅                | -                                 | -                                  | ✅                    |
| `?int`             | `*int64`                      | ✅                | -                                 | -                                  | ✅                    |
| `float`            | `float64`                     | ✅                | -                                 | -                                  | ✅                    |
| `?float`           | `*float64`                    | ✅                | -                                 | -                                  | ✅                    |
| `bool`             | `bool`                        | ✅                | -                                 | -                                  | ✅                    |
| `?bool`            | `*bool`                       | ✅                | -                                 | -                                  | ✅                    |
| `string`/`?string` | `*C.zend_string`              | ❌                | `frankenphp.GoString()`           | `frankenphp.PHPString()`           | ✅                    |
| `array`            | `frankenphp.AssociativeArray` | ❌                | `frankenphp.GoAssociativeArray()` | `frankenphp.PHPAssociativeArray()` | ✅                    |
| `array`            | `map[string]any`              | ❌                | `frankenphp.GoMap()`              | `frankenphp.PHPMap()`              | ✅                    |
| `array`            | `[]any`                       | ❌                | `frankenphp.GoPackedArray()`      | `frankenphp.PHPPackedArray()`      | ✅                    |
| `mixed`            | `any`                         | ❌                | `GoValue()`                       | `PHPValue()`                       | ❌                    |
| `callable`         | `*C.zval`                     | ❌                | -                                 | `frankenphp.CallPHPCallable()`     | ❌                    |
| `object`           | `struct`                      | ❌                | _Henüz uygulanmadı_               | _Henüz uygulanmadı_                | ❌                    |

> [!NOTE]
>
> Bu tablo henüz kapsamlı değildir ve FrankenPHP türler API'si daha da tamamlandıkça güncellenecektir.
>
> Sınıf metotları için özel olarak, şu anda ilkel türler ve diziler desteklenmektedir. Nesneler henüz metot parametresi veya dönüş türü olarak kullanılamaz.

Önceki bölümdeki kod parçacığına bakarsanız, ilk parametreyi ve dönüş değerini dönüştürmek için yardımcıların kullanıldığını görebilirsiniz. `repeat_this()` fonksiyonumuzun ikinci ve üçüncü parametrelerinin dönüştürülmesine gerek yoktur, çünkü temel türlerin bellek gösterimi hem C hem de Go için aynıdır.

#### Dizilerle Çalışma

FrankenPHP, `frankenphp.AssociativeArray` aracılığıyla veya doğrudan bir haritaya (map) veya dilime (slice) dönüştürerek PHP dizileri için yerel destek sağlar.

`AssociativeArray`, bir `Map: map[string]any` alanı ve isteğe bağlı bir `Order: []string` alanı içeren bir [hash haritasını](https://en.wikipedia.org/wiki/Hash_table) temsil eder (PHP'nin "ilişkisel dizilerinin" aksine, Go haritaları sıralı değildir).

Sıra veya ilişkilendirme gerekmiyorsa, doğrudan bir dilime `[]any` veya sırasız bir haritaya `map[string]any` dönüştürmek de mümkündür.

**Go'da dizileri oluşturma ve işleme:**

```go
package example

// #include <Zend/zend_types.h>
import "C"
import (
    "unsafe"

    "github.com/dunglas/frankenphp"
)

// export_php:function process_data_ordered(array $input): array
func process_data_ordered_map(arr *C.zend_array) unsafe.Pointer {
	// Convert PHP associative array to Go while keeping the order
	associativeArray, err := frankenphp.GoAssociativeArray[any](unsafe.Pointer(arr))
    if err != nil {
        // handle error
    }

	// loop over the entries in order
	for _, key := range associativeArray.Order {
		value, _ = associativeArray.Map[key]
		// do something with key and value
	}

	// return an ordered array
	// if 'Order' is not empty, only the key-value pairs in 'Order' will be respected
	return frankenphp.PHPAssociativeArray[string](frankenphp.AssociativeArray[string]{
		Map: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		Order: []string{"key1", "key2"},
	})
}

// export_php:function process_data_unordered(array $input): array
func process_data_unordered_map(arr *C.zend_array) unsafe.Pointer {
	// Convert PHP associative array to a Go map without keeping the order
	// ignoring the order will be more performant
	goMap, err := frankenphp.GoMap[any](unsafe.Pointer(arr))
    if err != nil {
        // handle error
    }

	// loop over the entries in no specific order
	for key, value := range goMap {
		// do something with key and value
	}

	// return an unordered array
	return frankenphp.PHPMap(map[string]string {
		"key1": "value1",
		"key2": "value2",
	})
}

// export_php:function process_data_packed(array $input): array
func process_data_packed(arr *C.zend_array) unsafe.Pointer {
	// Convert PHP packed array to Go
	goSlice, err := frankenphp.GoPackedArray(unsafe.Pointer(arr))
    if err != nil {
        // handle error
    }

	// loop over the slice in order
	for index, value := range goSlice {
		// do something with index and value
	}

	// return a packed array
	return frankenphp.PHPPackedArray([]string{"value1", "value2", "value3"})
}
```

**Dizi dönüşümünün temel özellikleri:**

- **Sıralı anahtar-değer çiftleri** - İlişkisel dizinin sırasını koruma seçeneği
- **Birden fazla durum için optimize edildi** - Daha iyi performans için sırayı bırakma veya doğrudan bir dilime dönüştürme seçeneği
- **Otomatik liste tespiti** - PHP'ye dönüştürürken, dizinin sıkıştırılmış bir liste mi yoksa hash haritası mı olması gerektiğini otomatik olarak algılar
- **İç İçe Diziler** - Diziler iç içe olabilir ve tüm desteklenen türleri otomatik olarak dönüştürür (`int64`,`float64`,`string`,`bool`,`nil`,`AssociativeArray`,`map[string]any`,`[]any`)
- **Nesneler desteklenmiyor** - Şu anda yalnızca skaler türler ve diziler değer olarak kullanılabilir. Bir nesne sağlamak, PHP dizisinde `null` bir değerle sonuçlanacaktır.

##### Mevcut metodlar: Packed ve İlişkisel

- `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer` - Anahtar-değer çiftleriyle sıralı bir PHP dizisine dönüştürür
- `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` - Bir haritayı anahtar-değer çiftleriyle sırasız bir PHP dizisine dönüştürür
- `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` - Bir dilimi yalnızca indekslenmiş değerlere sahip bir PHP packed dizisine dönüştürür
- `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray` - Bir PHP dizisini sıralı bir Go `AssociativeArray`'e (sıralı harita) dönüştürür
- `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` - Bir PHP dizisini sırasız bir Go haritasına dönüştürür
- `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` - Bir PHP dizisini bir Go dilimine dönüştürür
- `frankenphp.IsPacked(zval *C.zend_array) bool` - Bir PHP dizisinin packed (yalnızca indekslenmiş) mi yoksa ilişkisel (anahtar-değer çiftleri) mi olduğunu kontrol eder

### Çağrılabilirler (Callables) ile Çalışma

FrankenPHP, `frankenphp.CallPHPCallable` yardımcısı kullanarak PHP çağrılabilirleriyle çalışma olanağı sağlar. Bu, Go kodundan PHP fonksiyonlarını veya metotlarını çağırmanıza olanak tanır.

Bunu göstermek için, çağrılabilir ve bir dizi alan, çağrılabilir'i dizinin her elemanına uygulayan ve sonuçlarla yeni bir dizi döndüren kendi `array_map()` fonksiyonumuzu oluşturalım:

```go
// export_php:function my_array_map(array $data, callable $callback): array
func my_array_map(arr *C.zend_array, callback *C.zval) unsafe.Pointer {
	goSlice, err := frankenphp.GoPackedArray[any](unsafe.Pointer(arr))
	if err != nil {
		panic(err)
	}

	result := make([]any, len(goSlice))

	for index, value := range goSlice {
		result[index] = frankenphp.CallPHPCallable(unsafe.Pointer(callback), []interface{}{value})
	}

	return frankenphp.PHPPackedArray(result)
}
```

PHP'den parametre olarak geçirilen çağrılabilir'i çağırmak için `frankenphp.CallPHPCallable()`'ı nasıl kullandığımıza dikkat edin. Bu fonksiyon, çağrılabilir'e bir işaretçi ve bir argüman dizisi alır ve çağrılabilir'in yürütme sonucunu döndürür. Alıştığınız çağrılabilir sözdizimini kullanabilirsiniz:

```php
<?php

$result = my_array_map([1, 2, 3], function($x) { return $x * 2; });
// $result will be [2, 4, 6]

$result = my_array_map(['hello', 'world'], 'strtoupper');
// $result will be ['HELLO', 'WORLD']
```

### Yerel Bir PHP Sınıfı Bildirme

Üretici, Go struct'ları olarak **saydam olmayan sınıfları** bildirmeyi destekler; bunlar PHP nesneleri oluşturmak için kullanılabilir. Bir PHP sınıfı tanımlamak için `//export_php:class` yönerge yorumunu kullanabilirsiniz. Örneğin:

```go
package example

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### Saydam Olmayan Sınıflar Nelerdir?

**Saydam olmayan sınıflar**, dahili yapının (özelliklerin) PHP kodundan gizlendiği sınıflardır. Bu şu anlama gelir:

- **Doğrudan özellik erişimi yok**: PHP'den özellikleri doğrudan okuyamaz veya yazamazsınız (`$user->name` çalışmaz)
- **Yalnızca metot arayüzü** - Tüm etkileşimler, tanımladığınız metotlar aracılığıyla gerçekleşmelidir
- **Daha iyi kapsülleme** - Dahili veri yapısı tamamen Go kodu tarafından kontrol edilir
- **Tür güvenliği** - PHP kodunun yanlış türlerle dahili durumu bozma riski yok
- **Daha temiz API** - Uygun bir genel arayüz tasarlamaya zorlar

Bu yaklaşım, daha iyi kapsülleme sağlar ve PHP kodunun Go nesnelerinizin dahili durumunu yanlışlıkla bozmasını önler. Nesneyle olan tüm etkileşimler, açıkça tanımladığınız metotlar aracılığıyla gerçekleşmelidir.

#### Sınıflara Metot Ekleme

Özellikler doğrudan erişilebilir olmadığından, saydam olmayan sınıflarınızla etkileşim kurmak için **metotlar tanımlamanız gerekir**. Davranışı tanımlamak için `//export_php:method` yönergesini kullanın:

```go
package example

// #include <Zend/zend_types.h>
import "C"
import (
    "unsafe"

    "github.com/dunglas/frankenphp"
)

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}

//export_php:method User::getName(): string
func (us *UserStruct) GetUserName() unsafe.Pointer {
    return frankenphp.PHPString(us.Name, false)
}

//export_php:method User::setAge(int $age): void
func (us *UserStruct) SetUserAge(age int64) {
    us.Age = int(age)
}

//export_php:method User::getAge(): int
func (us *UserStruct) GetUserAge() int64 {
    return int64(us.Age)
}

//export_php:method User::setNamePrefix(string $prefix = "User"): void
func (us *UserStruct) SetNamePrefix(prefix *C.zend_string) {
    us.Name = frankenphp.GoString(unsafe.Pointer(prefix)) + ": " + us.Name
}
```

#### Boş Geçilebilir Parametreler (Nullable Parameters)

Üretici, PHP imzalarında `?` önekini kullanarak boş geçilebilir parametreleri destekler. Bir parametre boş geçilebilir olduğunda, Go fonksiyonunuzda bir işaretçiye dönüşür ve böylece PHP'de değerin `null` olup olmadığını kontrol etmenizi sağlar:

```go
package example

// #include <Zend/zend_types.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // Check if name was provided (not null)
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }

    // Check if age was provided (not null)
    if age != nil {
        us.Age = int(*age)
    }

    // Check if active was provided (not null)
    if active != nil {
        us.Active = *active
    }
}
```

**Boş geçilebilir parametreler hakkında önemli noktalar:**

- **Boş geçilebilir ilkel türler** (`?int`, `?float`, `?bool`) Go'da işaretçilere (`*int64`, `*float64`, `*bool`) dönüşür
- **Boş geçilebilir dizeler** (`?string`) `*C.zend_string` olarak kalır ancak `nil` olabilir
- İşaretçi değerlerini referans almadan önce `nil` kontrolü yapın
- **PHP `null` Go `nil` olur** - PHP `null` geçirdiğinde, Go fonksiyonunuz bir `nil` işaretçi alır

> [!WARNING]
>
> Şu anda sınıf metotlarının aşağıdaki sınırlamaları bulunmaktadır. Parametre türleri veya dönüş türleri olarak **nesneler desteklenmemektedir**. **Diziler**, hem parametreler hem de dönüş türleri için tam olarak desteklenmektedir. Desteklenen türler: `string`, `int`, `float`, `bool`, `array` ve `void` (dönüş türü için). **Boş geçilebilir parametre türleri**, tüm skaler türler (`?string`, `?int`, `?float`, `?bool`) için tam olarak desteklenmektedir.

Eklentiyi oluşturduktan sonra, sınıfı ve metotlarını PHP'de kullanmanıza izin verilecektir. Özelliklere **doğrudan erişemeyeceğinizi** unutmayın:

```php
<?php

$user = new User();

// ✅ Bu çalışır - metotları kullanarak
$user->setAge(25);
echo $user->getName();           // Çıktı: (boş, varsayılan değer)
echo $user->getAge();            // Çıktı: 25
$user->setNamePrefix("Employee");

// ✅ Bu da çalışır - boş geçilebilir parametreler
$user->updateInfo("John", 30, true);        // Tüm parametreler sağlandı
$user->updateInfo("Jane", null, false);     // Yaş null
$user->updateInfo(null, 25, null);          // Ad ve aktif null

// ❌ Bu ÇALIŞMAZ - doğrudan özellik erişimi
// echo $user->name;             // Hata: Özel özelliğe erişilemez
// $user->age = 30;              // Hata: Özel özelliğe erişilemez
```

Bu tasarım, Go kodunuzun nesnenin durumuna nasıl erişildiğini ve değiştirildiğini tamamen kontrol etmesini sağlayarak daha iyi kapsülleme ve tür güvenliği sunar.

### Sabitleri Bildirme

Üretici, Go sabitlerini PHP'ye iki yönerge kullanarak dışa aktarmayı destekler: genel sabitler için `//export_php:const` ve sınıf sabitleri için `//export_php:classconst`. Bu, yapılandırma değerlerini, durum kodlarını ve diğer sabitleri Go ve PHP kodu arasında paylaşmanıza olanak tanır.

#### Genel Sabitler

Genel PHP sabitleri oluşturmak için `//export_php:const` yönergesini kullanın:

```go
package example

//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const
const API_VERSION = "1.2.3"

//export_php:const
const STATUS_OK = iota

//export_php:const
const STATUS_ERROR = iota
```

#### Sınıf Sabitleri

Belirli bir PHP sınıfına ait sabitler oluşturmak için `//export_php:classconst ClassName` yönergesini kullanın:

```go
package example

//export_php:classconst User
const STATUS_ACTIVE = 1

//export_php:classconst User
const STATUS_INACTIVE = 0

//export_php:classconst User
const ROLE_ADMIN = "admin"

//export_php:classconst Order
const STATE_PENDING = iota

//export_php:classconst Order
const STATE_PROCESSING = iota

//export_php:classconst Order
const STATE_COMPLETED = iota
```

Sınıf sabitleri, PHP'deki sınıf adı kapsamı kullanılarak erişilebilir:

```php
<?php

// Genel sabitler
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// Sınıf sabitleri
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

Yönerge, dizeler, tam sayılar, boole'ler, ondalık sayılar ve iota sabitleri dahil olmak üzere çeşitli değer türlerini destekler. `iota` kullanıldığında, üretici otomatik olarak sıralı değerler (0, 1, 2 vb.) atar. Genel sabitler, PHP kodunuzda genel sabitler olarak erişilebilir hale gelirken, sınıf sabitleri herkese açık görünürlük kullanılarak kendi sınıflarına göre kapsamlandırılır. Tam sayılar kullanıldığında, farklı olası gösterimler (ikili, onaltılık, sekizlik) desteklenir ve PHP stub dosyasına olduğu gibi yazılır.

Sabitleri Go kodunda alıştığınız gibi kullanabilirsiniz. Örneğin, daha önce bildirdiğimiz `repeat_this()` fonksiyonunu ele alalım ve son argümanı bir tam sayıya değiştirelim:

```go
package example

// #include <Zend/zend_types.h>
import "C"
import (
	"strings"
	"unsafe"

	"github.com/dunglas/frankenphp"
)

//export_php:const
const STR_REVERSE = iota

//export_php:const
const STR_NORMAL = iota

//export_php:classconst StringProcessor
const MODE_LOWERCASE = 1

//export_php:classconst StringProcessor
const MODE_UPPERCASE = 2

//export_php:function repeat_this(string $str, int $count, int $mode): string
func repeat_this(s *C.zend_string, count int64, mode int) unsafe.Pointer {
	str := frankenphp.GoString(unsafe.Pointer(s))

	result := strings.Repeat(str, int(count))
	if mode == STR_REVERSE {
		// reverse the string
	}

	if mode == STR_NORMAL {
		// no-op, just to showcase the constant
	}

	return frankenphp.PHPString(str, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
	// internal fields
}

//export_php:method StringProcessor::process(string $input, int $mode): string
func (sp *StringProcessorStruct) Process(input *C.zend_string, mode int64) unsafe.Pointer {
	str := frankenphp.GoString(unsafe.Pointer(input))

	switch mode {
	case MODE_LOWERCASE:
		str = strings.ToLower(str)
	case MODE_UPPERCASE:
		str = strings.ToUpper(str)
	}

	return frankenphp.PHPString(str, false)
}
```

### Ad Alanlarını Kullanma (Namespaces)

Üretici, PHP eklentinizin fonksiyonlarını, sınıflarını ve sabitlerini `//export_php:namespace` yönergesini kullanarak bir ad alanı altında düzenlemeyi destekler. Bu, adlandırma çakışmalarını önlemeye yardımcı olur ve eklentinizin API'si için daha iyi bir organizasyon sağlar.

#### Bir Ad Alanı Bildirme

Tüm dışa aktarılan sembolleri belirli bir ad alanı altına yerleştirmek için Go dosyanızın en üstünde `//export_php:namespace` yönergesini kullanın:

```go
//export_php:namespace My\Extension
package example

import (
    "unsafe"

    "github.com/dunglas/frankenphp"
)

//export_php:function hello(): string
func hello() string {
    return "Hello from My\\Extension namespace!"
}

//export_php:class User
type UserStruct struct {
    // internal fields
}

//export_php:method User::getName(): string
func (u *UserStruct) GetName() unsafe.Pointer {
    return frankenphp.PHPString("John Doe", false)
}

//export_php:const
const STATUS_ACTIVE = 1
```

#### Ad Alanlı Eklentiyi PHP'de Kullanma

Bir ad alanı bildirildiğinde, tüm fonksiyonlar, sınıflar ve sabitler PHP'de o ad alanı altına yerleştirilir:

```php
<?php

echo My\Extension\hello(); // "Hello from My\Extension namespace!"

$user = new My\Extension\User();
echo $user->getName(); // "John Doe"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### Önemli Notlar

- Dosya başına yalnızca **bir** ad alanı yönergesine izin verilir. Birden fazla ad alanı yönergesi bulunursa, üretici bir hata döndürür.
- Ad alanı, dosyada dışa aktarılan **tüm** semboller için geçerlidir: fonksiyonlar, sınıflar, metotlar ve sabitler.
- Ad alanı adları, ayırıcı olarak ters eğik çizgi (`\`) kullanarak PHP ad alanı kurallarına uyar.
- Hiçbir ad alanı bildirilmezse, semboller her zamanki gibi genel ad alanına dışa aktarılır.

### Eklentiyi Oluşturma

İşte sihrin gerçekleştiği yer burası ve eklentiniz artık oluşturulabilir. Üreticiyi aşağıdaki komutla çalıştırabilirsiniz:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go
```

> [!NOTE]
> `GEN_STUB_SCRIPT` ortam değişkenini, daha önce indirdiğiniz PHP kaynaklarındaki `gen_stub.php` dosyasının yoluna ayarlamayı unutmayın. Bu, manuel uygulama bölümünde bahsedilen aynı `gen_stub.php` betiğidir.

Her şey yolunda gittiyse, `build` adında yeni bir dizin oluşturulmuş olmalıdır. Bu dizin, oluşturulan PHP fonksiyon taslaklarını içeren `my_extension.go` dosyası da dahil olmak üzere eklentiniz için oluşturulan dosyaları içerir.

### Oluşturulan Eklentiyi FrankenPHP'ye Entegre Etme

Eklentimiz artık derlenmeye ve FrankenPHP'ye entegre edilmeye hazır. Bunu yapmak için, FrankenPHP'yi nasıl derleyeceğinizi öğrenmek üzere FrankenPHP [derleme belgelerine](compile.md) başvurun. `--with` bayrağını kullanarak modülünüzün yolunu işaret ederek modülü ekleyin:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Üretim adımı sırasında oluşturulan `/build` alt dizinini işaret ettiğinizi unutmayın. Ancak bu zorunlu değildir: Oluşturulan dosyaları modül dizininize kopyalayıp doğrudan orayı da işaret edebilirsiniz.

### Oluşturulan Eklentinizi Test Etme

Oluşturduğunuz fonksiyonları ve sınıfları test etmek için bir PHP dosyası oluşturabilirsiniz. Örneğin, aşağıdaki içeriğe sahip bir `index.php` dosyası oluşturun:

```php
<?php

// Genel sabitleri kullanarak
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// Sınıf sabitlerini kullanarak
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

Eklentinizi önceki bölümde gösterildiği gibi FrankenPHP'ye entegre ettikten sonra, bu test dosyasını `./frankenphp php-server` kullanarak çalıştırabilirsiniz ve eklentinizin çalıştığını görmelisiniz.

## Manuel Uygulama

Eklentilerin nasıl çalıştığını anlamak veya eklentiniz üzerinde tam kontrole sahip olmak istiyorsanız, bunları manuel olarak yazabilirsiniz. Bu yaklaşım size tam kontrol sağlar ancak daha fazla temel kod (boilerplate) gerektirir.

### Temel Fonksiyon

Go dilinde yeni bir yerel fonksiyon tanımlayan basit bir PHP eklentisi nasıl yazılacağını göreceğiz. Bu fonksiyon PHP'den çağrılacak ve Caddy'nin loglarına bir mesaj kaydeden bir gorutin tetikleyecektir. Bu fonksiyon herhangi bir parametre almaz ve hiçbir şey döndürmez.

#### Go Fonksiyonunu Tanımlama

Modülünüzde, PHP'den çağrılacak yeni bir yerel fonksiyon tanımlamanız gerekir. Bunu yapmak için, örneğin `extension.go` adında istediğiniz bir dosya oluşturun ve aşağıdaki kodu ekleyin:

```go
package example

// #include "extension.h"
import "C"
import (
	"log/slog"
	"unsafe"

	"github.com/dunglas/frankenphp"
)

func init() {
	frankenphp.RegisterExtension(unsafe.Pointer(&C.ext_module_entry))
}

//export go_print_something
func go_print_something() {
	go func() {
		slog.Info("Hello from a goroutine!")
	}()
}
```

`frankenphp.RegisterExtension()` fonksiyonu, dahili PHP kayıt mantığını ele alarak eklenti kayıt sürecini basitleştirir. `go_print_something` fonksiyonu, CGO sayesinde yazacağımız C kodunda erişilebilir olacağını belirtmek için `//export` yönergesini kullanır.

Bu örnekte, yeni fonksiyonumuz Caddy'nin loglarına bir mesaj kaydeden bir gorutin tetikleyecektir.

#### PHP Fonksiyonunu Tanımlama

PHP'nin fonksiyonumuzu çağırabilmesi için ilgili bir PHP fonksiyonu tanımlamamız gerekiyor. Bunun için, örneğin `extension.stub.php` adında bir taslak dosya oluşturacağız ve bu dosya aşağıdaki kodu içerecektir:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

Bu dosya, PHP'den çağrılacak olan `go_print()` fonksiyonunun imzasını tanımlar. `@generate-class-entries` yönergesi, PHP'nin eklentimiz için fonksiyon girişlerini otomatik olarak oluşturmasına olanak tanır.

Bu manuel olarak değil, PHP kaynaklarında sağlanan bir betik kullanılarak yapılır (PHP kaynaklarınızın nerede olduğuna bağlı olarak `gen_stub.php` betiğinin yolunu ayarladığınızdan emin olun):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Bu betik, PHP'nin fonksiyonumuzu nasıl tanımlayacağını ve çağıracağını bilmesi için gerekli bilgileri içeren `extension_arginfo.h` adında bir dosya oluşturacaktır.

#### Go ve C Arasındaki Köprüyü Yazma

Şimdi Go ve C arasındaki köprüyü yazmamız gerekiyor. Modül dizininizde `extension.h` adında bir dosya oluşturun ve aşağıdaki içeriği ekleyin:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Ardından, aşağıdaki adımları gerçekleştirecek `extension.c` adında bir dosya oluşturun:

- PHP başlıklarını dahil etme;
- Yeni yerel PHP fonksiyonumuz `go_print()`'i bildirme;
- Eklenti meta verilerini bildirme.

Gerekli başlıkları dahil ederek başlayalım:

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Go tarafından dışa aktarılan sembolleri içerir
#include "_cgo_export.h"
```

Daha sonra PHP fonksiyonumuzu yerel bir dil fonksiyonu olarak tanımlıyoruz:

```c
PHP_FUNCTION(go_print)
{
    ZEND_PARSE_PARAMETERS_NONE();

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* Fonksiyonlar */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

Bu durumda, fonksiyonumuz hiçbir parametre almaz ve hiçbir şey döndürmez. Daha önce tanımladığımız ve `//export` yönergesi kullanılarak dışa aktarılan Go fonksiyonunu çağırmaktadır.

Son olarak, eklentinin adını, sürümünü ve özelliklerini içeren `zend_module_entry` yapısında meta verilerini tanımlıyoruz. Bu bilgiler, PHP'nin eklentimizi tanıması ve yüklemesi için gereklidir. `ext_functions`'ın tanımladığımız PHP fonksiyonlarına işaretçilerden oluşan bir dizi olduğunu ve `gen_stub.php` betiği tarafından `extension_arginfo.h` dosyasında otomatik olarak oluşturulduğunu unutmayın.

Eklenti kaydı, Go kodumuzda çağırdığımız FrankenPHP'nin `RegisterExtension()` fonksiyonu tarafından otomatik olarak halledilir.

### Gelişmiş Kullanım

Şimdi Go'da temel bir PHP eklentisi oluşturmayı bildiğimize göre, örneğimizi karmaşıklaştıralım. Şimdi parametre olarak bir dize alan ve büyük harfli versiyonunu döndüren bir PHP fonksiyonu oluşturacağız.

#### PHP Fonksiyon Taslağını Tanımlama

Yeni PHP fonksiyonunu tanımlamak için, `extension.stub.php` dosyamızı yeni fonksiyon imzasını içerecek şekilde değiştireceğiz:

```php
<?php

/** @generate-class-entries */

/**
 * Bir dizeyi büyük harflere dönüştürür.
 *
 * @param string $string Dönüştürülecek dize.
 * @return string Dizinin büyük harfli versiyonu.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> Fonksiyonlarınızın dokümantasyonunu ihmal etmeyin! Eklenti taslaklarınızı diğer geliştiricilerle paylaşarak eklentinizin nasıl kullanılacağını ve hangi özelliklerin mevcut olduğunu belgeleyebilirsiniz.

Taslak dosyasını `gen_stub.php` betiği ile yeniden oluşturarak, `extension_arginfo.h` dosyası şöyle görünmelidir:

```c
ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_go_upper, 0, 1, IS_STRING, 0)
    ZEND_ARG_TYPE_INFO(0, string, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_FUNCTION(go_upper);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(go_upper, arginfo_go_upper)
    ZEND_FE_END
};
```

`go_upper` fonksiyonunun `string` tipinde bir parametre ve `string` tipinde bir dönüş değeriyle tanımlandığını görebiliriz.

#### Go ve PHP/C Arasında Tür Dengeleme

Go fonksiyonunuz doğrudan bir PHP dizesini parametre olarak kabul edemez. Onu bir Go dizesine dönüştürmeniz gerekir. Neyse ki FrankenPHP, üretici yaklaşımında gördüğümüze benzer şekilde, PHP dizeleri ile Go dizeleri arasındaki dönüşümü yönetmek için yardımcı fonksiyonlar sağlar.

Başlık dosyası basit kalır:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Şimdi `extension.c` dosyamızda Go ve C arasındaki köprüyü yazabiliriz. PHP dizesini doğrudan Go fonksiyonumuza geçireceğiz:

```c
PHP_FUNCTION(go_upper)
{
    zend_string *str;

    ZEND_PARSE_PARAMETERS_START(1, 1)
        Z_PARAM_STR(str)
    ZEND_PARSE_PARAMETERS_END();

    zend_string *result = go_upper(str);
    RETVAL_STR(result);
}
```

`ZEND_PARSE_PARAMETERS_START` ve parametre ayrıştırma hakkında daha fazla bilgiyi [PHP Dahili Kitabı'nın](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters) ilgili sayfasında bulabilirsiniz. Burada, PHP'ye fonksiyonumuzun `zend_string` tipinde bir zorunlu parametre aldığını söylüyoruz. Daha sonra bu dizeyi doğrudan Go fonksiyonumuza iletiyor ve sonucu `RETVAL_STR` kullanarak döndürüyoruz.

Yapılacak tek bir şey kaldı: `go_upper` fonksiyonunu Go'da uygulamak.

#### Go Fonksiyonunu Uygulama

Go fonksiyonumuz parametre olarak bir `*C.zend_string` alacak, FrankenPHP'nin yardımcı fonksiyonunu kullanarak onu bir Go dizesine dönüştürecek, işleyecek ve sonucu yeni bir `*C.zend_string` olarak döndürecektir. Yardımcı fonksiyonlar, tüm bellek yönetimi ve dönüşüm karmaşıklığını bizim için halleder.

```go
package example

// #include <Zend/zend_types.h>
import "C"
import (
    "unsafe"
    "strings"

    "github.com/dunglas/frankenphp"
)

//export go_upper
func go_upper(s *C.zend_string) *C.zend_string {
    str := frankenphp.GoString(unsafe.Pointer(s))

    upper := strings.ToUpper(str)

    return (*C.zend_string)(frankenphp.PHPString(upper, false))
}
```

Bu yaklaşım, manuel bellek yönetiminden çok daha temiz ve güvenlidir.
FrankenPHP'nin yardımcı fonksiyonları, PHP'nin `zend_string` formatı ile Go dizeleri arasındaki dönüşümü otomatik olarak halleder.
`PHPString()`'deki `false` parametresi, yeni bir kalıcı olmayan dize oluşturmak istediğimizi belirtir (isteğin sonunda serbest bırakılır).

> [!TIP]
>
> Bu örnekte herhangi bir hata işleme yapmıyoruz, ancak Go fonksiyonlarınızda kullanmadan önce her zaman işaretçilerin `nil` olmadığını ve verilerin geçerli olduğunu kontrol etmelisiniz.

### Eklentiyi FrankenPHP'ye Entegre Etme

Eklentimiz artık derlenmeye ve FrankenPHP'ye entegre edilmeye hazır. Bunu yapmak için, FrankenPHP'yi nasıl derleyeceğinizi öğrenmek üzere FrankenPHP [derleme belgelerine](compile.md) başvurun. `--with` bayrağını kullanarak modülünüzün yolunu işaret ederek modülü ekleyin:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

İşte bu kadar! Eklentiniz artık FrankenPHP'ye entegre edildi ve PHP kodunuzda kullanılabilir.

### Eklentinizi Test Etme

Eklentinizi FrankenPHP'ye entegre ettikten sonra, uyguladığınız fonksiyonlar için örnekler içeren bir `index.php` dosyası oluşturabilirsiniz:

```php
<?php

// Temel fonksiyonu test edin
go_print();

// Gelişmiş fonksiyonu test edin
echo go_upper("hello world") . "\n";
```

Şimdi FrankenPHP'yi bu dosya ile `./frankenphp php-server` kullanarak çalıştırabilirsiniz ve eklentinizin çalıştığını görmelisiniz.
