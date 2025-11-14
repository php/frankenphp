# Writing PHP Extensions in Go

With FrankenPHP, you can **write PHP extensions in Go**, which allows you to create **high-performance native functions** that can be called directly from PHP. Your applications can leverage any existing or new Go library, as well as the infamous concurrency model of **goroutines right from your PHP code**.

Writing PHP extensions is typically done in C, but it's also possible to write them in other languages with a bit of extra work. PHP extensions allow you to leverage the power of low-level languages to extend PHP's functionalities, for example, by adding native functions or optimizing specific operations.

Thanks to Caddy modules, you can write PHP extensions in Go and integrate them very quickly into FrankenPHP.

## Two Approaches

FrankenPHP provides two ways to create PHP extensions in Go:

1. **Using the Extension Generator** - The recommended approach that generates all necessary boilerplate for most use cases, allowing you to focus on writing your Go code
2. **Manual Implementation** - Full control over the extension structure for advanced use cases

We'll start with the generator approach as it's the easiest way to get started, then show the manual implementation for those who need complete control.

## Using the Extension Generator

FrankenPHP is bundled with a tool that allows you **to create a PHP extension** only using Go. **No need to write C code** or use CGO directly: FrankenPHP also includes a **public types API** to help you write your extensions in Go without having to worry about **the type juggling between PHP/C and Go**.

> [!TIP]
> If you want to understand how extensions can be written in Go from scratch, you can read the manual implementation section below demonstrating how to write a PHP extension in Go without using the generator.

Keep in mind that this tool is **not a full-fledged extension generator**. It is meant to help you write simple extensions in Go, but it does not provide the most advanced features of PHP extensions. If you need to write a more **complex and optimized** extension, you may need to write some C code or use CGO directly.

### Prerequisites

