# Написание расширений PHP на Go

С FrankenPHP вы можете **писать расширения PHP на Go**, что позволяет создавать **высокопроизводительные нативные функции**, которые можно вызывать непосредственно из PHP. Ваши приложения могут использовать любые существующие или новые Go библиотеки, а также знаменитую модель параллелизма **горутин прямо из вашего PHP-кода**.

Написание расширений PHP обычно осуществляется на C, но их также можно писать на других языках с небольшими дополнительными усилиями. Расширения PHP позволяют использовать мощь низкоуровневых языков для расширения функциональности PHP, например, путем добавления нативных функций или оптимизации конкретных операций.

Благодаря модулям Caddy, вы можете писать расширения PHP на Go и очень быстро интегрировать их в FrankenPHP.

## Два подхода

FrankenPHP предоставляет два способа создания расширений PHP на Go:

1.  **Использование генератора расширений** – Рекомендуемый подход, который генерирует весь необходимый шаблонный код для большинства случаев использования, позволяя вам сосредоточиться на написании кода на Go.
2.  **Ручная реализация** – Полный контроль над структурой расширения для продвинутых случаев использования.

Мы начнем с подхода с генератором, так как это самый простой способ начать работу, а затем покажем ручную реализацию для тех, кому нужен полный контроль.

## Использование генератора расширений

FrankenPHP поставляется с инструментом, который позволяет **создавать расширения PHP**, используя только Go. **Не нужно писать код на C** или напрямую использовать CGO: FrankenPHP также включает **публичный API типов**, который поможет вам писать расширения на Go, не беспокоясь о **согласовании типов между PHP/C и Go**.

> [!TIP]
> Если вы хотите понять, как расширения могут быть написаны на Go с нуля, вы можете прочитать раздел "Ручная реализация" ниже, демонстрирующий, как написать расширение PHP на Go без использования генератора.

Имейте в виду, что этот инструмент **не является полноценным генератором расширений**. Он предназначен для помощи в написании простых расширений на Go, но не предоставляет самых продвинутых функций расширений PHP. Если вам нужно написать более **сложное и оптимизированное** расширение, вам может потребоваться написать некоторый код на C или напрямую использовать CGO.

### Предварительные требования

