# Go ile PHP Uzantıları Yazma

FrankenPHP ile **Go dilinde PHP uzantıları yazabilir**, bu da doğrudan PHP'den çağrılabilen **yüksek performanslı yerel işlevler** oluşturmanıza olanak tanır. Uygulamalarınız, mevcut veya yeni herhangi bir Go kütüphanesini ve **goroutine'lerin meşhur eşzamanlılık modelini doğrudan PHP kodunuzdan** kullanabilir.

PHP uzantıları tipik olarak C dilinde yazılır, ancak biraz ek çalışma ile başka dillerde de yazmak mümkündür. PHP uzantıları, örneğin yerel işlevler ekleyerek veya belirli işlemleri optimize ederek PHP'nin işlevselliğini genişletmek için düşük seviyeli dillerin gücünden yararlanmanızı sağlar.

Caddy modülleri sayesinde, Go dilinde PHP uzantıları yazabilir ve bunları FrankenPHP'ye çok hızlı bir şekilde entegre edebilirsiniz.

## İki Yaklaşım

FrankenPHP, Go dilinde PHP uzantıları oluşturmak için iki yol sunar:

1.  **Uzantı Oluşturucuyu Kullanma** - Çoğu kullanım durumu için gerekli tüm boilerplate kodunu üreten, Go kodunuzu yazmaya odaklanmanızı sağlayan önerilen yaklaşım.
2.  **Manuel Uygulama** - Gelişmiş kullanım durumları için uzantı yapısı üzerinde tam kontrol.

Başlamak için en kolay yol olduğu için oluşturucu yaklaşımıyla başlayacak, ardından tam kontrole ihtiyaç duyanlar için manuel uygulamayı göstereceğiz.

## Uzantı Oluşturucuyu Kullanma

FrankenPHP, yalnızca Go kullanarak **bir PHP uzantısı oluşturmanıza** olanak tanıyan bir araçla birlikte gelir. **C kodu yazmaya** veya doğrudan CGO kullanmaya gerek yok: FrankenPHP ayrıca, **PHP/C ve Go arasındaki tür dengeleme** konusunda endişelenmenize gerek kalmadan uzantılarınızı Go'da yazmanıza yardımcı olacak **genel bir tür API'si** içerir.

> [!TIP]
> Uzantıların Go'da sıfırdan nasıl yazılabileceğini anlamak isterseniz, oluşturucuyu kullanmadan bir PHP uzantısının Go'da nasıl yazılacağını gösteren manuel uygulama bölümünü okuyabilirsiniz.

Bu aracın **tam teşekküllü bir uzantı oluşturucu olmadığını** unutmayın. Go'da basit uzantılar yazmanıza yardımcı olmak için tasarlanmıştır, ancak PHP uzantılarının en gelişmiş özelliklerini sağlamaz. Daha **karmaşık ve optimize edilmiş** bir uzantı yazmanız gerekiyorsa, doğrudan bazı C kodu yazmanız veya CGO kullanmanız gerekebilir.

### Ön Koşullar

