# 使用 Go 编写 PHP 扩展

使用 FrankenPHP，你可以**使用 Go 编写 PHP 扩展**，这允许你创建**高性能的原生函数**，可以直接从 PHP 调用。你的应用程序可以利用任何现有或新的 Go 库，以及直接从你的 PHP 代码中使用**协程（goroutines）的并发模型**。

编写 PHP 扩展通常使用 C 语言完成，但通过一些额外的工作，也可以使用其他语言编写。PHP 扩展允许你利用底层语言的强大功能来扩展 PHP 的功能，例如，通过添加原生函数或优化特定操作。

借助 Caddy 模块，你可以使用 Go 编写 PHP 扩展，并将其快速集成到 FrankenPHP 中。

## 两种方法

FrankenPHP 提供两种方式来创建 Go 语言的 PHP 扩展：

1. **使用扩展生成器** - 推荐的方法，为大多数用例生成所有必要的样板代码，让你专注于编写 Go 代码
2. **手动实现** - 对于高级用例，完全控制扩展结构

我们将从生成器方法开始，因为这是最简单的入门方式，然后为那些需要完全控制的人展示手动实现。

## 使用扩展生成器

FrankenPHP 捆绑了一个工具，允许你**仅使用 Go 创建 PHP 扩展**。**无需编写 C 代码**或直接使用 CGO：FrankenPHP 还包含一个**公共类型 API**，帮助你在 Go 中编写扩展，而无需担心**PHP/C 和 Go 之间的类型转换**。

> [!TIP]
> 如果你想了解如何从头开始在 Go 中编写扩展，可以阅读下面的手动实现部分，该部分演示了如何在不使用生成器的情况下在 Go 中编写 PHP 扩展。

请记住，此工具**不是功能齐全的扩展生成器**。它旨在帮助你在 Go 中编写简单的扩展，但它不提供 PHP 扩展的最高级功能。如果你需要编写更**复杂和优化**的扩展，你可能需要编写一些 C 代码或直接使用 CGO。

### 先决条件