Как описано также в разделе "Ручная реализация" ниже, вам необходимо [получить исходники PHP](https://www.php.net/downloads.php) и создать новый модуль Go.

#### Создайте новый модуль и получите исходники PHP

Первым шагом к написанию расширения PHP на Go является создание нового модуля Go. Вы можете использовать следующую команду для этого:

```console
go mod init example.com/example
```

Вторым шагом является [получение исходников PHP](https://www.php.net/downloads.php) для следующих шагов. Как только вы их получите, распакуйте их в каталог по вашему выбору, но не внутрь вашего модуля Go:

```console
tar xf php-*
```

### Написание расширения

Теперь все настроено для написания вашей нативной функции на Go. Создайте новый файл с именем `stringext.go`. Наша первая функция будет принимать строку в качестве аргумента, количество раз для ее повторения, булево значение, указывающее, нужно ли инвертировать строку, и возвращать результирующую строку. Это должно выглядеть так:

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

Здесь следует отметить две важные вещи:

- Директива-комментарий `//export_php:function` определяет сигнатуру функции в PHP. Таким образом генератор знает, как создать PHP-функцию с правильными параметрами и типом возвращаемого значения;
- Функция должна возвращать `unsafe.Pointer`. FrankenPHP предоставляет API, чтобы помочь вам с согласованием типов между C и Go.

В то время как первый пункт говорит сам за себя, второй может быть сложнее для понимания. Давайте углубимся в согласование типов в следующем разделе.

### Согласование типов

В то время как некоторые типы переменных имеют одно и то же представление в памяти между C/PHP и Go, некоторые типы требуют большей логики для прямого использования. Это, возможно, самая сложная часть при написании расширений, поскольку она требует понимания внутренних механизмов движка Zend и того, как переменные хранятся внутри PHP.
Эта таблица суммирует то, что вам нужно знать:

| Тип PHP            | Тип Go                        | Прямое преобразование | Помощник C в Go                   | Помощник Go в C                    | Поддержка методов класса |
| :----------------- | :---------------------------- | :------------------ | :-------------------------------- | :--------------------------------- | :--------------------- |
| `int`              | `int64`                       | ✅                  | -                                 | -                                  | ✅                     |
| `?int`             | `*int64`                      | ✅                  | -                                 | -                                  | ✅                     |
| `float`            | `float64`                     | ✅                  | -                                 | -                                  | ✅                     |
| `?float`           | `*float64`                    | ✅                  | -                                 | -                                  | ✅                     |
| `bool`             | `bool`                        | ✅                  | -                                 | -                                  | ✅                     |
| `?bool`            | `*bool`                       | ✅                  | -                                 | -                                  | ✅                     |
| `string`/`?string` | `*C.zend_string`              | ❌                  | `frankenphp.GoString()`           | `frankenphp.PHPString()`           | ✅                     |
| `array`            | `frankenphp.AssociativeArray` | ❌                  | `frankenphp.GoAssociativeArray()` | `frankenphp.PHPAssociativeArray()` | ✅                     |
| `array`            | `map[string]any`              | ❌                  | `frankenphp.GoMap()`              | `frankenphp.PHPMap()`              | ✅                     |
| `array`            | `[]any`                       | ❌                  | `frankenphp.GoPackedArray()`      | `frankenphp.PHPPackedArray()`      | ✅                     |
| `mixed`            | `any`                         | ❌                  | `GoValue()`                       | `PHPValue()`                       | ❌                     |
| `callable`         | `*C.zval`                     | ❌                  | -                                 | frankenphp.CallPHPCallable()       | ❌                     |
| `object`           | `struct`                      | ❌                  | _Еще не реализовано_             | _Еще не реализовано_              | ❌                     |

> [!NOTE]
>
> Эта таблица еще не исчерпывающая и будет дополнена по мере того, как API типов FrankenPHP станет более полным.
>
> Для методов классов, в частности, в настоящее время поддерживаются примитивные типы и массивы. Объекты пока не могут использоваться в качестве параметров методов или возвращаемых типов.

Если вы обратитесь к фрагменту кода из предыдущего раздела, вы увидите, что для преобразования первого параметра и возвращаемого значения используются вспомогательные функции. Второй и третий параметры нашей функции `repeat_this()` не требуют преобразования, так как представление в памяти базовых типов одинаково как для C, так и для Go.

#### Работа с массивами

FrankenPHP обеспечивает нативную поддержку массивов PHP через `frankenphp.AssociativeArray` или прямое преобразование в карту (map) или срез (slice).

`AssociativeArray` представляет собой [хеш-таблицу](https://en.wikipedia.org/wiki/Hash_table), состоящую из поля `Map: map[string]any` и необязательного поля `Order: []string` (в отличие от "ассоциативных массивов" PHP, Go-карты не упорядочены).

Если порядок или ассоциация не требуются, также можно напрямую преобразовать в срез `[]any` или неупорядоченную карту `map[string]any`.

**Создание и манипулирование массивами в Go:**

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
	// Преобразование ассоциативного массива PHP в Go с сохранением порядка
	associativeArray, err := frankenphp.GoAssociativeArray[any](unsafe.Pointer(arr))
    if err != nil {
        // обработка ошибки
    }

	// итерация по элементам в порядке
	for _, key := range associativeArray.Order {
		value, _ = associativeArray.Map[key]
		// что-то делаем с ключом и значением
	}

	// возвращаем упорядоченный массив
	// если 'Order' не пуст, будут учитываться только пары ключ-значение, указанные в 'Order'
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
	// Преобразование ассоциативного массива PHP в Go-карту без сохранения порядка
	// игнорирование порядка будет более производительным
	goMap, err := frankenphp.GoMap[any](unsafe.Pointer(arr))
    if err != nil {
        // обработка ошибки
    }

	// итерация по элементам без определенного порядка
	for key, value := range goMap {
		// что-то делаем с ключом и значением
	}

	// возвращаем неупорядоченный массив
	return frankenphp.PHPMap(map[string]string {
		"key1": "value1",
		"key2": "value2",
	})
}

// export_php:function process_data_packed(array $input): array
func process_data_packed(arr *C.zend_array) unsafe.Pointer {
	// Преобразование упакованного массива PHP в Go
	goSlice, err := frankenphp.GoPackedArray(unsafe.Pointer(arr))
    if err != nil {
        // обработка ошибки
    }

	// итерация по срезу в порядке
	for index, value := range goSlice {
		// что-то делаем с индексом и значением
	}

	// возвращаем упакованный массив
	return frankenphp.PHPPackedArray([]string{"value1", "value2", "value3"})
}
```

**Ключевые особенности преобразования массивов:**

- **Упорядоченные пары ключ-значение** — Возможность сохранять порядок ассоциативного массива
- **Оптимизировано для различных случаев** — Возможность отказаться от порядка для лучшей производительности или преобразовать напрямую в срез
- **Автоматическое обнаружение списка** — При преобразовании в PHP автоматически определяет, должен ли массив быть упакованным списком или хеш-картой
- **Вложенные массивы** — Массивы могут быть вложенными и автоматически преобразуют все поддерживаемые типы (`int64`, `float64`, `string`, `bool`, `nil`, `AssociativeArray`, `map[string]any`, `[]any`)
- **Объекты не поддерживаются** — В настоящее время в качестве значений могут использоваться только скалярные типы и массивы. Предоставление объекта приведет к значению `null` в массиве PHP.

##### Доступные методы: Packed и Associative

- `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer` — Преобразует в упорядоченный массив PHP с парами ключ-значение
- `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` — Преобразует карту в неупорядоченный массив PHP с парами ключ-значение
- `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` — Преобразует срез в упакованный массив PHP только с индексированными значениями
- `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray` — Преобразует массив PHP в упорядоченный `AssociativeArray` Go (карту с порядком)
- `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` — Преобразует массив PHP в неупорядоченную карту Go
- `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` — Преобразует массив PHP в срез Go
- `frankenphp.IsPacked(zval *C.zend_array) bool` — Проверяет, является ли массив PHP упакованным (только индексированным) или ассоциативным (пары ключ-значение)

### Работа с вызываемыми объектами (Callables)

FrankenPHP предоставляет способ работы с вызываемыми объектами PHP с помощью вспомогательной функции `frankenphp.CallPHPCallable`. Это позволяет вызывать функции или методы PHP из кода Go.

Чтобы продемонстрировать это, давайте создадим нашу собственную функцию `array_map()`, которая принимает вызываемый объект (callable) и массив, применяет вызываемый объект к каждому элементу массива и возвращает новый массив с результатами:

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

Обратите внимание, как мы используем `frankenphp.CallPHPCallable()` для вызова PHP-вызываемого объекта, переданного в качестве параметра. Эта функция принимает указатель на вызываемый объект и массив аргументов, а затем возвращает результат выполнения вызываемого объекта. Вы можете использовать привычный синтаксис вызываемых объектов:

```php
<?php

$result = my_array_map([1, 2, 3], function($x) { return $x * 2; });
// $result будет [2, 4, 6]

$result = my_array_map(['hello', 'world'], 'strtoupper');
// $result будет ['HELLO', 'WORLD']
```

### Объявление нативного PHP-класса

Генератор поддерживает объявление **непрозрачных классов** как Go-структур, которые могут использоваться для создания PHP-объектов. Вы можете использовать директиву-комментарий `//export_php:class` для определения PHP-класса. Например:

```go
package example

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### Что такое непрозрачные классы?

**Непрозрачные классы** — это классы, внутренняя структура (свойства) которых скрыта от PHP-кода. Это означает:

- **Нет прямого доступа к свойствам**: Вы не можете читать или записывать свойства напрямую из PHP (`$user->name` не будет работать)
- **Только интерфейс методов** — Все взаимодействия должны происходить через методы, которые вы определяете
- **Лучшая инкапсуляция** — Внутренняя структура данных полностью контролируется кодом Go
- **Типобезопасность** — Нет риска, что PHP-код повредит внутреннее состояние неправильными типами
- **Более чистый API** — Принуждает к разработке правильного публичного интерфейса

Этот подход обеспечивает лучшую инкапсуляцию и предотвращает случайное повреждение PHP-кодом внутреннего состояния ваших Go-объектов. Все взаимодействия с объектом должны проходить через явно определенные вами методы.

#### Добавление методов к классам

Поскольку свойства недоступны напрямую, вы **должны определить методы** для взаимодействия с вашими непрозрачными классами. Используйте директиву `//export_php:method` для определения поведения:

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

#### Обнуляемые параметры

Генератор поддерживает обнуляемые параметры с использованием префикса `?` в сигнатурах PHP. Когда параметр является обнуляемым, он становится указателем в вашей Go-функции, что позволяет вам проверить, было ли значение `null` в PHP:

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
    // Проверить, было ли предоставлено имя (не null)
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }

    // Проверить, был ли предоставлен возраст (не null)
    if age != nil {
        us.Age = int(*age)
    }

    // Проверить, был ли предоставлен статус активности (не null)
    if active != nil {
        us.Active = *active
    }
}
```

**Ключевые моменты об обнуляемых параметрах:**

- **Обнуляемые примитивные типы** (`?int`, `?float`, `?bool`) становятся указателями (`*int64`, `*float64`, `*bool`) в Go
- **Обнуляемые строки** (`?string`) остаются `*C.zend_string`, но могут быть `nil`
- **Проверяйте на `nil`** перед разыменованием значений указателей
- **PHP `null` становится Go `nil`** — когда PHP передает `null`, ваша Go-функция получает `nil`-указатель

> [!WARNING]
>
> В настоящее время методы классов имеют следующие ограничения. **Объекты не поддерживаются** в качестве типов параметров или возвращаемых типов. **Массивы полностью поддерживаются** как для параметров, так и для возвращаемых типов. Поддерживаемые типы: `string`, `int`, `float`, `bool`, `array` и `void` (для возвращаемого типа). **Обнуляемые типы параметров полностью поддерживаются** для всех скалярных типов (`?string`, `?int`, `?float`, `?bool`).

После генерации расширения вы сможете использовать класс и его методы в PHP. Обратите внимание, что вы **не можете получать доступ к свойствам напрямую**:

```php
<?php