Aşağıdaki manuel uygulama bölümünde de belirtildiği gibi, [PHP kaynaklarını edinmeniz](https://www.php.net/downloads.php) ve yeni bir Go modülü oluşturmanız gerekir.

#### Yeni Bir Modül Oluşturun ve PHP Kaynaklarını Edinin

Go'da bir PHP uzantısı yazmanın ilk adımı yeni bir Go modülü oluşturmaktır. Bunu aşağıdaki komutu kullanarak yapabilirsiniz:

```console
go mod init example.com/example
```

İkinci adım, sonraki adımlar için [PHP kaynaklarını edinmektir](https://www.php.net/downloads.php). Bunları edindikten sonra, Go modülünüzün içine değil, istediğiniz dizine açın:

```console
tar xf php-*
```

### Uzantıyı Yazma

Yerel işlevinizi Go'da yazmak için her şey hazır. `stringext.go` adında yeni bir dosya oluşturun. İlk işlevimiz bir dizeyi argüman olarak, tekrar sayısını, dizeyi ters çevirip çevirmeyeceğini belirten bir boole değeri alacak ve sonuç dizeyi döndürecektir. Bu şöyle görünmelidir:

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

-   `//export_php:function` yönerge yorumu, PHP'deki işlev imzasını tanımlar. Oluşturucu, PHP işlevini doğru parametreler ve dönüş türüyle nasıl oluşturacağını bu şekilde bilir;
-   İşlev bir `unsafe.Pointer` döndürmelidir. FrankenPHP, C ve Go arasındaki tür dengeleme konusunda size yardımcı olacak bir API sağlar.

İlk nokta kendini açıklarken, ikincisi kavranması daha zor olabilir. Bu kılavuzda tür dengeleme konusuna daha derinlemesine bakalım.

### Uzantıyı Oluşturma

İşte sihrin gerçekleştiği yer ve uzantınız artık oluşturulabilir. Oluşturucuyu aşağıdaki komutla çalıştırabilirsiniz:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go
```

> [!NOTE]
> `GEN_STUB_SCRIPT` ortam değişkenini daha önce indirdiğiniz PHP kaynaklarındaki `gen_stub.php` dosyasının yoluna ayarlamayı unutmayın. Bu, manuel uygulama bölümünde bahsedilen aynı `gen_stub.php` betiğidir.

Her şey yolunda giderse, projenizin dizini uzantınız için aşağıdaki dosyaları içermelidir:

-   **`my_extension.go`** - Orijinal kaynak dosyanız (değişmeden kalır)
-   **`my_extension_generated.go`** - İşlevlerinizi çağıran CGO sarıcılarıyla oluşturulan dosya
-   **`my_extension.stub.php`** - IDE otomatik tamamlama için PHP stub dosyası
-   **`my_extension_arginfo.h`** - PHP argüman bilgisi
-   **`my_extension.h`** - C başlık dosyası
-   **`my_extension.c`** - C uygulama dosyası
-   **`README.md`** - Dokümantasyon

> [!IMPORTANT]
> **Kaynak dosyanız (`my_extension.go`) asla değiştirilmez.** Oluşturucu, orijinal işlevlerinizi çağıran CGO sarıcılarını içeren ayrı bir `_generated.go` dosyası oluşturur. Bu, kaynak dosyanızı oluşturulan kodun kirletmesi endişesi olmadan güvenle sürüm kontrolüne alabileceğiniz anlamına gelir.

### Oluşturulan Uzantıyı FrankenPHP'ye Entegre Etme

Uzantımız artık derlenmeye ve FrankenPHP'ye entegre edilmeye hazır. Bunu yapmak için, FrankenPHP'yi nasıl derleyeceğinizi öğrenmek üzere FrankenPHP [derleme dokümantasyonuna](compile.md) bakın. Modülü `--with` bayrağını kullanarak ekleyin ve modülünüzün yolunu işaret edin:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Oluşturma adımı sırasında oluşturulan `/build` alt dizinini işaret ettiğinize dikkat edin. Ancak bu zorunlu değildir: oluşturulan dosyaları modül dizininize de kopyalayabilir ve doğrudan ona işaret edebilirsiniz.

### Oluşturulan Uzantınızı Test Etme

Oluşturduğunuz işlevleri ve sınıfları test etmek için bir PHP dosyası oluşturabilirsiniz. Örneğin, aşağıdaki içeriğe sahip bir `index.php` dosyası oluşturun:

```php
<?php

// Genel sabitleri kullanma
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// Sınıf sabitlerini kullanma
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

Uzantınızı önceki bölümde gösterildiği gibi FrankenPHP'ye entegre ettikten sonra, bu test dosyasını `./frankenphp php-server` kullanarak çalıştırabilir ve uzantınızın çalıştığını görmelisiniz.

### Tür Dengeleme

Bazı değişken türleri C/PHP ve Go arasında aynı bellek gösterimine sahipken, bazı türlerin doğrudan kullanılabilmesi için daha fazla mantık gerektirir. Uzantı yazarken belki de en zor kısım budur çünkü Zend Engine'in iç işleyişini ve değişkenlerin PHP'de dahili olarak nasıl saklandığını anlamayı gerektirir.
Bu tablo, bilmeniz gerekenleri özetler:

| PHP türü           | Go türü                       | Doğrudan dönüşüm | C'den Go'ya yardımcı                    | Go'dan C'ye yardımcı                     | Sınıf Metotları Desteği |
| :----------------- | :---------------------------- | :---------------- | :-------------------------------------- | :--------------------------------------- | :---------------------- |
| `int`              | `int64`                       | ✅                | -                                       | -                                        | ✅                      |
| `?int`             | `*int64`                      | ✅                | -                                       | -                                        | ✅                      |
| `float`            | `float64`                     | ✅                | -                                       | -                                        | ✅                      |
| `?float`           | `*float64`                    | ✅                | -                                       | -                                        | ✅                      |
| `bool`             | `bool`                        | ✅                | -                                       | -                                        | ✅                      |
| `?bool`            | `*bool`                       | ✅                | -                                       | -                                        | ✅                      |
| `string`/`?string` | `*C.zend_string`              | ❌                | `frankenphp.GoString()`                 | `frankenphp.PHPString()`                 | ✅                      |
| `array`            | `frankenphp.AssociativeArray` | ❌                | `frankenphp.GoAssociativeArray()`       | `frankenphp.PHPAssociativeArray()`      | ✅                      |
| `array`            | `map[string]any`              | ❌                | `frankenphp.GoMap()`                    | `frankenphp.PHPMap()`                    | ✅                      |
| `array`            | `[]any`                       | ❌                | `frankenphp.GoPackedArray()`            | `frankenphp.PHPPackedArray()`            | ✅                      |
| `mixed`            | `any`                         | ❌                | `GoValue()`                             | `PHPValue()`                             | ❌                      |
| `callable`         | `*C.zval`                     | ❌                | -                                       | frankenphp.CallPHPCallable()             | ❌                      |
| `object`           | `struct`                      | ❌                | _Henüz uygulanmadı_                      | _Henüz uygulanmadı_                       | ❌                      |

> [!NOTE]
>
> Bu tablo henüz kapsamlı değildir ve FrankenPHP tür API'si daha eksiksiz hale geldikçe tamamlanacaktır.
>
> Özellikle sınıf metotları için, ilkel türler ve diziler şu anda desteklenmektedir. Nesneler henüz metot parametresi veya dönüş türü olarak kullanılamaz.

Önceki bölümdeki kod parçacığına bakarsanız, ilk parametreyi ve dönüş değerini dönüştürmek için yardımcıların kullanıldığını görebilirsiniz. `repeat_this()` işlevimizin ikinci ve üçüncü parametrelerinin dönüştürülmesi gerekmez, çünkü temel türlerin bellek gösterimi hem C hem de Go için aynıdır.

#### Dizilerle Çalışma

FrankenPHP, `frankenphp.AssociativeArray` aracılığıyla veya doğrudan bir haritaya veya dilime dönüştürülerek PHP dizileri için yerel destek sağlar.

`AssociativeArray`, bir `Map: map[string]any` alanı ve isteğe bağlı bir `Order: []string` alanından oluşan bir [karma haritayı](https://en.wikipedia.org/wiki/Hash_table) temsil eder (PHP "ilişkisel dizilerinin" aksine, Go haritaları sıralı değildir).

Sıra veya ilişkilendirme gerekli değilse, doğrudan bir dilim `[]any` veya sırasız bir harita `map[string]any`'e dönüştürmek de mümkündür.

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
	// PHP ilişkisel dizisini sırayı koruyarak Go'ya dönüştür
	associativeArray, err := frankenphp.GoAssociativeArray[any](unsafe.Pointer(arr))
    if err != nil {
        // hatayı ele al
    }

	// girdiler üzerinde sırayla döngü yap
	for _, key := range associativeArray.Order {
		value, _ := associativeArray.Map[key]
		// anahtar ve değer ile bir şeyler yap
	}

	// sıralı bir dizi döndür
	// eğer 'Order' boş değilse, yalnızca 'Order' içindeki anahtar-değer çiftleri dikkate alınacaktır
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
	// PHP ilişkisel dizisini sırayı korumadan bir Go haritasına dönüştür
	// sırayı göz ardı etmek daha performanslı olacaktır
	goMap, err := frankenphp.GoMap[any](unsafe.Pointer(arr))
    if err != nil {
        // hatayı ele al
    }

	// belirli bir sıra olmadan girdiler üzerinde döngü yap
	for key, value := range goMap {
		// anahtar ve değer ile bir şeyler yap
	}

	// sırasız bir dizi döndür
	return frankenphp.PHPMap(map[string]string {
		"key1": "value1",
		"key2": "value2",
	})
}

// export_php:function process_data_packed(array $input): array
func process_data_packed(arr *C.zend_array) unsafe.Pointer {
	// PHP sıkıştırılmış dizisini Go'ya dönüştür
	goSlice, err := frankenphp.GoPackedArray(unsafe.Pointer(arr))
    if err != nil {
        // hatayı ele al
    }

	// dilim üzerinde sırayla döngü yap
	for index, value := range goSlice {
		// indeks ve değer ile bir şeyler yap
	}

	// sıkıştırılmış bir dizi döndür
	return frankenphp.PHPPackedArray([]string{"value1", "value2", "value3"})
}
```

**Dizi dönüştürmenin temel özellikleri:**

-   **Sıralı anahtar-değer çiftleri** - İlişkisel dizinin sırasını koruma seçeneği
-   **Birden fazla durum için optimize edildi** - Daha iyi performans için sırayı bırakma veya doğrudan bir dilime dönüştürme seçeneği
-   **Otomatik liste tespiti** - PHP'ye dönüştürürken, dizinin sıkıştırılmış bir liste mi yoksa karma harita mı olması gerektiğini otomatik olarak algılar
-   **İç içe diziler** - Diziler iç içe olabilir ve tüm desteklenen türleri otomatik olarak dönüştürecektir (`int64`, `float64`, `string`, `bool`, `nil`, `AssociativeArray`, `map[string]any`, `[]any`)
-   **Nesneler desteklenmiyor** - Şu anda yalnızca skaler türler ve diziler değer olarak kullanılabilir. Bir nesne sağlamak, PHP dizisinde `null` değeriyle sonuçlanacaktır.

##### Mevcut metotlar: Sıkıştırılmış ve İlişkisel

-   `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer` - Anahtar-değer çiftleri içeren sıralı bir PHP dizisine dönüştür
-   `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` - Bir haritayı anahtar-değer çiftleri içeren sırasız bir PHP dizisine dönüştür
-   `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` - Bir dilimi yalnızca indekslenmiş değerlere sahip bir PHP sıkıştırılmış dizisine dönüştür
-   `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray` - Bir PHP dizisini sıralı bir Go `AssociativeArray`'e (sıralı harita) dönüştür
-   `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` - Bir PHP dizisini sırasız bir Go haritasına dönüştür
-   `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` - Bir PHP dizisini bir Go dilimine dönüştür
-   `frankenphp.IsPacked(zval *C.zend_array) bool` - Bir PHP dizisinin sıkıştırılmış (yalnızca indekslenmiş) mı yoksa ilişkisel (anahtar-değer çiftleri) mi olduğunu kontrol et

### Çağrılabilirlerle Çalışma

FrankenPHP, `frankenphp.CallPHPCallable` yardımcısını kullanarak PHP çağrılabilirleriyle çalışmanın bir yolunu sunar. Bu, Go kodundan PHP işlevlerini veya metotlarını çağırmanıza olanak tanır.

Bunu göstermek için, çağrılabilir ve bir dizi alan, çağrılabiliri dizinin her öğesine uygulayan ve sonuçlarla yeni bir dizi döndüren kendi `array_map()` işlevimizi oluşturalım:

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

PHP'de parametre olarak geçirilen çağrılabilir'i çağırmak için `frankenphp.CallPHPCallable()`'ı nasıl kullandığımıza dikkat edin. Bu işlev, çağrılabilir'e bir işaretçi ve bir argüman dizisi alır ve çağrılabilir yürütmesinin sonucunu döndürür. Alıştığınız çağrılabilir sözdizimini kullanabilirsiniz:

```php
<?php

$result = my_array_map([1, 2, 3], function($x) { return $x * 2; });
// $result [2, 4, 6] olacaktır

$result = my_array_map(['hello', 'world'], 'strtoupper');
// $result ['HELLO', 'WORLD'] olacaktır
```

### Yerel Bir PHP Sınıfı Bildirme

Oluşturucu, Go struct'ları olarak **saydam sınıfları** bildirmeyi destekler, bunlar PHP nesneleri oluşturmak için kullanılabilir. Bir PHP sınıfı tanımlamak için `//export_php:class` yönerge yorumunu kullanabilirsiniz. Örneğin:

```go
package example

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### Saydam Sınıflar Nelerdir?

**Saydam sınıflar**, dahili yapının (özelliklerin) PHP kodundan gizlendiği sınıflardır. Bu şu anlama gelir:

-   **Doğrudan özellik erişimi yok**: PHP'den doğrudan özellik okuyamaz veya yazamazsınız (`$user->name` çalışmaz)
-   **Yalnızca metot arayüzü** - Tüm etkileşimler tanımladığınız metotlar aracılığıyla gerçekleşmelidir
-   **Daha iyi kapsülleme** - Dahili veri yapısı tamamen Go kodu tarafından kontrol edilir
-   **Tür güvenliği** - Yanlış türlerle dahili durumu bozan PHP kodu riski yok
-   **Daha temiz API** - Doğru bir genel arayüz tasarlamayı zorlar

Bu yaklaşım, daha iyi kapsülleme sağlar ve PHP kodunun Go nesnelerinizin dahili durumunu yanlışlıkla bozmasını önler. Nesneyle tüm etkileşimler, açıkça tanımladığınız metotlar aracılığıyla gerçekleşmelidir.

#### Sınıflara Metot Ekleme

Özellikler doğrudan erişilemediği için, saydam sınıflarınızla etkileşim kurmak için **metotlar tanımlamanız gerekir**. Davranışı tanımlamak için `//export_php:method` yönergesini kullanın:

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

#### Null Olabilir Parametreler

Oluşturucu, PHP imzalarında `?` önekini kullanarak null olabilir parametreleri destekler. Bir parametre null olabilir olduğunda, Go işlevinizde bir işaretçiye dönüşür ve PHP'de değerin `null` olup olmadığını kontrol etmenizi sağlar:

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
    // İsim sağlandıysa kontrol et (null değilse)
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }

    // Yaş sağlandıysa kontrol et (null değilse)
    if age != nil {
        us.Age = int(*age)
    }

    // Aktif sağlandıysa kontrol et (null değilse)
    if active != nil {
        us.Active = *active
    }
}
```

**Null olabilir parametreler hakkında önemli noktalar:**

-   **Null olabilir ilkel türler** (`?int`, `?float`, `?bool`) Go'da işaretçilere (`*int64`, `*float64`, `*bool`) dönüşür
-   **Null olabilir dizeler** (`?string`) `*C.zend_string` olarak kalır ancak `nil` olabilir
-   İşaretçi değerlerini referans almadan önce **`nil` kontrolü yapın**
-   **PHP `null` Go `nil` olur** - PHP `null` geçirdiğinde, Go işleviniz `nil` bir işaretçi alır

> [!WARNING]
>
> Şu anda, sınıf metotlarının aşağıdaki sınırlamaları vardır. **Nesneler** parametre türleri veya dönüş türleri olarak **desteklenmez**. **Diziler** hem parametreler hem de dönüş türleri için **tamamen desteklenir**. Desteklenen türler: `string`, `int`, `float`, `bool`, `array` ve `void` (dönüş türü için). Tüm skaler türler (`?string`, `?int`, `?float`, `?bool`) için **null olabilir parametre türleri tamamen desteklenir**.

Uzantıyı oluşturduktan sonra, sınıfı ve metotlarını PHP'de kullanmanıza izin verilecektir. Özelliklere doğrudan erişemeyeceğinizi unutmayın:

```php
<?php

