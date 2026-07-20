# Написание PHP-расширений на Go

С FrankenPHP вы можете **писать PHP-расширения на Go**, что позволяет создавать **высокопроизводительные нативные функции**, которые могут быть вызваны непосредственно из PHP. Ваши приложения могут использовать любые существующие или новые библиотеки Go, а также знаменитую модель параллелизма **горутин прямо из вашего PHP-кода**.

Написание PHP-расширений обычно осуществляется на C, но их также возможно написать на других языках с небольшими дополнительными усилиями. PHP-расширения позволяют использовать мощь низкоуровневых языков для расширения функциональности PHP, например, путем добавления нативных функций или оптимизации специфических операций.

Благодаря модулям Caddy, вы можете писать PHP-расширения на Go и очень быстро интегрировать их в FrankenPHP.

## Два подхода

FrankenPHP предоставляет два способа создания PHP-расширений на Go:

1.  **Использование Генератора Расширений** — Рекомендуемый подход, который генерирует весь необходимый шаблонный код для большинства случаев использования, позволяя вам сосредоточиться на написании вашего Go-кода.
2.  **Ручная Реализация** — Полный контроль над структурой расширения для продвинутых случаев использования.

Мы начнем с подхода с генератором, так как это самый простой способ начать, а затем покажем ручную реализацию для тех, кому нужен полный контроль.

## Использование генератора расширений

FrankenPHP поставляется с инструментом, который позволяет **создавать PHP-расширение**, используя только Go. **Нет необходимости писать C-код** или напрямую использовать CGO: FrankenPHP также включает **публичный API типов**, чтобы помочь вам писать расширения на Go, не беспокоясь о **жонглировании типами между PHP/C и Go**.

> [!TIP]
> Если вы хотите понять, как расширения можно писать на Go с нуля, вы можете прочитать раздел о ручной реализации ниже, демонстрирующий, как написать PHP-расширение на Go без использования генератора.

Имейте в виду, что этот инструмент **не является полноценным генератором расширений**. Он предназначен для помощи в написании простых расширений на Go, но не предоставляет самых продвинутых функций PHP-расширений. Если вам нужно написать более **сложное и оптимизированное** расширение, возможно, вам придется написать некоторый C-код или напрямую использовать CGO.

### Предварительные условия