$user = new User();

// ✅ Это работает - использование методов
$user->setAge(25);
echo $user->getName();           // Вывод: (пусто, значение по умолчанию)
echo $user->getAge();            // Вывод: 25
$user->setNamePrefix("Employee");

// ✅ Это тоже работает - обнуляемые параметры
$user->updateInfo("John", 30, true);        // Все параметры предоставлены
$user->updateInfo("Jane", null, false);     // Возраст null
$user->updateInfo(null, 25, null);          // Имя и активность null

// ❌ Это НЕ будет работать - прямой доступ к свойствам
// echo $user->name;             // Ошибка: Невозможно получить доступ к приватному свойству
// $user->age = 30;              // Ошибка: Невозможно получить доступ к приватному свойству
```

Этот дизайн гарантирует, что ваш Go-код полностью контролирует то, как состояние объекта доступно и изменяется, обеспечивая лучшую инкапсуляцию и типобезопасность.

### Объявление констант

Генератор поддерживает экспорт Go-констант в PHP с использованием двух директив: `//export_php:const` для глобальных констант и `//export_php:classconst` для констант класса. Это позволяет вам обмениваться значениями конфигурации, кодами состояния и другими константами между Go- и PHP-кодом.

#### Глобальные константы

Используйте директиву `//export_php:const` для создания глобальных PHP-констант:

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