$user = new User();

// ✅ Bu çalışır - metotları kullanma
$user->setAge(25);
echo $user->getName();           // Çıktı: (boş, varsayılan değer)
echo $user->getAge();            // Çıktı: 25
$user->setNamePrefix("Employee");

// ✅ Bu da çalışır - null olabilir parametreler
$user->updateInfo("John", 30, true);        // Tüm parametreler sağlandı
$user->updateInfo("Jane", null, false);     // Yaş null
$user->updateInfo(null, 25, null);          // İsim ve aktif null

// ❌ Bu ÇALIŞMAZ - doğrudan özellik erişimi
// echo $user->name;             // Hata: Özel özelliğe erişilemez
// $user->age = 30;              // Hata: Özel özelliğe erişilemez
```

Bu tasarım, Go kodunuzun nesnenin durumuna nasıl erişildiğini ve değiştirildiğini tamamen kontrol etmesini sağlayarak daha iyi kapsülleme ve tür güvenliği sunar.

### Sabitleri Bildirme

Oluşturucu, Go sabitlerini PHP'ye aktarmayı iki yönerge kullanarak destekler: genel sabitler için `//export_php:const` ve sınıf sabitleri için `//export_php:classconst`. Bu, Go ve PHP kodu arasında yapılandırma değerleri, durum kodları ve diğer sabitleri paylaşmanıza olanak tanır.