正如下面的手动实现部分所涵盖的，你需要[获取 PHP 源代码](https://www.php.net/downloads.php)并创建一个新的 Go 模块。

#### 创建新模块并获取 PHP 源代码

在 Go 中编写 PHP 扩展的第一步是创建一个新的 Go 模块。你可以使用以下命令：

```console
go mod init github.com/my-account/my-module
```

第二步是为后续步骤[获取 PHP 源代码](https://www.php.net/downloads.php)。获取后，将它们解压到你选择的目录中，不要放在你的 Go 模块内：

```console
tar xf php-*
```

### 编写扩展

现在一切都设置好了，可以在 Go 中编写你的原生函数。创建一个名为 `stringext.go` 的新文件。我们的第一个函数将接受一个字符串作为参数，重复次数，一个布尔值来指示是否反转字符串，并返回结果字符串。这应该看起来像这样：

```go
import (
    "C"
    "github.com/dunglas/frankenphp"
    "strings"
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

这里有两个重要的事情要注意：

* 指令注释 `//export_php:function` 定义了 PHP 中的函数签名。这是生成器知道如何使用正确的参数和返回类型生成 PHP 函数的方式；
* 函数必须返回 `unsafe.Pointer`。FrankenPHP 提供了一个 API 来帮助你在 C 和 Go 之间进行类型转换。

虽然第一点不言自明，但第二点可能更难理解。让我们在下一节中深入了解类型转换。

### 类型转换

虽然一些变量类型在 C/PHP 和 Go 之间具有相同的内存表示，但某些类型需要更多逻辑才能直接使用。这可能是编写扩展时最困难的部分，因为它需要了解 Zend 引擎的内部结构以及变量在 PHP 中的内部存储方式。此表总结了你需要知道的内容：

| PHP 类型           | Go 类型             | 直接转换 | C 到 Go 助手          | Go 到 C 助手           | 类方法支持 |
|--------------------|---------------------|----------|----------------------|----------------------|------------|
| `int`              | `int64`             | ✅        | -                    | -                    | ✅          |
| `?int`             | `*int64`            | ✅        | -                    | -                    | ✅          |
| `float`            | `float64`           | ✅        | -                    | -                    | ✅          |
| `?float`           | `*float64`          | ✅        | -                    | -                    | ✅          |
| `bool`             | `bool`              | ✅        | -                    | -                    | ✅          |
| `?bool`            | `*bool`             | ✅        | -                    | -                    | ✅          |
| `string`/`?string` | `*C.zend_string`    | ❌        | frankenphp.GoString() | frankenphp.PHPString() | ✅          |
| `array`            | `*frankenphp.Array` | ❌        | frankenphp.GoArray()  | frankenphp.PHPArray()  | ✅          |
| `object`           | `struct`            | ❌        | _尚未实现_            | _尚未实现_            | ❌          |

> [!NOTE]
> 此表尚不详尽，将随着 FrankenPHP 类型 API 变得更加完整而完善。
>
> 特别是对于类方法，目前支持原始类型和数组。对象尚不能用作方法参数或返回类型。

如果你参考上一节的代码片段，你可以看到助手用于转换第一个参数和返回值。我们的 `repeat_this()` 函数的第二和第三个参数不需要转换，因为底层类型的内存表示对于 C 和 Go 都是相同的。

#### 处理数组

FrankenPHP 通过 `frankenphp.Array` 类型为 PHP 数组提供原生支持。此类型表示 PHP 索引数组（列表）和关联数组（哈希映射），具有有序的键值对。

**在 Go 中创建和操作数组：**

```go
//export_php:function process_data(array $input): array
func process_data(arr *C.zval) unsafe.Pointer {
    // 将 PHP 数组转换为 Go
    goArray := frankenphp.GoArray(unsafe.Pointer(arr))
	
	result := &frankenphp.Array{}
    
    result.SetInt(0, "first")
    result.SetInt(1, "second")
    result.Append("third") // 自动分配下一个整数键
    
    result.SetString("name", "John")
    result.SetString("age", int64(30))
    
    for i := uint32(0); i < goArray.Len(); i++ {
        key, value := goArray.At(i)
        if key.Type == frankenphp.PHPStringKey {
            result.SetString("processed_"+key.Str, value)
        } else {
            result.SetInt(key.Int+100, value)
        }
    }
    
    // 转换回 PHP 数组
    return frankenphp.PHPArray(result)
}
```

**`frankenphp.Array` 的关键特性：**

* **有序键值对** - 像 PHP 数组一样维护插入顺序
* **混合键类型** - 在同一数组中支持整数和字符串键
* **类型安全** - `PHPKey` 类型确保正确的键处理
* **自动列表检测** - 转换为 PHP 时，自动检测数组应该是打包列表还是哈希映射
* **不支持对象** - 目前，只有标量类型和数组可以用作值。提供对象将导致 PHP 数组中的 `null` 值。

**可用方法：**

* `SetInt(key int64, value interface{})` - 使用整数键设置值
* `SetString(key string, value interface{})` - 使用字符串键设置值  
* `Append(value interface{})` - 使用下一个可用整数键添加值
* `Len() uint32` - 获取元素数量
* `At(index uint32) (PHPKey, interface{})` - 获取索引处的键值对
* `frankenphp.PHPArray(arr *frankenphp.Array) unsafe.Pointer` - 转换为 PHP 数组

### 声明原生 PHP 类

生成器支持将 Go 结构体声明为**不透明类**，可用于创建 PHP 对象。你可以使用 `//export_php:class` 指令注释来定义 PHP 类。例如：

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### 什么是不透明类？

**不透明类**是内部结构（属性）对 PHP 代码隐藏的类。这意味着：

* **无直接属性访问**：你不能直接从 PHP 读取或写入属性（`$user->name` 不起作用）
* **仅方法接口** - 所有交互必须通过你定义的方法进行
* **更好的封装** - 内部数据结构完全由 Go 代码控制
* **类型安全** - 没有 PHP 代码使用错误类型破坏内部状态的风险
* **更清晰的 API** - 强制设计适当的公共接口

这种方法提供了更好的封装，并防止 PHP 代码意外破坏 Go 对象的内部状态。与对象的所有交互都必须通过你明确定义的方法进行。

#### 为类添加方法

由于属性不能直接访问，你**必须定义方法**来与不透明类交互。使用 `//export_php:method` 指令来定义行为：

```go
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

#### 可空参数

生成器支持在 PHP 签名中使用 `?` 前缀的可空参数。当参数可空时，它在你的 Go 函数中变成指针，允许你检查值在 PHP 中是否为 `null`：

```go
//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // 检查是否提供了 name（不为 null）
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }
    
    // 检查是否提供了 age（不为 null）
    if age != nil {
        us.Age = int(*age)
    }
    
    // 检查是否提供了 active（不为 null）
    if active != nil {
        us.Active = *active
    }
}
```

**关于可空参数的要点：**

* **可空原始类型**（`?int`、`?float`、`?bool`）在 Go 中变成指针（`*int64`、`*float64`、`*bool`）
* **可空字符串**（`?string`）仍然是 `*C.zend_string`，但可以是 `nil`
* **在解引用指针值之前检查 `nil`**
* **PHP `null` 变成 Go `nil`** - 当 PHP 传递 `null` 时，你的 Go 函数接收 `nil` 指针

> [!WARNING]
> 目前，类方法有以下限制。**不支持对象**作为参数类型或返回类型。**完全支持数组**作为参数和返回类型。支持的类型：`string`、`int`、`float`、`bool`、`array` 和 `void`（用于返回类型）。**完全支持可空参数类型**，适用于所有标量类型（`?string`、`?int`、`?float`、`?bool`）。

生成扩展后，你将被允许在 PHP 中使用类及其方法。请注意，你**不能直接访问属性**：

```php
<?php