#### Константы класса

Используйте директиву `//export_php:classconst ClassName` для создания констант, принадлежащих определенному PHP-классу:

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

Константы класса доступны с использованием области видимости имени класса в PHP:

```php
<?php

// Глобальные константы
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// Константы класса
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

Директива поддерживает различные типы значений, включая строки, целые числа, булевы значения, числа с плавающей запятой и константы `iota`. При использовании `iota` генератор автоматически присваивает последовательные значения (0, 1, 2 и т.д.). Глобальные константы становятся доступными в вашем PHP-коде как глобальные константы, в то время как константы класса ограничены их соответствующими классами с использованием публичной видимости. При использовании целых чисел поддерживаются различные возможные нотации (двоичная, шестнадцатеричная, восьмеричная) и они записываются "как есть" в файл-заглушку PHP.

Вы можете использовать константы так же, как вы привыкли в коде Go. Например, возьмем функцию `repeat_this()`, которую мы объявили ранее, и изменим последний аргумент на целое число:

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
		// инвертировать строку
	}

	if mode == STR_NORMAL {
		// ничего не делать, просто для демонстрации константы
	}

	return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
	// внутренние поля
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

### Использование пространств имен

Генератор поддерживает организацию функций, классов и констант вашего PHP-расширения в пространстве имен с использованием директивы `//export_php:namespace`. Это помогает избежать конфликтов имен и обеспечивает лучшую организацию API вашего расширения.