#### Genel Sabitler

Genel PHP sabitleri oluşturmak için `//export_php:const` yönergesini kullanın:

```go
package example

//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const
const API_VERSION = "1.2.3"

//export_php:const
const (
	STATUS_OK = iota
	STATUS_ERROR
)
```

> [!NOTE]
> PHP sabitleri, Go sabitinin adını alacaktır, bu nedenle büyük harfler kullanılması önerilir.

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
const (
	STATE_PENDING = iota
	STATE_PROCESSING
	STATE_COMPLETED
)
```

> [!NOTE]
> Tıpkı genel sabitler gibi, sınıf sabitleri de Go sabitinin adını alacaktır.

Sınıf sabitlerine PHP'de sınıf adı kapsamı kullanılarak erişilebilir:

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

Yönerge, dizeler, tam sayılar, boole değerleri, ondalıklı sayılar ve iota sabitleri dahil olmak üzere çeşitli değer türlerini destekler. `iota` kullanıldığında, oluşturucu otomatik olarak sıralı değerler (0, 1, 2 vb.) atar. Genel sabitler PHP kodunuzda genel sabitler olarak kullanılabilir hale gelirken, sınıf sabitleri genel görünürlük kullanılarak ilgili sınıflarına kapsamlandırılır. Tam sayılar kullanıldığında, farklı olası gösterimler (ikili, onaltılı, sekizli) desteklenir ve PHP stub dosyasında olduğu gibi dökülür.

Sabitleri Go kodunda alıştığınız gibi kullanabilirsiniz. Örneğin, daha önce bildirdiğimiz `repeat_this()` işlevini alıp son argümanı bir tam sayıya çevirelim:

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
		// dizeyi ters çevir
	}

	if mode == STR_NORMAL {
		// hiçbir şey yapma, sadece sabiti göstermek için
	}

	return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
	// dahili alanlar
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

### Ad Alanları Kullanma

Oluşturucu, PHP uzantınızın işlevlerini, sınıflarını ve sabitlerini `//export_php:namespace` yönergesini kullanarak bir ad alanı altında düzenlemeyi destekler. Bu, adlandırma çakışmalarını önlemeye yardımcı olur ve uzantınızın API'si için daha iyi bir organizasyon sağlar.

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
    return "My\\Extension ad alanından Merhaba!"
}