$user = new User();

// ✅ 这可以工作 - 使用方法
$user->setAge(25);
echo $user->getName();           // 输出：（空，默认值）
echo $user->getAge();            // 输出：25
$user->setNamePrefix("Employee");

// ✅ 这也可以工作 - 可空参数
$user->updateInfo("John", 30, true);        // 提供所有参数
$user->updateInfo("Jane", null, false);     // Age 为 null
$user->updateInfo(null, 25, null);          // Name 和 active 为 null

// ❌ 这不会工作 - 直接属性访问
// echo $user->name;             // 错误：无法访问私有属性
// $user->age = 30;              // 错误：无法访问私有属性
```

这种设计确保你的 Go 代码完全控制如何访问和修改对象的状态，提供更好的封装和类型安全。

### 声明常量

生成器支持使用两个指令将 Go 常量导出到 PHP：`//export_php:const` 用于全局常量，`//export_php:classconstant` 用于类常量。这允许你在 Go 和 PHP 代码之间共享配置值、状态代码和其他常量。

#### 全局常量

使用 `//export_php:const` 指令创建全局 PHP 常量：

```go
//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const
const API_VERSION = "1.2.3"

//export_php:const
const STATUS_OK = iota

//export_php:const
const STATUS_ERROR = iota
```

#### 类常量

使用 `//export_php:classconstant ClassName` 指令创建属于特定 PHP 类的常量：

```go
//export_php:classconstant User
const STATUS_ACTIVE = 1

//export_php:classconstant User
const STATUS_INACTIVE = 0

//export_php:classconstant User
const ROLE_ADMIN = "admin"

//export_php:classconstant Order
const STATE_PENDING = iota

//export_php:classconstant Order
const STATE_PROCESSING = iota

//export_php:classconstant Order
const STATE_COMPLETED = iota
```

类常量在 PHP 中使用类名作用域访问：