Как также описано в разделе о ручной реализации ниже, вам необходимо [получить исходный код PHP](https://www.php.net/downloads.php) и создать новый модуль Go.

#### Создайте новый модуль и получите исходный код PHP

Первый шаг к написанию PHP-расширения на Go — это создание нового модуля Go. Вы можете использовать следующую команду для этого:

```console
go mod init example.com/example
```

Второй шаг — [получить исходный код PHP](https://www.php.net/downloads.php) для следующих шагов. Как только вы их получите, распакуйте их в выбранную вами директорию, не внутрь вашего модуля Go:

```console
tar xf php-*
```

### Написание расширения

Теперь все настроено для написания вашей нативной функции на Go. Создайте новый файл с именем `stringext.go`. Наша первая функция будет принимать строку в качестве аргумента, количество раз для ее повторения, булево значение для указания, нужно ли переворачивать строку, и возвращать результирующую строку. Это должно выглядеть так:

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

Здесь есть две важные вещи, на которые стоит обратить внимание:

-   Комментарий-директива `//export_php:function` определяет сигнатуру функции в PHP. Таким образом генератор знает, как сгенерировать PHP-функцию с правильными параметрами и типом возвращаемого значения;
-   Функция должна возвращать `unsafe.Pointer`. FrankenPHP предоставляет API, который поможет вам с жонглированием типами между C и Go.

В то время как первый пункт говорит сам за себя, второй может быть сложнее для понимания. Давайте подробнее рассмотрим жонглирование типами далее в этом руководстве.

### Генерация расширения

Здесь происходит волшебство, и ваше расширение теперь может быть сгенерировано. Вы можете запустить генератор следующей командой:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go
```

> [!NOTE]
> Не забудьте установить переменную окружения `GEN_STUB_SCRIPT` на путь к файлу `gen_stub.php` в исходниках PHP, которые вы скачали ранее. Это тот же скрипт `gen_stub.php`, упомянутый в разделе ручной реализации.

Если все прошло хорошо, в вашей директории проекта должны содержаться следующие файлы для вашего расширения:

-   **`my_extension.go`** — Ваш исходный файл (остается без изменений)
-   **`my_extension_generated.go`** — Сгенерированный файл с обертками CGO, вызывающими ваши функции
-   **`my_extension.stub.php`** — PHP-файл-заглушка для автодополнения в IDE
-   **`my_extension_arginfo.h`** — Информация об аргументах PHP
-   **`my_extension.h`** — Заголовочный файл C
-   **`my_extension.c`** — Файл реализации C
-   **`README.md`** — Документация

> [!IMPORTANT]
> **Ваш исходный файл (`my_extension.go`) никогда не изменяется.** Генератор создает отдельный файл `_generated.go`, содержащий обертки CGO, которые вызывают ваши исходные функции. Это означает, что вы можете безопасно управлять версиями вашего исходного файла, не беспокоясь о том, что сгенерированный код загрязняет его.

### Интеграция сгенерированного расширения в FrankenPHP

Наше расширение теперь готово к компиляции и интеграции в FrankenPHP. Для этого обратитесь к [документации по компиляции](compile.md) FrankenPHP, чтобы узнать, как компилировать FrankenPHP. Добавьте модуль с помощью флага `--with`, указывая путь к вашему модулю:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Обратите внимание, что вы указываете на поддиректорию `/build`, которая была создана на этапе генерации. Однако это не является обязательным: вы также можете скопировать сгенерированные файлы в директорию вашего модуля и указать на нее напрямую.

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

После того как вы интегрировали ваше расширение в FrankenPHP, как показано в предыдущем разделе, вы можете запустить этот тестовый файл, используя `./frankenphp php-server`, и вы должны увидеть, как ваше расширение работает.

### Жонглирование типами

В то время как некоторые типы переменных имеют одно и то же представление в памяти между C/PHP и Go, некоторые типы требуют большей логики для прямого использования. Это, возможно, самая сложная часть при написании расширений, потому что она требует понимания внутренних механизмов Zend Engine и того, как переменные хранятся внутри PHP.
Эта таблица суммирует то, что вам нужно знать:

| PHP-тип            | Go-тип                        | Прямое преобразование | Помощник C в Go                   | Помощник Go в C                    | Поддержка методов класса |
| :----------------- | :---------------------------- | :------------------ | :-------------------------------- | :--------------------------------- | :----------------------- |
| `int`              | `int64`                       | ✅                  | -                                 | -                                  | ✅                       |
| `?int`             | `*int64`                      | ✅                  | -                                 | -                                  | ✅                       |
| `float`            | `float64`                     | ✅                  | -                                 | -                                  | ✅                       |
| `?float`           | `*float64`                    | ✅                  | -                                 | -                                  | ✅                       |
| `bool`             | `bool`                        | ✅                  | -                                 | -                                  | ✅                       |
| `?bool`            | `*bool`                       | ✅                  | -                                 | -                                  | ✅                       |
| `string`/`?string` | `*C.zend_string`              | ❌                  | `frankenphp.GoString()`           | `frankenphp.PHPString()`           | ✅                       |
| `array`            | `frankenphp.AssociativeArray` | ❌                  | `frankenphp.GoAssociativeArray()` | `frankenphp.PHPAssociativeArray()` | ✅                       |
| `array`            | `map[string]any`              | ❌                  | `frankenphp.GoMap()`              | `frankenphp.PHPMap()`              | ✅                       |
| `array`            | `[]any`                       | ❌                  | `frankenphp.GoPackedArray()`      | `frankenphp.PHPPackedArray()`      | ✅                       |
| `mixed`            | `any`                         | ❌                  | `GoValue()`                       | `PHPValue()`                       | ❌                       |
| `callable`         | `*C.zval`                     | ❌                  | -                                 | `frankenphp.CallPHPCallable()`     | ❌                       |
| `object`           | `struct`                      | ❌                  | _Пока не реализовано_             | _Пока не реализовано_              | ❌                       |

> [!NOTE]
>
> Эта таблица еще не исчерпывающая и будет пополняться по мере доработки API типов FrankenPHP.
>
> Для методов классов, в частности, в настоящее время поддерживаются примитивные типы и массивы. Объекты пока не могут использоваться в качестве параметров методов или возвращаемых типов.

Если вы обратитесь к фрагменту кода из предыдущего раздела, вы увидите, что для преобразования первого параметра и возвращаемого значения используются вспомогательные функции. Второй и третий параметры нашей функции `repeat_this()` не требуют преобразования, так как представление в памяти базовых типов одинаково как для C, так и для Go.

#### Работа с массивами

FrankenPHP предоставляет нативную поддержку PHP-массивов через `frankenphp.AssociativeArray` или прямое преобразование в карту или срез.

`AssociativeArray` представляет собой [хеш-таблицу](https://en.wikipedia.org/wiki/Hash_table), состоящую из поля `Map: map[string]any` и опционального поля `Order: []string` (в отличие от "ассоциативных массивов" PHP, карты Go не упорядочены).

Если порядок или ассоциативность не нужны, также возможно прямое преобразование в срез `[]any` или неупорядоченную карту `map[string]any`.

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
        // обработать ошибку
    }

	// перебираем записи по порядку
	for _, key := range associativeArray.Order {
		value, _ := associativeArray.Map[key]
		// делаем что-то с ключом и значением
	}

	// возвращаем упорядоченный массив
	// если 'Order' не пуст, будут соблюдены только пары ключ-значение, указанные в 'Order'
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
	// Преобразование ассоциативного массива PHP в карту Go без сохранения порядка
	// игнорирование порядка будет более производительным
	goMap, err := frankenphp.GoMap[any](unsafe.Pointer(arr))
    if err != nil {
        // обработать ошибку
    }

	// перебираем записи в произвольном порядке
	for key, value := range goMap {
		// делаем что-то с ключом и значением
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
        // обработать ошибку
    }

	// перебираем срез по порядку
	for index, value := range goSlice {
		// делаем что-то с индексом и значением
	}

	// возвращаем упакованный массив
	return frankenphp.PHPPackedArray([]string{"value1", "value2", "value3"})
}
```

**Основные особенности преобразования массивов:**

-   **Упорядоченные пары ключ-значение** — Возможность сохранять порядок ассоциативного массива
-   **Оптимизировано для различных случаев** — Возможность отказаться от порядка для лучшей производительности или преобразовать напрямую в срез
-   **Автоматическое определение списка** — При преобразовании в PHP автоматически определяет, должен ли массив быть упакованным списком или хеш-картой
-   **Вложенные массивы** — Массивы могут быть вложенными и будут автоматически преобразовывать все поддерживаемые типы (`int64`, `float64`, `string`, `bool`, `nil`, `AssociativeArray`, `map[string]any`, `[]any`)
-   **Объекты не поддерживаются** — В настоящее время в качестве значений могут использоваться только скалярные типы и массивы. Предоставление объекта приведет к значению `null` в PHP-массиве.

##### Доступные методы: packed и associative

-   `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer` — Преобразует в упорядоченный PHP-массив с парами ключ-значение
-   `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` — Преобразует карту в неупорядоченный PHP-массив с парами ключ-значение
-   `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` — Преобразует срез в упакованный PHP-массив только с индексированными значениями
-   `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray` — Преобразует PHP-массив в упорядоченный Go `AssociativeArray` (карта с порядком)
-   `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` — Преобразует PHP-массив в неупорядоченную Go-карту
-   `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` — Преобразует PHP-массив в Go-срез
-   `frankenphp.IsPacked(zval *C.zend_array) bool` — Проверяет, является ли PHP-массив упакованным (только индексированным) или ассоциативным (пары ключ-значение)

### Работа с вызываемыми объектами

FrankenPHP предоставляет способ работы с PHP-вызываемыми объектами с помощью вспомогательной функции `frankenphp.CallPHPCallable`. Это позволяет вызывать PHP-функции или методы из Go-кода.

Чтобы продемонстрировать это, давайте создадим нашу собственную функцию `array_map()`, которая принимает вызываемый объект и массив, применяет вызываемый объект к каждому элементу массива и возвращает новый массив с результатами:

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

Обратите внимание, как мы используем `frankenphp.CallPHPCallable()` для вызова PHP-вызываемого объекта, переданного в качестве параметра. Эта функция принимает указатель на вызываемый объект и массив аргументов и возвращает результат выполнения вызываемого объекта. Вы можете использовать привычный вам синтаксис вызываемого объекта:

```php
<?php

$result = my_array_map([1, 2, 3], function($x) { return $x * 2; });
// $result будет [2, 4, 6]

$result = my_array_map(['hello', 'world'], 'strtoupper');
// $result будет ['HELLO', 'WORLD']
```

### Объявление нативного PHP-класса

Генератор поддерживает объявление **непрозрачных классов** как Go-структур, которые могут использоваться для создания PHP-объектов. Вы можете использовать директиву `//export_php:class` для определения PHP-класса. Например:

```go
package example

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### Что такое непрозрачные классы?

**Непрозрачные классы** — это классы, внутренняя структура которых (свойства) скрыта от PHP-кода. Это означает:

-   **Нет прямого доступа к свойствам**: Вы не можете читать или записывать свойства напрямую из PHP (`$user->name` не будет работать).
-   **Интерфейс только через методы** — Все взаимодействия должны происходить через методы, которые вы определяете.
-   **Лучшая инкапсуляция** — Внутренняя структура данных полностью контролируется Go-кодом.
-   **Типовая безопасность** — Нет риска того, что PHP-код повредит внутреннее состояние неправильными типами.
-   **Более чистый API** — Принуждает к разработке правильного публичного интерфейса.

Этот подход обеспечивает лучшую инкапсуляцию и предотвращает случайное повреждение PHP-кодом внутреннего состояния ваших Go-объектов. Все взаимодействия с объектом должны происходить через явно определенные вами методы.

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

#### Нулевые параметры

Генератор поддерживает нулевые параметры с использованием префикса `?` в PHP-сигнатурах. Когда параметр является нулевым, он становится указателем в вашей Go-функции, что позволяет вам проверять, было ли значение `null` в PHP:

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
    // Проверяем, было ли предоставлено имя (не null)
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }

    // Проверяем, был ли предоставлен возраст (не null)
    if age != nil {
        us.Age = int(*age)
    }

    // Проверяем, был ли предоставлен статус активности (не null)
    if active != nil {
        us.Active = *active
    }
}
```

**Ключевые моменты о нулевых параметрах:**

-   **Нулевые примитивные типы** (`?int`, `?float`, `?bool`) становятся указателями (`*int64`, `*float64`, `*bool`) в Go.
-   **Нулевые строки** (`?string`) остаются `*C.zend_string`, но могут быть `nil`.
-   **Проверяйте на `nil`** перед разыменованием значений указателей.
-   **PHP `null` становится Go `nil`** — когда PHP передает `null`, ваша Go-функция получает `nil`-указатель.

> [!WARNING]
>
> В настоящее время методы классов имеют следующие ограничения. **Объекты не поддерживаются** в качестве типов параметров или возвращаемых типов. **Массивы полностью поддерживаются** как для параметров, так и для возвращаемых типов. Поддерживаемые типы: `string`, `int`, `float`, `bool`, `array` и `void` (для возвращаемого типа). **Нулевые типы параметров полностью поддерживаются** для всех скалярных типов (`?string`, `?int`, `?float`, `?bool`).

После генерации расширения вы сможете использовать класс и его методы в PHP. Обратите внимание, что вы **не можете получить доступ к свойствам напрямую**:

```php
<?php

$user = new User();

// ✅ Это работает - использование методов
$user->setAge(25);
echo $user->getName();           // Вывод: (пусто, значение по умолчанию)
echo $user->getAge();            // Вывод: 25
$user->setNamePrefix("Employee");

// ✅ Это также работает - нулевые параметры
$user->updateInfo("John", 30, true);        // Все параметры предоставлены
$user->updateInfo("Jane", null, false);     // Возраст null
$user->updateInfo(null, 25, null);          // Имя и активность null

// ❌ Это НЕ будет работать - прямой доступ к свойствам
// echo $user->name;             // Ошибка: Невозможно получить доступ к приватному свойству
// $user->age = 30;              // Ошибка: Невозможно получить доступ к приватному свойству
```

Такой дизайн гарантирует, что ваш Go-код полностью контролирует то, как состояние объекта доступно и изменяется, обеспечивая лучшую инкапсуляцию и типовую безопасность.

### Объявление констант

Генератор поддерживает экспорт Go-констант в PHP с использованием двух директив: `//export_php:const` для глобальных констант и `//export_php:classconst` для констант класса. Это позволяет обмениваться значениями конфигурации, кодами статуса и другими константами между Go- и PHP-кодом.

#### Глобальные константы

Используйте директиву `//export_php:const` для создания глобальных PHP-констант:

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
> PHP-константы будут принимать имя Go-константы, поэтому рекомендуется использовать заглавные буквы.

#### Константы класса

Используйте директиву `//export_php:classconst ClassName` для создания констант, принадлежащих конкретному PHP-классу:

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
> Как и глобальные константы, константы класса будут принимать имя Go-константы.

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

Директива поддерживает различные типы значений, включая строки, целые числа, булевы значения, числа с плавающей запятой и константы iota. При использовании `iota` генератор автоматически присваивает последовательные значения (0, 1, 2 и т.д.). Глобальные константы становятся доступными в вашем PHP-коде как глобальные константы, в то время как константы класса ограничены их соответствующими классами с использованием публичной видимости. При использовании целых чисел поддерживаются различные возможные нотации (двоичная, шестнадцатеричная, восьмеричная) и выводятся как есть в PHP-файле-заглушке.

Вы можете использовать константы так же, как вы привыкли в Go-коде. Например, давайте возьмем функцию `repeat_this()`, которую мы объявили ранее, и изменим последний аргумент на целое число:

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
		// переворачиваем строку
	}

	if mode == STR_NORMAL {
		// без операции, просто для демонстрации константы
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

Когда пространство имен объявлено, все функции, классы и константы размещаются под этим пространством имен в PHP:

```php
<?php

echo My\Extension\hello(); // "Hello from My\Extension namespace!"

$user = new My\Extension\User();
echo $user->getName(); // "John Doe"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### Важные заметки

-   Разрешена только **одна** директива пространства имен на файл. Если найдено несколько директив пространства имен, генератор вернет ошибку.
-   Пространство имен применяется ко **всем** экспортируемым символам в файле: функциям, классам, методам и константам.
-   Имена пространств имен соответствуют соглашениям PHP, используя обратные слеши (`\`) в качестве разделителей.
-   Если пространство имен не объявлено, символы экспортируются в глобальное пространство имен как обычно.

## Ручная реализация

Если вы хотите понять, как работают расширения, или вам нужен полный контроль над вашим расширением, вы можете написать их вручную. Этот подход дает вам полный контроль, но требует больше шаблонного кода.

### Базовая функция

Мы рассмотрим, как написать простое PHP-расширение на Go, которое определяет новую нативную функцию. Эта функция будет вызвана из PHP и запустит горутину, которая запишет сообщение в логи Caddy. Эта функция не принимает никаких параметров и ничего не возвращает.

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
		slog.Info("Hello from a goroutine!")
	}()
}
```

Функция `frankenphp.RegisterExtension()` упрощает процесс регистрации расширения, обрабатывая внутреннюю логику регистрации PHP. Функция `go_print_something` использует директиву `//export`, чтобы указать, что она будет доступна в коде C, который мы напишем, благодаря CGO.

В этом примере наша новая функция запустит горутину, которая запишет сообщение в логи Caddy.

#### Определение PHP-функции

Чтобы позволить PHP вызывать нашу функцию, нам нужно определить соответствующую PHP-функцию. Для этого мы создадим файл-заглушку, например, `extension.stub.php`, который будет содержать следующий код:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

Этот файл определяет сигнатуру функции `go_print()`, которая будет вызываться из PHP. Директива `@generate-class-entries` позволяет PHP автоматически генерировать записи функций для нашего расширения.

Это делается не вручную, а с помощью скрипта, предоставляемого в исходниках PHP (убедитесь, что путь к скрипту `gen_stub.php` соответствует месту расположения ваших исходников PHP):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Этот скрипт сгенерирует файл с именем `extension_arginfo.h`, который будет содержать необходимую информацию для PHP, чтобы знать, как определить и вызвать нашу функцию.

#### Написание моста между Go и C

Теперь нам нужно написать мост между Go и C. Создайте файл с именем `extension.h` в директории вашего модуля со следующим содержимым:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Затем создайте файл с именем `extension.c`, который выполнит следующие шаги:

-   Включит заголовки PHP;
-   Объявит нашу новую нативную PHP-функцию `go_print()`;
-   Объявит метаданные расширения.

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

Наконец, мы определяем метаданные расширения в структуре `zend_module_entry`, такие как его имя, версия и свойства. Эта информация необходима для того, чтобы PHP распознал и загрузил наше расширение. Обратите внимание, что `ext_functions` — это массив указателей на PHP-функции, которые мы определили, и он был автоматически сгенерирован скриптом `gen_stub.php` в файле `extension_arginfo.h`.

Регистрация расширения автоматически обрабатывается функцией `RegisterExtension()` FrankenPHP, которую мы вызываем в нашем Go-коде.

### Продвинутое использование

Теперь, когда мы знаем, как создать базовое PHP-расширение на Go, давайте усложним наш пример. Теперь мы создадим PHP-функцию, которая принимает строку в качестве параметра и возвращает ее версию в верхнем регистре.

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
> Не пренебрегайте документированием ваших функций! Вы, скорее всего, будете делиться заглушками ваших расширений с другими разработчиками, чтобы документировать, как использовать ваше расширение и какие функции доступны.

После повторной генерации файла заглушки с помощью скрипта `gen_stub.php` файл `extension_arginfo.h` должен выглядеть так:

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

#### Жонглирование типами между Go и PHP/C

Ваша Go-функция не может напрямую принимать PHP-строку в качестве параметра. Вам нужно преобразовать ее в Go-строку. К счастью, FrankenPHP предоставляет вспомогательные функции для обработки преобразования между PHP-строками и Go-строками, аналогично тому, что мы видели в подходе с генератором.

Заголовочный файл остается простым:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Теперь мы можем написать мост между Go и C в нашем файле `extension.c`. Мы будем передавать PHP-строку напрямую нашей Go-функции:

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

Вы можете узнать больше о `ZEND_PARSE_PARAMETERS_START` и разборе параметров на специальной странице [PHP Internals Book](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters). Здесь мы сообщаем PHP, что наша функция принимает один обязательный параметр типа `string` как `zend_string`. Затем мы передаем эту строку напрямую нашей Go-функции и возвращаем результат, используя `RETVAL_STR`.

Осталось сделать только одно: реализовать функцию `go_upper` в Go.

#### Реализация Go-функции

Наша Go-функция будет принимать `*C.zend_string` в качестве параметра, преобразовывать его в Go-строку с помощью вспомогательной функции FrankenPHP, обрабатывать ее и возвращать результат в виде новой `*C.zend_string`. Вспомогательные функции обрабатывают все сложности управления памятью и преобразования для нас.

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

Этот подход гораздо чище и безопаснее, чем ручное управление памятью.
Вспомогательные функции FrankenPHP автоматически обрабатывают преобразование между форматом `zend_string` PHP и строками Go.
Параметр `false` в `PHPString()` указывает, что мы хотим создать новую непостоянную строку (освобождаемую в конце запроса).

> [!TIP]
>
> В этом примере мы не выполняем никакой обработки ошибок, но вы всегда должны проверять, что указатели не `nil` и что данные действительны, прежде чем использовать их в ваших Go-функциях.

### Интеграция расширения в FrankenPHP

Наше расширение теперь готово к компиляции и интеграции в FrankenPHP. Для этого обратитесь к [документации по компиляции](compile.md) FrankenPHP, чтобы узнать, как компилировать FrankenPHP. Добавьте модуль с помощью флага `--with`, указывая путь к вашему модулю:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

Вот и все! Ваше расширение теперь интегрировано в FrankenPHP и может использоваться в вашем PHP-коде.

### Тестирование вашего расширения

После интеграции вашего расширения в FrankenPHP вы можете создать файл `index.php` с примерами функций, которые вы реализовали:

```php
<?php

// Тест базовой функции
go_print();

// Тест продвинутой функции
echo go_upper("hello world") . "\n";
```

Теперь вы можете запустить FrankenPHP с этим файлом, используя `./frankenphp php-server`, и вы должны увидеть, как ваше расширение работает.