//export_php:class User
type UserStruct struct {
    // dahili alanlar
}

//export_php:method User::getName(): string
func (u *UserStruct) GetName() unsafe.Pointer {
    return frankenphp.PHPString("John Doe", false)
}

//export_php:const
const STATUS_ACTIVE = 1
```

#### Ad Alanlı Uzantıyı PHP'de Kullanma

Bir ad alanı bildirildiğinde, tüm işlevler, sınıflar ve sabitler PHP'de bu ad alanı altına yerleştirilir:

```php
<?php

echo My\Extension\hello(); // "My\Extension ad alanından Merhaba!"

$user = new My\Extension\User();
echo $user->getName(); // "John Doe"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### Önemli Notlar

-   Dosya başına yalnızca **bir** ad alanı yönergesine izin verilir. Birden fazla ad alanı yönergesi bulunursa, oluşturucu bir hata döndürür.
-   Ad alanı, dosyadaki **tüm** dışa aktarılan sembollere (işlevler, sınıflar, metotlar ve sabitler) uygulanır.
-   Ad alanı adları, ayırıcı olarak ters eğik çizgi (`\`) kullanarak PHP ad alanı kurallarına uyar.
-   Hiçbir ad alanı bildirilmezse, semboller her zamanki gibi genel ad alanına aktarılır.

## Manuel Uygulama

Uzantıların nasıl çalıştığını anlamak veya uzantınız üzerinde tam kontrole sahip olmak istiyorsanız, bunları manuel olarak yazabilirsiniz. Bu yaklaşım size tam kontrol sağlar ancak daha fazla boilerplate kodu gerektirir.

### Temel İşlev

Go'da yeni bir yerel işlev tanımlayan basit bir PHP uzantısının nasıl yazılacağını göreceğiz. Bu işlev PHP'den çağrılacak ve Caddy'nin günlüklerine bir mesaj kaydeden bir goroutine tetikleyecektir. Bu işlev herhangi bir parametre almaz ve hiçbir şey döndürmez.

#### Go İşlevini Tanımla

Modülünüzde, PHP'den çağrılacak yeni bir yerel işlev tanımlamanız gerekir. Bunu yapmak için, örneğin `extension.go` adında bir dosya oluşturun ve aşağıdaki kodu ekleyin:

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
		slog.Info("Merhaba bir goroutine'den!")
	}()
}
```

`frankenphp.RegisterExtension()` işlevi, dahili PHP kayıt mantığını ele alarak uzantı kayıt sürecini basitleştirir. `go_print_something` işlevi, CGO sayesinde yazacağımız C kodunda erişilebilir olacağını belirtmek için `//export` yönergesini kullanır.

Bu örnekte, yeni işlevimiz Caddy'nin günlüklerine bir mesaj kaydeden bir goroutine tetikleyecektir.

#### PHP İşlevini Tanımla

PHP'nin işlevimizi çağırmasına izin vermek için karşılık gelen bir PHP işlevi tanımlamamız gerekir. Bunun için, örneğin `extension.stub.php` adında bir stub dosyası oluşturacağız ve bu dosya aşağıdaki kodu içerecektir:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

Bu dosya, PHP'den çağrılacak `go_print()` işlevinin imzasını tanımlar. `@generate-class-entries` yönergesi, PHP'nin uzantımız için işlev girişlerini otomatik olarak oluşturmasına olanak tanır.

Bu manuel olarak yapılmaz, ancak PHP kaynaklarında sağlanan bir betik kullanılarak yapılır (PHP kaynaklarınızın nerede olduğuna bağlı olarak `gen_stub.php` betiğinin yolunu ayarladığınızdan emin olun):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Bu betik, PHP'nin işlevimizi nasıl tanımlayacağını ve çağıracağını bilmesi için gerekli bilgileri içeren `extension_arginfo.h` adlı bir dosya oluşturacaktır.

#### Go ve C Arasında Köprü Yazma

Şimdi, Go ve C arasında köprü yazmamız gerekiyor. Modül dizininizde `extension.h` adında bir dosya oluşturun ve içine aşağıdaki içeriği ekleyin:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Ardından, aşağıdaki adımları gerçekleştirecek `extension.c` adında bir dosya oluşturun:

-   PHP başlıklarını dahil et;
-   Yeni yerel PHP işlevimiz `go_print()`'i bildir;
-   Uzantı meta verilerini bildir.

Gerekli başlıkları dahil ederek başlayalım:

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Go tarafından dışa aktarılan sembolleri içerir
#include "_cgo_export.h"
```

Daha sonra PHP işlevimizi yerel bir dil işlevi olarak tanımlarız:

```c
PHP_FUNCTION(go_print)
{
    ZEND_PARSE_PARAMETERS_NONE();

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* İşlevler */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

Bu durumda, işlevimiz hiçbir parametre almaz ve hiçbir şey döndürmez. Sadece daha önce tanımladığımız ve `//export` yönergesi kullanılarak dışa aktarılan Go işlevini çağırır.

Son olarak, uzantının adını, sürümünü ve özelliklerini içeren `zend_module_entry` yapısında uzantının meta verilerini tanımlarız. Bu bilgiler, PHP'nin uzantımızı tanıması ve yüklemesi için gereklidir. `ext_functions`'ın tanımladığımız PHP işlevlerine işaretçiler dizisi olduğunu ve `extension_arginfo.h` dosyasında `gen_stub.php` betiği tarafından otomatik olarak oluşturulduğunu unutmayın.

Uzantı kaydı, Go kodumuzda çağırdığımız FrankenPHP'nin `RegisterExtension()` işlevi tarafından otomatik olarak ele alınır.

### Gelişmiş Kullanım

Artık Go'da temel bir PHP uzantısının nasıl oluşturulacağını bildiğimize göre, örneğimizi karmaşıklaştıralım. Şimdi bir dizeyi parametre olarak alan ve büyük harfli versiyonunu döndüren bir PHP işlevi oluşturacağız.

#### PHP İşlev Stub'ını Tanımla

Yeni PHP işlevini tanımlamak için `extension.stub.php` dosyamızı yeni işlev imzasını içerecek şekilde değiştireceğiz:

```php
<?php

/** @generate-class-entries */

/**
 * Bir dizeyi büyük harfe dönüştürür.
 *
 * @param string $string Dönüştürülecek dize.
 * @return string Dizinin büyük harfli versiyonu.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> İşlevlerinizin dokümantasyonunu ihmal etmeyin! Uzantı stub'larınızı diğer geliştiricilerle paylaşarak uzantınızın nasıl kullanılacağını ve hangi özelliklerin mevcut olduğunu belgeleyebilirsiniz.

Stub dosyasını `gen_stub.php` betiğiyle yeniden oluşturarak, `extension_arginfo.h` dosyası şöyle görünmelidir:

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

`go_upper` işlevinin `string` türünde bir parametre ve `string` türünde bir dönüş değeriyle tanımlandığını görebiliriz.

#### Go ve PHP/C Arasında Tür Dengeleme

Go işleviniz doğrudan bir PHP dizesini parametre olarak kabul edemez. Onu bir Go dizesine dönüştürmeniz gerekir. Neyse ki, FrankenPHP, oluşturucu yaklaşımında gördüğümüz gibi, PHP dizeleri ve Go dizeleri arasındaki dönüşümü yönetmek için yardımcı işlevler sağlar.

Başlık dosyası basit kalır:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Şimdi `extension.c` dosyamızda Go ve C arasındaki köprüyü yazabiliriz. PHP dizesini doğrudan Go işlevimize geçireceğiz:

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

`ZEND_PARSE_PARAMETERS_START` ve parametre ayrıştırması hakkında daha fazla bilgiyi [PHP Dahili Kitabı'nın](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters) ilgili sayfasında bulabilirsiniz. Burada, PHP'ye işlevimizin `zend_string` olarak bir zorunlu `string` parametresi aldığını söylüyoruz. Daha sonra bu dizeyi doğrudan Go işlevimize iletiyoruz ve `RETVAL_STR` kullanarak sonucu döndürüyoruz.

Yapılacak tek bir şey kaldı: `go_upper` işlevini Go'da uygulamak.

#### Go İşlevini Uygula

Go işlevimiz parametre olarak bir `*C.zend_string` alacak, FrankenPHP'nin yardımcı işlevini kullanarak onu bir Go dizesine dönüştürecek, işleyecek ve sonucu yeni bir `*C.zend_string` olarak döndürecektir. Yardımcı işlevler, tüm bellek yönetimi ve dönüştürme karmaşıklığını bizim için halleder.

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
FrankenPHP'nin yardımcı işlevleri, PHP'nin `zend_string` formatı ile Go dizeleri arasındaki dönüştürmeyi otomatik olarak halleder.
`PHPString()` içindeki `false` parametresi, yeni kalıcı olmayan bir dize oluşturmak istediğimizi belirtir (istek sonunda serbest bırakılır).

> [!TIP]
>
> Bu örnekte herhangi bir hata işleme yapmıyoruz, ancak Go işlevlerinizde kullanmadan önce işaretçilerin `nil` olmadığını ve verilerin geçerli olduğunu her zaman kontrol etmelisiniz.

### Uzantıyı FrankenPHP'ye Entegre Etme

Uzantımız artık derlenmeye ve FrankenPHP'ye entegre edilmeye hazır. Bunu yapmak için, FrankenPHP'yi nasıl derleyeceğinizi öğrenmek üzere FrankenPHP [derleme dokümantasyonuna](compile.md) bakın. Modülü `--with` bayrağını kullanarak ekleyin ve modülünüzün yolunu işaret edin:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

İşte bu kadar! Uzantınız artık FrankenPHP'ye entegre edilmiştir ve PHP kodunuzda kullanılabilir.

### Uzantınızı Test Etme

Uzantınızı FrankenPHP'ye entegre ettikten sonra, uyguladığınız işlevler için örnekler içeren bir `index.php` dosyası oluşturabilirsiniz:

```php
<?php

// Temel işlevi test et
go_print();

// Gelişmiş işlevi test et
echo go_upper("hello world") . "\n";
```

Şimdi FrankenPHP'yi bu dosya ile `./frankenphp php-server` komutunu kullanarak çalıştırabilir ve uzantınızın çalıştığını görmelisiniz.