#### Объявление пространства имен

Используйте директиву `//export_php:namespace` в начале вашего Go-файла, чтобы разместить все экспортируемые символы в определенном пространстве имен:

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
    // внутренние поля
}

//export_php:method User::getName(): string
func (u *UserStruct) GetName() unsafe.Pointer {
    return frankenphp.PHPString("John Doe", false)
}

//export_php:const
const STATUS_ACTIVE = 1
```

#### Использование расширения с пространством имен в PHP

Когда объявляется пространство имен, все функции, классы и константы помещаются в это пространство имен в PHP:

```php
<?php

echo My\Extension\hello(); // "Hello from My\Extension namespace!"

$user = new My\Extension\User();
echo $user->getName(); // "John Doe"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### Важные замечания

- Разрешена только **одна** директива пространства имен на файл. Если найдено несколько директив пространства имен, генератор вернет ошибку.
- Пространство имен применяется ко **всем** экспортируемым символам в файле: функциям, классам, методам и константам.
- Имена пространств имен следуют соглашениям PHP, используя обратные слеши (`\`) в качестве разделителей.
- Если пространство имен не объявлено, символы экспортируются в глобальное пространство имен как обычно.

### Генерация расширения

Здесь происходит волшебство, и ваше расширение теперь может быть сгенерировано. Вы можете запустить генератор следующей командой:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go
```

> [!NOTE]
> Не забудьте установить переменную окружения `GEN_STUB_SCRIPT` на путь к файлу `gen_stub.php` в исходниках PHP, которые вы скачали ранее. Это тот же скрипт `gen_stub.php`, упомянутый в разделе "Ручная реализация".

Если все прошло хорошо, должна быть создана новая директория с именем `build`. Эта директория содержит сгенерированные файлы для вашего расширения, включая файл `my_extension.go` с сгенерированными заглушками функций PHP.

### Интеграция сгенерированного расширения в FrankenPHP

Наше расширение готово к компиляции и интеграции в FrankenPHP. Для этого обратитесь к [документации по компиляции](compile.md) FrankenPHP, чтобы узнать, как скомпилировать FrankenPHP. Добавьте модуль, используя флаг `--with`, указывающий путь к вашему модулю:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Обратите внимание, что вы указываете на подкаталог `/build`, который был создан на этапе генерации. Однако это не является обязательным: вы также можете скопировать сгенерированные файлы в каталог вашего модуля и указать на него напрямую.

### Тестирование сгенерированного расширения

Вы можете создать PHP-файл для тестирования созданных вами функций и классов. Например, создайте файл `index.php` со следующим содержимым:

```php
<?php

// Использование глобальных констант
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// Использование констант класса
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

После того как вы интегрировали ваше расширение в FrankenPHP, как показано в предыдущем разделе, вы можете запустить этот тестовый файл, используя `./frankenphp php-server`, и вы должны увидеть работу вашего расширения.

## Ручная реализация

Если вы хотите понять, как работают расширения, или вам нужен полный контроль над вашим расширением, вы можете написать их вручную. Этот подход дает вам полный контроль, но требует больше шаблонного кода.

### Базовая функция

Мы рассмотрим, как написать простое расширение PHP на Go, которое определяет новую нативную функцию. Эта функция будет вызываться из PHP и будет запускать горутину, которая записывает сообщение в логи Caddy. Эта функция не принимает никаких параметров и ничего не возвращает.

#### Определение Go-функции

В вашем модуле вам нужно определить новую нативную функцию, которая будет вызываться из PHP. Для этого создайте файл с нужным вам именем, например, `extension.go`, и добавьте следующий код:

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
		slog.Info("Hello from a goroutine!") // Привет из горутины!
	}()
}
```

Функция `frankenphp.RegisterExtension()` упрощает процесс регистрации расширения, обрабатывая внутреннюю логику регистрации PHP. Функция `go_print_something` использует директиву `//export`, чтобы указать, что она будет доступна в C-коде, который мы напишем, благодаря CGO.

В этом примере наша новая функция будет запускать горутину, которая записывает сообщение в логи Caddy.

#### Определение PHP-функции

Чтобы позволить PHP вызывать нашу функцию, нам нужно определить соответствующую PHP-функцию. Для этого мы создадим файл-заглушку, например, `extension.stub.php`, который будет содержать следующий код:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

Этот файл определяет сигнатуру функции `go_print()`, которая будет вызываться из PHP. Директива `@generate-class-entries` позволяет PHP автоматически генерировать записи функций для нашего расширения.

Это делается не вручную, а с помощью скрипта, предоставляемого в исходниках PHP (убедитесь, что вы скорректировали путь к скрипту `gen_stub.php` в соответствии с тем, где расположены ваши исходники PHP):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Этот скрипт сгенерирует файл с именем `extension_arginfo.h`, который содержит необходимую информацию для PHP, чтобы знать, как определять и вызывать нашу функцию.

#### Написание моста между Go и C

Теперь нам нужно написать мост между Go и C. Создайте файл с именем `extension.h` в каталоге вашего модуля со следующим содержимым:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Затем создайте файл с именем `extension.c`, который будет выполнять следующие шаги:

- Включать заголовки PHP;
- Объявлять нашу новую нативную PHP-функцию `go_print()`;
- Объявлять метаданные расширения.

Начнем с включения необходимых заголовков:

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Содержит символы, экспортируемые Go
#include "_cgo_export.h"
```

Затем мы определяем нашу PHP-функцию как нативную языковую функцию:

```c
PHP_FUNCTION(go_print)
{
    ZEND_PARSE_PARAMETERS_NONE();

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* Функции */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

В этом случае наша функция не принимает параметров и ничего не возвращает. Она просто вызывает Go-функцию, которую мы определили ранее, экспортированную с помощью директивы `//export`.

Наконец, мы определяем метаданные расширения в структуре `zend_module_entry`, такие как его имя, версия и свойства. Эта информация необходима PHP для распознавания и загрузки нашего расширения. Обратите внимание, что `ext_functions` — это массив указателей на определенные нами PHP-функции, и он был автоматически сгенерирован скриптом `gen_stub.php` в файле `extension_arginfo.h`.

Регистрация расширения автоматически обрабатывается функцией `RegisterExtension()` FrankenPHP, которую мы вызываем в нашем Go-коде.

### Продвинутое использование

Теперь, когда мы знаем, как создать базовое расширение PHP на Go, давайте усложним наш пример. Теперь мы создадим PHP-функцию, которая принимает строку в качестве параметра и возвращает ее версию в верхнем регистре.

#### Определение заглушки PHP-функции

Чтобы определить новую PHP-функцию, мы изменим наш файл `extension.stub.php`, чтобы включить новую сигнатуру функции:

```php
<?php

/** @generate-class-entries */

/**
 * Преобразует строку в верхний регистр.
 *
 * @param string $string Строка для преобразования.
 * @return string Версия строки в верхнем регистре.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> Не пренебрегайте документацией ваших функций! Вы, вероятно, будете делиться заглушками ваших расширений с другими разработчиками, чтобы документировать, как использовать ваше расширение и какие функции доступны.

После повторной генерации файла-заглушки с помощью скрипта `gen_stub.php` файл `extension_arginfo.h` должен выглядеть следующим образом:

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

Мы видим, что функция `go_upper` определена с параметром типа `string` и возвращаемым типом `string`.

#### Согласование типов между Go и PHP/C

Ваша Go-функция не может напрямую принимать PHP-строку в качестве параметра. Вам нужно преобразовать ее в Go-строку. К счастью, FrankenPHP предоставляет вспомогательные функции для обработки преобразования между PHP-строками и Go-строками, аналогично тому, что мы видели в подходе с генератором.

Заголовочный файл остается простым:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Теперь мы можем написать мост между Go и C в нашем файле `extension.c`. Мы передадим PHP-строку непосредственно нашей Go-функции:

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

Вы можете узнать больше о `ZEND_PARSE_PARAMETERS_START` и разборе параметров на специальной странице [книги PHP Internals Book](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters). Здесь мы сообщаем PHP, что наша функция принимает один обязательный параметр типа `string` как `zend_string`. Затем мы передаем эту строку напрямую нашей Go-функции и возвращаем результат, используя `RETVAL_STR`.

Осталось сделать только одно: реализовать функцию `go_upper` в Go.

#### Реализация Go-функции

Наша Go-функция будет принимать `*C.zend_string` в качестве параметра, преобразовывать ее в Go-строку с помощью вспомогательной функции FrankenPHP, обрабатывать ее и возвращать результат в виде нового `*C.zend_string`. Вспомогательные функции обрабатывают все сложности управления памятью и преобразования за нас.

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

Этот подход намного чище и безопаснее, чем ручное управление памятью.
Вспомогательные функции FrankenPHP автоматически обрабатывают преобразование между форматом `zend_string` PHP и строками Go.
Параметр `false` в `PHPString()` указывает, что мы хотим создать новую непостоянную строку (освобождаемую в конце запроса).

> [!TIP]
>
> В этом примере мы не выполняем никакой обработки ошибок, но вы всегда должны проверять, что указатели не `nil` и что данные действительны, прежде чем использовать их в ваших Go-функциях.

### Интеграция расширения в FrankenPHP

Наше расширение готово к компиляции и интеграции в FrankenPHP. Для этого обратитесь к [документации по компиляции](compile.md) FrankenPHP, чтобы узнать, как скомпилировать FrankenPHP. Добавьте модуль, используя флаг `--with`, указывающий путь к вашему модулю:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

Вот и все! Ваше расширение теперь интегрировано в FrankenPHP и может быть использовано в вашем PHP-коде.

### Тестирование вашего расширения

После интеграции вашего расширения в FrankenPHP вы можете создать файл `index.php` с примерами реализованных вами функций:

```php
<?php

// Тестирование базовой функции
go_print();

// Тестирование продвинутой функции
echo go_upper("hello world") . "\n";
```

Теперь вы можете запустить FrankenPHP с этим файлом, используя `./frankenphp php-server`, и вы должны увидеть работу вашего расширения.