```php
<?php

// 全局常量
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// 类常量
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

该指令支持各种值类型，包括字符串、整数、布尔值、浮点数和 iota 常量。使用 `iota` 时，生成器自动分配顺序值（0、1、2 等）。全局常量在你的 PHP 代码中作为全局常量可用，而类常量使用公共可见性限定在各自的类中。使用整数时，支持不同的可能记法（二进制、十六进制、八进制）并在 PHP 存根文件中按原样转储。

你可以像在 Go 代码中习惯的那样使用常量。例如，让我们采用我们之前声明的 `repeat_this()` 函数，并将最后一个参数更改为整数：

```go
import (
    "C"
    "github.com/dunglas/frankenphp"
    "strings"
)

//export_php:const
const STR_REVERSE = iota

//export_php:const
const STR_NORMAL = iota

//export_php:classconstant StringProcessor
const MODE_LOWERCASE = 1

//export_php:classconstant StringProcessor
const MODE_UPPERCASE = 2

//export_php:function repeat_this(string $str, int $count, int $mode): string
func repeat_this(s *C.zend_string, count int64, mode int) unsafe.Pointer {
    str := frankenphp.GoString(unsafe.Pointer(s))

    result := strings.Repeat(str, int(count))
    if mode == STR_REVERSE { 
        // 反转字符串
    }

    if mode == STR_NORMAL {
        // 无操作，只是为了展示常量
    }

    return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
    // 内部字段
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

### 使用命名空间

生成器支持使用 `//export_php:namespace` 指令将 PHP 扩展的函数、类和常量组织在命名空间下。这有助于避免命名冲突，并为扩展的 API 提供更好的组织。

#### 声明命名空间

在你的 Go 文件顶部使用 `//export_php:namespace` 指令，将所有导出的符号放在特定命名空间下：

```go
//export_php:namespace My\Extension
package main

import "C"

//export_php:function hello(): string
func hello() string {
    return "Hello from My\\Extension namespace!"
}

//export_php:class User
type UserStruct struct {
    // 内部字段
}

//export_php:method User::getName(): string
func (u *UserStruct) GetName() unsafe.Pointer {
    return frankenphp.PHPString("John Doe", false)
}

//export_php:const
const STATUS_ACTIVE = 1
```

#### 在 PHP 中使用命名空间扩展

当声明命名空间时，所有函数、类和常量都放在 PHP 中的该命名空间下：

```php
<?php

echo My\Extension\hello(); // "Hello from My\Extension namespace!"

$user = new My\Extension\User();
echo $user->getName(); // "John Doe"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### 重要说明

* 每个文件只允许**一个**命名空间指令。如果找到多个命名空间指令，生成器将返回错误。
* 命名空间适用于文件中的**所有**导出符号：函数、类、方法和常量。
* 命名空间名称遵循 PHP 命名空间约定，使用反斜杠（`\`）作为分隔符。
* 如果没有声明命名空间，符号将照常导出到全局命名空间。

### 生成扩展

这就是魔法发生的地方，现在可以生成你的扩展。你可以使用以下命令运行生成器：

```console
GEN_STUB_FILE=php-src/build/gen_stub.php frankenphp extension-init my_extension.go 
```

> [!NOTE]
> 不要忘记将 `GEN_STUB_FILE` 环境变量设置为你之前下载的 PHP 源代码中 `gen_stub.php` 文件的路径。这是在手动实现部分中提到的同一个 `gen_stub.php` 脚本。

如果一切顺利，应该创建了一个名为 `build` 的新目录。此目录包含扩展的生成文件，包括带有生成的 PHP 函数存根的 `my_extension.go` 文件。

### 将生成的扩展集成到 FrankenPHP 中

我们的扩展现在已准备好编译并集成到 FrankenPHP 中。为此，请参阅 FrankenPHP [编译文档](compile.md)以了解如何编译 FrankenPHP。使用 `--with` 标志添加模块，指向你的模块路径：

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

请注意，你指向在生成步骤中创建的 `/build` 子目录。但是，这不是强制性的：你也可以将生成的文件复制到你的模块目录并直接指向它。

### 测试你的生成扩展

你可以创建一个 PHP 文件来测试你创建的函数和类。例如，创建一个包含以下内容的 `index.php` 文件：

```php
<?php

// 使用全局常量
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// 使用类常量
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

一旦你按照上一节所示将扩展集成到 FrankenPHP 中，你就可以使用 `./frankenphp php-server` 运行此测试文件，你应该看到你的扩展正在工作。

## 手动实现

如果你想了解扩展的工作原理或需要完全控制你的扩展，你可以手动编写它们。这种方法给你完全的控制，但需要更多的样板代码。

### 基本函数

我们将看到如何在 Go 中编写一个简单的 PHP 扩展，定义一个新的原生函数。此函数将从 PHP 调用，并将触发一个在 Caddy 日志中记录消息的协程。此函数不接受任何参数并且不返回任何内容。

#### 定义 Go 函数

在你的模块中，你需要定义一个新的原生函数，该函数将从 PHP 调用。为此，创建一个你想要的名称的文件，例如 `extension.go`，并添加以下代码：

```go
package ext_go

//#include "extension.h"
import "C"
import (
    "unsafe"
    "github.com/caddyserver/caddy/v2"
    "github.com/dunglas/frankenphp"
)

func init() {
    frankenphp.RegisterExtension(unsafe.Pointer(&C.ext_module_entry))
}

//export go_print_something
func go_print_something() {
    go func() {
        caddy.Log().Info("Hello from a goroutine!")
    }()
}
```

`frankenphp.RegisterExtension()` 函数通过处理内部 PHP 注册逻辑简化了扩展注册过程。`go_print_something` 函数使用 `//export` 指令表示它将在我们将编写的 C 代码中可访问，这要归功于 CGO。

在此示例中，我们的新函数将触发一个在 Caddy 日志中记录消息的协程。

#### 定义 PHP 函数

为了允许 PHP 调用我们的函数，我们需要定义相应的 PHP 函数。为此，我们将创建一个存根文件，例如 `extension.stub.php`，其中包含以下代码：

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

此文件定义了 `go_print()` 函数的签名，该函数将从 PHP 调用。`@generate-class-entries` 指令允许 PHP 自动为我们的扩展生成函数条目。

这不是手动完成的，而是使用 PHP 源代码中提供的脚本（确保根据你的 PHP 源代码所在位置调整 `gen_stub.php` 脚本的路径）：

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

此脚本将生成一个名为 `extension_arginfo.h` 的文件，其中包含 PHP 知道如何定义和调用我们函数所需的信息。

#### 编写 Go 和 C 之间的桥梁

现在，我们需要编写 Go 和 C 之间的桥梁。在你的模块目录中创建一个名为 `extension.h` 的文件，内容如下：

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

接下来，创建一个名为 `extension.c` 的文件，该文件将执行以下步骤：

* 包含 PHP 头文件；
* 声明我们的新原生 PHP 函数 `go_print()`；
* 声明扩展元数据。

让我们首先包含所需的头文件：

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// 包含 Go 导出的符号
#include "_cgo_export.h"
```

然后我们将 PHP 函数定义为原生语言函数：

```c
PHP_FUNCTION(go_print)
{
    ZEND_PARSE_PARAMETERS_NONE();

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* Functions */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

在这种情况下，我们的函数不接受参数并且不返回任何内容。它只是调用我们之前定义的 Go 函数，使用 `//export` 指令导出。

最后，我们在 `zend_module_entry` 结构中定义扩展的元数据，例如其名称、版本和属性。这些信息对于 PHP 识别和加载我们的扩展是必需的。请注意，`ext_functions` 是指向我们定义的 PHP 函数的指针数组，它由 `gen_stub.php` 脚本在 `extension_arginfo.h` 文件中自动生成。

扩展注册由我们在 Go 代码中调用的 FrankenPHP 的 `RegisterExtension()` 函数自动处理。

### 高级用法

现在我们知道了如何在 Go 中创建基本的 PHP 扩展，让我们复杂化我们的示例。我们现在将创建一个 PHP 函数，该函数接受一个字符串作为参数并返回其大写版本。

#### 定义 PHP 函数存根

为了定义新的 PHP 函数，我们将修改我们的 `extension.stub.php` 文件以包含新的函数签名：

```php
<?php

/** @generate-class-entries */

/**
 * 将字符串转换为大写。
 *
 * @param string $string 要转换的字符串。
 * @return string 字符串的大写版本。
 */
function go_upper(string $string): string {}
```

> [!TIP]
> 不要忽视函数的文档！你可能会与其他开发人员共享扩展存根，以记录如何使用你的扩展以及哪些功能可用。

通过使用 `gen_stub.php` 脚本重新生成存根文件，`extension_arginfo.h` 文件应该如下所示：

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

我们可以看到 `go_upper` 函数定义了一个 `string` 类型的参数和一个 `string` 的返回类型。

#### Go 和 PHP/C 之间的类型转换

你的 Go 函数不能直接接受 PHP 字符串作为参数。你需要将其转换为 Go 字符串。幸运的是，FrankenPHP 提供了助手函数来处理 PHP 字符串和 Go 字符串之间的转换，类似于我们在生成器方法中看到的。

头文件保持简单：

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

我们现在可以在我们的 `extension.c` 文件中编写 Go 和 C 之间的桥梁。我们将 PHP 字符串直接传递给我们的 Go 函数：

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

你可以在 [PHP 内部手册](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters) 的专门页面中了解更多关于 `ZEND_PARSE_PARAMETERS_START` 和参数解析的信息。在这里，我们告诉 PHP 我们的函数接受一个 `string` 类型的强制参数作为 `zend_string`。然后我们将此字符串直接传递给我们的 Go 函数，并使用 `RETVAL_STR` 返回结果。

只剩下一件事要做：在 Go 中实现 `go_upper` 函数。

#### 实现 Go 函数

我们的 Go 函数将接受 `*C.zend_string` 作为参数，使用 FrankenPHP 的助手函数将其转换为 Go 字符串，处理它，并将结果作为新的 `*C.zend_string` 返回。助手函数为我们处理所有内存管理和转换复杂性。

```go
import "strings"

//export go_upper
func go_upper(s *C.zend_string) *C.zend_string {
    str := frankenphp.GoString(unsafe.Pointer(s))
    
    upper := strings.ToUpper(str)
    
    return (*C.zend_string)(frankenphp.PHPString(upper, false))
}
```

这种方法比手动内存管理更清洁、更安全。FrankenPHP 的助手函数自动处理 PHP 的 `zend_string` 格式和 Go 字符串之间的转换。`PHPString()` 中的 `false` 参数表示我们想要创建一个新的非持久字符串（在请求结束时释放）。

> [!TIP]
> 在此示例中，我们不执行任何错误处理，但你应该始终检查指针不是 `nil` 并且数据在 Go 函数中使用之前是有效的。

### 将扩展集成到 FrankenPHP 中

我们的扩展现在已准备好编译并集成到 FrankenPHP 中。为此，请参阅 FrankenPHP [编译文档](compile.md)以了解如何编译 FrankenPHP。使用 `--with` 标志添加模块，指向你的模块路径：

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

就是这样！你的扩展现在集成到 FrankenPHP 中，可以在你的 PHP 代码中使用。

### 测试你的扩展

将扩展集成到 FrankenPHP 后，你可以为你实现的函数创建一个包含示例的 `index.php` 文件：

```php
<?php

// 测试基本函数
go_print();

// 测试高级函数
echo go_upper("hello world") . "\n";
```

你现在可以使用 `./frankenphp php-server` 运行带有此文件的 FrankenPHP，你应该看到你的扩展正在工作。