As covered in the manual implementation section below as well, you need to [get the PHP sources](https://www.php.net/downloads.php) and create a new Go module.

#### Create a New Module and Get PHP Sources

The first step to writing a PHP extension in Go is to create a new Go module. You can use the following command for this:

```console
go mod init example.com/example
```

The second step is to [get the PHP sources](https://www.php.net/downloads.php) for the next steps. Once you have them, decompress them into the directory of your choice, not inside your Go module:

```console
tar xf php-*
```

### Writing the Extension

Everything is now setup to write your native function in Go. Create a new file named `stringext.go`. Our first function will take a string as an argument, the number of times to repeat it, a boolean to indicate whether to reverse the string, and return the resulting string. This should look like this:

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

There are two important things to note here:

- A directive comment `//export_php:function` defines the function signature in PHP. This is how the generator knows how to generate the PHP function with the right parameters and return type;
- The function must return an `unsafe.Pointer`. FrankenPHP provides an API to help you with type juggling between C and Go.

While the first point speaks for itself, the second may be harder to apprehend. Let's take a deeper dive to type juggling in the next section.

### Type Juggling

While some variable types have the same memory representation between C/PHP and Go, some types require more logic to be directly used. This is maybe the hardest part when it comes to writing extensions because it requires understanding internals of the Zend Engine and how variables are stored internally in PHP.
This table summarizes what you need to know:

| PHP type           | Go type                       | Direct conversion | C to Go helper                    | Go to C helper                     | Class Methods Support |
|--------------------|-------------------------------|-------------------|-----------------------------------|------------------------------------|-----------------------|
| `int`              | `int64`                       | ✅                 | -                                 | -                                  | ✅                     |
| `?int`             | `*int64`                      | ✅                 | -                                 | -                                  | ✅                     |
| `float`            | `float64`                     | ✅                 | -                                 | -                                  | ✅                     |
| `?float`           | `*float64`                    | ✅                 | -                                 | -                                  | ✅                     |
| `bool`             | `bool`                        | ✅                 | -                                 | -                                  | ✅                     |
| `?bool`            | `*bool`                       | ✅                 | -                                 | -                                  | ✅                     |
| `string`/`?string` | `*C.zend_string`              | ❌                 | `frankenphp.GoString()`           | `frankenphp.PHPString()`           | ✅                     |
| `array`            | `frankenphp.AssociativeArray` | ❌                 | `frankenphp.GoAssociativeArray()` | `frankenphp.PHPAssociativeArray()` | ✅                     |
| `array`            | `map[string]any`              | ❌                 | `frankenphp.GoMap()`              | `frankenphp.PHPMap()`              | ✅                     |
| `array`            | `[]any`                       | ❌                 | `frankenphp.GoPackedArray()`      | `frankenphp.PHPPackedArray()`      | ✅                     |
| `mixed`            | `any`                         | ❌                 | `GoValue()`                       | `PHPValue()`                       | ❌                     |
| `object`           | `struct`                      | ❌                 | _Not yet implemented_             | _Not yet implemented_              | ❌                     |

> [!NOTE]
>
> This table is not exhaustive yet and will be completed as the FrankenPHP types API gets more complete.
>
> For class methods specifically, primitive types and arrays are currently supported. Objects cannot be used as method parameters or return types yet.

If you refer to the code snippet of the previous section, you can see that helpers are used to convert the first parameter and the return value. The second and third parameter of our `repeat_this()` function don't need to be converted as memory representation of the underlying types are the same for both C and Go.

#### Working with Arrays

FrankenPHP provides native support for PHP arrays through `frankenphp.AssociativeArray` or direct conversion to a map or slice.

`AssociativeArray` represents a [hash map](https://en.wikipedia.org/wiki/Hash_table) composed of a `Map: map[string]any`field and an optional `Order: []string` field (unlike PHP "associative arrays", Go maps aren't ordered).

If order or association are not needed, it's also possible to directly convert to a slice `[]any` or unordered map `map[string]any`.

**Creating and manipulating arrays in Go:**

```go
package example

// #include <Zend/zend_types.h>
import "C"
import (
    "unsafe"

    "github.com/dunglas/frankenphp"
)

// export_php:function process_data_ordered(array $input): array
func process_data_ordered_map(arr *C.zval) unsafe.Pointer {
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
func process_data_unordered_map(arr *C.zval) unsafe.Pointer {
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
func process_data_packed(arr *C.zval) unsafe.Pointer {
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

**Key features of array conversion:**

- **Ordered key-value pairs** - Option to keep the order of the associative array
- **Optimized for multiple cases** - Option to ditch the order for better performance or convert straight to a slice
- **Automatic list detection** - When converting to PHP, automatically detects if array should be a packed list or hashmap
- **Nested Arrays** - Arrays can be nested and will convert all support types automatically (`int64`,`float64`,`string`,`bool`,`nil`,`AssociativeArray`,`map[string]any`,`[]any`)
- **Objects are not supported** - Currently, only scalar types and arrays can be used as values. Providing an object will result in a `null` value in the PHP array.

##### Available methods: Packed and Associative

- `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer` - Convert to an ordered PHP array with key-value pairs
- `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` - Convert a map to an unordered PHP array with key-value pairs
- `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` - Convert a slice to a PHP packed array with indexed values only
- `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray` - Convert a PHP array to an ordered Go `AssociativeArray` (map with order)
- `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` - Convert a PHP array to an unordered Go map
- `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` - Convert a PHP array to a Go slice

### Declaring a Native PHP Class

The generator supports declaring **opaque classes** as Go structs, which can be used to create PHP objects. You can use the `//export_php:class` directive comment to define a PHP class. For example:

```go
package example

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### What are Opaque Classes?

**Opaque classes** are classes where the internal structure (properties) is hidden from PHP code. This means:

- **No direct property access**: You cannot read or write properties directly from PHP (`$user->name` won't work)
- **Method-only interface** - All interactions must go through methods you define
- **Better encapsulation** - Internal data structure is completely controlled by Go code
- **Type safety** - No risk of PHP code corrupting internal state with wrong types
- **Cleaner API** - Forces to design a proper public interface

This approach provides better encapsulation and prevents PHP code from accidentally corrupting the internal state of your Go objects. All interactions with the object must go through the methods you explicitly define.

#### Adding Methods to Classes

Since properties are not directly accessible, you **must define methods** to interact with your opaque classes. Use the `//export_php:method` directive to define behavior:

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

#### Nullable Parameters

The generator supports nullable parameters using the `?` prefix in PHP signatures. When a parameter is nullable, it becomes a pointer in your Go function, allowing you to check if the value was `null` in PHP:

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

**Key points about nullable parameters:**

- **Nullable primitive types** (`?int`, `?float`, `?bool`) become pointers (`*int64`, `*float64`, `*bool`) in Go
- **Nullable strings** (`?string`) remain as `*C.zend_string` but can be `nil`
- **Check for `nil`** before dereferencing pointer values
- **PHP `null` becomes Go `nil`** - when PHP passes `null`, your Go function receives a `nil` pointer

> [!WARNING]
>
> Currently, class methods have the following limitations. **Objects are not supported** as parameter types or return types. **Arrays are fully supported** for both parameters and return types. Supported types: `string`, `int`, `float`, `bool`, `array`, and `void` (for return type). **Nullable parameter types are fully supported** for all scalar types (`?string`, `?int`, `?float`, `?bool`).

After generating the extension, you will be allowed to use the class and its methods in PHP. Note that you **cannot access properties directly**:

```php
<?php

$user = new User();

// ✅ This works - using methods
$user->setAge(25);
echo $user->getName();           // Output: (empty, default value)
echo $user->getAge();            // Output: 25
$user->setNamePrefix("Employee");

// ✅ This also works - nullable parameters
$user->updateInfo("John", 30, true);        // All parameters provided
$user->updateInfo("Jane", null, false);     // Age is null
$user->updateInfo(null, 25, null);          // Name and active are null

// ❌ This will NOT work - direct property access
// echo $user->name;             // Error: Cannot access private property
// $user->age = 30;              // Error: Cannot access private property
```

This design ensures that your Go code has complete control over how the object's state is accessed and modified, providing better encapsulation and type safety.

### Declaring Constants

The generator supports exporting Go constants to PHP using two directives: `//export_php:const` for global constants and `//export_php:classconstant` for class constants. This allows you to share configuration values, status codes, and other constants between Go and PHP code.

#### Global Constants

Use the `//export_php:const` directive to create global PHP constants:

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

#### Class Constants

Use the `//export_php:classconstant ClassName` directive to create constants that belong to a specific PHP class:

```go
package example

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

Class constants are accessible using the class name scope in PHP:

```php
<?php

// Global constants
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// Class constants
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

The directive supports various value types including strings, integers, booleans, floats, and iota constants. When using `iota`, the generator automatically assigns sequential values (0, 1, 2, etc.). Global constants become available in your PHP code as global constants, while class constants are scoped to their respective classes using the public visibility. When using integers, different possible notation (binary, hex, octal) are supported and dumped as is in the PHP stub file.

You can use constants just like you are used to in the Go code. For example, let's take the `repeat_this()` function we declared earlier and change the last argument to an integer:

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

//export_php:classconstant StringProcessor
const MODE_LOWERCASE = 1

//export_php:classconstant StringProcessor
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

	return frankenphp.PHPString(result, false)
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

### Using Namespaces

The generator supports organizing your PHP extension's functions, classes, and constants under a namespace using the `//export_php:namespace` directive. This helps avoid naming conflicts and provides better organization for your extension's API.

#### Declaring a Namespace

Use the `//export_php:namespace` directive at the top of your Go file to place all exported symbols under a specific namespace:

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

#### Using Namespaced Extension in PHP

When a namespace is declared, all functions, classes, and constants are placed under that namespace in PHP:

```php
<?php

echo My\Extension\hello(); // "Hello from My\Extension namespace!"

$user = new My\Extension\User();
echo $user->getName(); // "John Doe"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### Important Notes

- Only **one** namespace directive is allowed per file. If multiple namespace directives are found, the generator will return an error.
- The namespace applies to **all** exported symbols in the file: functions, classes, methods, and constants.
- Namespace names follow PHP namespace conventions using backslashes (`\`) as separators.
- If no namespace is declared, symbols are exported to the global namespace as usual.

### Generating the Extension

This is where the magic happens, and your extension can now be generated. You can run the generator with the following command:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go
```

> [!NOTE]
> Don't forget to set the `GEN_STUB_SCRIPT` environment variable to the path of the `gen_stub.php` file in the PHP sources you downloaded earlier. This is the same `gen_stub.php` script mentioned in the manual implementation section.

If everything went well, a new directory named `build` should have been created. This directory contains the generated files for your extension, including the `my_extension.go` file with the generated PHP function stubs.

### Integrating the Generated Extension into FrankenPHP

Our extension is now ready to be compiled and integrated into FrankenPHP. To do this, refer to the FrankenPHP [compilation documentation](compile.md) to learn how to compile FrankenPHP. Add the module using the `--with` flag, pointing to the path of your module:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Note that you point to the `/build` subdirectory that was created during the generation step. However, this is not mandatory: you can also copy the generated files to your module directory and point to it directly.

### Testing Your Generated Extension

You can create a PHP file to test the functions and classes you've created. For example, create an `index.php` file with the following content:

```php
<?php

// Using global constants
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// Using class constants
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

Once you've integrated your extension into FrankenPHP as demonstrated in the previous section, you can run this test file using `./frankenphp php-server`, and you should see your extension working.

## Manual Implementation

If you want to understand how extensions work or need full control over your extension, you can write them manually. This approach gives you complete control but requires more boilerplate code.

### Basic Function

We'll see how to write a simple PHP extension in Go that defines a new native function. This function will be called from PHP and will trigger a goroutine that logs a message in Caddy's logs. This function doesn't take any parameters and returns nothing.

#### Define the Go Function

In your module, you need to define a new native function that will be called from PHP. To do this, create a file with the name you want, for example, `extension.go`, and add the following code:

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

The `frankenphp.RegisterExtension()` function simplifies the extension registration process by handling the internal PHP registration logic. The `go_print_something` function uses the `//export` directive to indicate that it will be accessible in the C code we will write, thanks to CGO.

In this example, our new function will trigger a goroutine that logs a message in Caddy's logs.

#### Define the PHP Function

To allow PHP to call our function, we need to define a corresponding PHP function. For this, we will create a stub file, for example, `extension.stub.php`, which will contain the following code:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

This file defines the signature of the `go_print()` function, which will be called from PHP. The `@generate-class-entries` directive allows PHP to automatically generate function entries for our extension.

This is not done manually but using a script provided in the PHP sources (make sure to adjust the path to the `gen_stub.php` script based on where your PHP sources are located):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

This script will generate a file named `extension_arginfo.h` that contains the necessary information for PHP to know how to define and call our function.

#### Write the Bridge Between Go and C

Now, we need to write the bridge between Go and C. Create a file named `extension.h` in your module directory with the following content:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Next, create a file named `extension.c` that will perform the following steps:

- Include PHP headers;
- Declare our new native PHP function `go_print()`;
- Declare the extension metadata.

Let's start by including the required headers:

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Contains symbols exported by Go
#include "_cgo_export.h"
```

We then define our PHP function as a native language function:

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

In this case, our function takes no parameters and returns nothing. It simply calls the Go function we defined earlier, exported using the `//export` directive.

Finally, we define the extension's metadata in a `zend_module_entry` structure, such as its name, version, and properties. This information is necessary for PHP to recognize and load our extension. Note that `ext_functions` is an array of pointers to the PHP functions we defined, and it was automatically generated by the `gen_stub.php` script in the `extension_arginfo.h` file.

The extension registration is automatically handled by FrankenPHP's `RegisterExtension()` function that we call in our Go code.

### Advanced Usage

Now that we know how to create a basic PHP extension in Go, let's complexify our example. We will now create a PHP function that takes a string as a parameter and returns its uppercase version.

#### Define the PHP Function Stub

To define the new PHP function, we will modify our `extension.stub.php` file to include the new function signature:

```php
<?php

/** @generate-class-entries */

/**
 * Converts a string to uppercase.
 *
 * @param string $string The string to convert.
 * @return string The uppercase version of the string.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> Don't neglect the documentation of your functions! You are likely to share your extension stubs with other developers to document how to use your extension and which features are available.

By regenerating the stub file with the `gen_stub.php` script, the `extension_arginfo.h` file should look like this:

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

We can see that the `go_upper` function is defined with a parameter of type `string` and a return type of `string`.

#### Type Juggling Between Go and PHP/C

Your Go function cannot directly accept a PHP string as a parameter. You need to convert it to a Go string. Fortunately, FrankenPHP provides helper functions to handle the conversion between PHP strings and Go strings, similar to what we saw in the generator approach.

The header file remains simple:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

We can now write the bridge between Go and C in our `extension.c` file. We will pass the PHP string directly to our Go function:

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

You can learn more about the `ZEND_PARSE_PARAMETERS_START` and parameters parsing in the dedicated page of [the PHP Internals Book](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters). Here, we tell PHP that our function takes one mandatory parameter of type `string` as a `zend_string`. We then pass this string directly to our Go function and return the result using `RETVAL_STR`.

There's only one thing left to do: implement the `go_upper` function in Go.

#### Implement the Go Function

Our Go function will take a `*C.zend_string` as a parameter, convert it to a Go string using FrankenPHP's helper function, process it, and return the result as a new `*C.zend_string`. The helper functions handle all the memory management and conversion complexity for us.

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

This approach is much cleaner and safer than manual memory management.
FrankenPHP's helper functions handle the conversion between PHP's `zend_string` format and Go strings automatically.
The `false` parameter in `PHPString()` indicates that we want to create a new non-persistent string (freed at the end of the request).

> [!TIP]
>
> In this example, we don't perform any error handling, but you should always check that pointers are not `nil` and that the data is valid before using it in your Go functions.

### Integrating the Extension into FrankenPHP

Our extension is now ready to be compiled and integrated into FrankenPHP. To do this, refer to the FrankenPHP [compilation documentation](compile.md) to learn how to compile FrankenPHP. Add the module using the `--with` flag, pointing to the path of your module:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

That's it! Your extension is now integrated into FrankenPHP and can be used in your PHP code.

### Testing Your Extension

After integrating your extension into FrankenPHP, you can create an `index.php` file with examples for the functions you've implemented:

```php
<?php

// Test basic function
go_print();

// Test advanced function
echo go_upper("hello world") . "\n";
```

You can now run FrankenPHP with this file using `./frankenphp php-server`, and you should see your extension working.
