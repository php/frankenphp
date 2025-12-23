# Escribir Extensiones PHP en Go

Con FrankenPHP, puedes **escribir extensiones PHP en Go**, lo que te permite crear **funciones nativas de alto rendimiento** que pueden ser llamadas directamente desde PHP. Tus aplicaciones pueden aprovechar cualquier biblioteca Go existente o nueva, así como el famoso modelo de concurrencia de **goroutines directamente desde tu código PHP**.

Escribir extensiones PHP típicamente se hace en C, pero también es posible escribirlas en otros lenguajes con un poco de trabajo adicional. Las extensiones PHP te permiten aprovechar el poder de lenguajes de bajo nivel para extender las funcionalidades de PHP, por ejemplo, añadiendo funciones nativas o optimizando operaciones específicas.

Gracias a los módulos de Caddy, puedes escribir extensiones PHP en Go e integrarlas muy rápidamente en FrankenPHP.

## Dos Enfoques

FrankenPHP proporciona dos formas de crear extensiones PHP en Go:

1. **Usando el Generador de Extensiones** - El enfoque recomendado que genera todo el código repetitivo necesario para la mayoría de los casos de uso, permitiéndote enfocarte en escribir tu código Go.
2. **Implementación Manual** - Control total sobre la estructura de la extensión para casos de uso avanzados.

Comenzaremos con el enfoque del generador ya que es la forma más fácil de empezar, luego mostraremos la implementación manual para aquellos que necesitan un control completo.

## Usando el Generador de Extensiones

FrankenPHP incluye una herramienta que te permite **crear una extensión PHP** usando solo Go. **No necesitas escribir código C** ni usar CGO directamente: FrankenPHP también incluye una **API de tipos públicos** para ayudarte a escribir tus extensiones en Go sin tener que preocuparte por **la manipulación de tipos entre PHP/C y Go**.

> [!TIP]
> Si quieres entender cómo se pueden escribir extensiones en Go desde cero, puedes leer la sección de implementación manual a continuación que demuestra cómo escribir una extensión PHP en Go sin usar el generador.

Ten en cuenta que esta herramienta **no es un generador de extensiones completo**. Está diseñada para ayudarte a escribir extensiones simples en Go, pero no proporciona las características más avanzadas de las extensiones PHP. Si necesitas escribir una extensión más **compleja y optimizada**, es posible que necesites escribir algo de código C o usar CGO directamente.

### Requisitos Previos

Como se cubre en la sección de implementación manual a continuación, necesitas [obtener las fuentes de PHP](https://www.php.net/downloads.php) y crear un nuevo módulo Go.

#### Crear un Nuevo Módulo y Obtener las Fuentes de PHP

El primer paso para escribir una extensión PHP en Go es crear un nuevo módulo Go. Puedes usar el siguiente comando para esto:

```console
go mod init ejemplo.com/ejemplo
```

El segundo paso es [obtener las fuentes de PHP](https://www.php.net/downloads.php) para los siguientes pasos. Una vez que las tengas, descomprímelas en el directorio de tu elección, no dentro de tu módulo Go:

```console
tar xf php-*
```

### Escribiendo la Extensión

Todo está listo para escribir tu función nativa en Go. Crea un nuevo archivo llamado `stringext.go`. Nuestra primera función tomará una cadena como argumento, el número de veces para repetirla, un booleano para indicar si invertir la cadena, y devolverá la cadena resultante. Esto debería verse así:

```go
package ejemplo

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

Hay dos cosas importantes a tener en cuenta aquí:

- Un comentario de directiva `//export_php:function` define la firma de la función en PHP. Así es como el generador sabe cómo generar la función PHP con los parámetros y tipo de retorno correctos;
- La función debe devolver un `unsafe.Pointer`. FrankenPHP proporciona una API para ayudarte con la manipulación de tipos entre C y Go.

Mientras que el primer punto se explica por sí mismo, el segundo puede ser más difícil de entender. Profundicemos en la manipulación de tipos en la siguiente sección.

### Manipulación de Tipos

Aunque algunos tipos de variables tienen la misma representación en memoria entre C/PHP y Go, algunos tipos requieren más lógica para ser usados directamente. Esta es quizá la parte más difícil cuando se trata de escribir extensiones porque requiere entender los internos del motor Zend y cómo se almacenan las variables internamente en PHP.
Esta tabla resume lo que necesitas saber:

| Tipo PHP            | Tipo Go                        | Conversión directa | Helper de C a Go                     | Helper de Go a C                      | Soporte para Métodos de Clase |
|---------------------|--------------------------------|---------------------|---------------------------------------|----------------------------------------|-------------------------------|
| `int`               | `int64`                        | ✅                   | -                                     | -                                      | ✅                             |
| `?int`              | `*int64`                       | ✅                   | -                                     | -                                      | ✅                             |
| `float`             | `float64`                      | ✅                   | -                                     | -                                      | ✅                             |
| `?float`            | `*float64`                     | ✅                   | -                                     | -                                      | ✅                             |
| `bool`              | `bool`                         | ✅                   | -                                     | -                                      | ✅                             |
| `?bool`             | `*bool`                        | ✅                   | -                                     | -                                      | ✅                             |
| `string`/`?string` | `*C.zend_string`              | ❌                   | `frankenphp.GoString()`              | `frankenphp.PHPString()`              | ✅                             |
| `array`             | `frankenphp.AssociativeArray`  | ❌                   | `frankenphp.GoAssociativeArray()`    | `frankenphp.PHPAssociativeArray()`    | ✅                             |
| `array`             | `map[string]any`               | ❌                   | `frankenphp.GoMap()`                 | `frankenphp.PHPMap()`                 | ✅                             |
| `array`             | `[]any`                        | ❌                   | `frankenphp.GoPackedArray()`         | `frankenphp.PHPPackedArray()`         | ✅                             |
| `mixed`             | `any`                          | ❌                   | `GoValue()`                          | `PHPValue()`                          | ❌                             |
| `callable`          | `*C.zval`                      | ❌                   | -                                     | frankenphp.CallPHPCallable()          | ❌                             |
| `object`            | `struct`                       | ❌                   | _Aún no implementado_                | _Aún no implementado_                 | ❌                             |

> [!NOTE]
>
> Esta tabla aún no es exhaustiva y se completará a medida que la API de tipos de FrankenPHP se vuelva más completa.
>
> Para métodos de clase específicamente, los tipos primitivos y los arrays están actualmente soportados. Los objetos aún no pueden usarse como parámetros de métodos o tipos de retorno.

Si te refieres al fragmento de código de la sección anterior, puedes ver que se usan helpers para convertir el primer parámetro y el valor de retorno. El segundo y tercer parámetro de nuestra función `repeat_this()` no necesitan ser convertidos ya que la representación en memoria de los tipos subyacentes es la misma para C y Go.

#### Trabajando con Arrays

FrankenPHP proporciona soporte nativo para arrays PHP a través de `frankenphp.AssociativeArray` o conversión directa a un mapa o slice.

`AssociativeArray` representa un [mapa hash](https://es.wikipedia.org/wiki/Tabla_hash) compuesto por un campo `Map: map[string]any` y un campo opcional `Order: []string` (a diferencia de los "arrays asociativos" de PHP, los mapas de Go no están ordenados).

Si no se necesita orden o asociación, también es posible convertir directamente a un slice `[]any` o un mapa no ordenado `map[string]any`.

**Creando y manipulando arrays en Go:**

```go
package ejemplo

// #include <Zend/zend_types.h>
import "C"
import (
    "unsafe"

    "github.com/dunglas/frankenphp"
)

// export_php:function process_data_ordered(array $input): array
func process_data_ordered_map(arr *C.zend_array) unsafe.Pointer {
	// Convertir array asociativo PHP a Go manteniendo el orden
	associativeArray, err := frankenphp.GoAssociativeArray[any](unsafe.Pointer(arr))
    if err != nil {
        // manejar error
    }

	// iterar sobre las entradas en orden
	for _, key := range associativeArray.Order {
		value, _ = associativeArray.Map[key]
		// hacer algo con key y value
	}

	// devolver un array ordenado
	// si 'Order' no está vacío, solo se respetarán los pares clave-valor en 'Order'
	return frankenphp.PHPAssociativeArray[string](frankenphp.AssociativeArray[string]{
		Map: map[string]string{
			"clave1": "valor1",
			"clave2": "valor2",
		},
		Order: []string{"clave1", "clave2"},
	})
}

// export_php:function process_data_unordered(array $input): array
func process_data_unordered_map(arr *C.zend_array) unsafe.Pointer {
	// Convertir array asociativo PHP a un mapa Go sin mantener el orden
	// ignorar el orden será más eficiente
	goMap, err := frankenphp.GoMap[any](unsafe.Pointer(arr))
    if err != nil {
        // manejar error
    }

	// iterar sobre las entradas sin un orden específico
	for key, value := range goMap {
		// hacer algo con key y value
	}

	// devolver un array no ordenado
	return frankenphp.PHPMap(map[string]string {
		"clave1": "valor1",
		"clave2": "valor2",
	})
}

// export_php:function process_data_packed(array $input): array
func process_data_packed(arr *C.zend_array) unsafe.Pointer {
	// Convertir array empaquetado PHP a Go
	goSlice, err := frankenphp.GoPackedArray(unsafe.Pointer(arr))
    if err != nil {
        // manejar error
    }

	// iterar sobre el slice en orden
	for index, value := range goSlice {
		// hacer algo con index y value
	}

	// devolver un array empaquetado
	return frankenphp.PHPPackedArray([]string{"valor1", "valor2", "valor3"})
}
```

**Características clave de la conversión de arrays:**

- **Pares clave-valor ordenados** - Opción para mantener el orden del array asociativo
- **Optimizado para múltiples casos** - Opción para prescindir del orden para un mejor rendimiento o convertir directamente a un slice
- **Detección automática de listas** - Al convertir a PHP, detecta automáticamente si el array debe ser una lista empaquetada o un mapa hash
- **Arrays Anidados** - Los arrays pueden estar anidados y convertirán automáticamente todos los tipos soportados (`int64`, `float64`, `string`, `bool`, `nil`, `AssociativeArray`, `map[string]any`, `[]any`)
- **Objetos no soportados** - Actualmente, solo se pueden usar tipos escalares y arrays como valores. Proporcionar un objeto resultará en un valor `null` en el array PHP.

##### Métodos Disponibles: Empaquetados y Asociativos

- `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer` - Convertir a un array PHP ordenado con pares clave-valor
- `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` - Convertir un mapa a un array PHP no ordenado con pares clave-valor
- `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` - Convertir un slice a un array PHP empaquetado con solo valores indexados
- `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray` - Convertir un array PHP a un `AssociativeArray` de Go ordenado (mapa con orden)
- `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` - Convertir un array PHP a un mapa Go no ordenado
- `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` - Convertir un array PHP a un slice Go
- `frankenphp.IsPacked(zval *C.zend_array) bool` - Verificar si un array PHP está empaquetado (solo indexado) o es asociativo (pares clave-valor)

### Trabajando con Callables

FrankenPHP proporciona una forma de trabajar con callables de PHP usando el helper `frankenphp.CallPHPCallable`. Esto te permite llamar a funciones o métodos de PHP desde código Go.

Para mostrar esto, creemos nuestra propia función `array_map()` que toma un callable y un array, aplica el callable a cada elemento del array, y devuelve un nuevo array con los resultados:

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

Observa cómo usamos `frankenphp.CallPHPCallable()` para llamar al callable de PHP pasado como parámetro. Esta función toma un puntero al callable y un array de argumentos, y devuelve el resultado de la ejecución del callable. Puedes usar la sintaxis de callable a la que estás acostumbrado:

```php
<?php

$result = my_array_map([1, 2, 3], function($x) { return $x * 2; });
// $result será [2, 4, 6]

$result = my_array_map(['hola', 'mundo'], 'strtoupper');
// $result será ['HOLA', 'MUNDO']
```

### Declarando una Clase Nativa de PHP

El generador soporta la declaración de **clases opacas** como estructuras Go, que pueden usarse para crear objetos PHP. Puedes usar el comentario de directiva `//export_php:class` para definir una clase PHP. Por ejemplo:

```go
package ejemplo

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### ¿Qué son las Clases Opaque?

Las **clases opacas** son clases donde la estructura interna (propiedades) está oculta del código PHP. Esto significa:

- **Sin acceso directo a propiedades**: No puedes leer o escribir propiedades directamente desde PHP (`$user->name` no funcionará)
- **Interfaz solo de métodos** - Todas las interacciones deben pasar a través de los métodos que defines
- **Mejor encapsulación** - La estructura de datos interna está completamente controlada por el código Go
- **Seguridad de tipos** - Sin riesgo de que el código PHP corrompa el estado interno con tipos incorrectos
- **API más limpia** - Obliga a diseñar una interfaz pública adecuada

Este enfoque proporciona una mejor encapsulación y evita que el código PHP corrompa accidentalmente el estado interno de tus objetos Go. Todas las interacciones con el objeto deben pasar a través de los métodos que defines explícitamente.

#### Añadiendo Métodos a las Clases

Dado que las propiedades no son directamente accesibles, **debes definir métodos** para interactuar con tus clases opacas. Usa la directiva `//export_php:method` para definir el comportamiento:

```go
package ejemplo

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

#### Parámetros Nulos

El generador soporta parámetros nulos usando el prefijo `?` en las firmas de PHP. Cuando un parámetro es nulo, se convierte en un puntero en tu función Go, permitiéndote verificar si el valor era `null` en PHP:

```go
package ejemplo

// #include <Zend/zend_types.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // Verificar si se proporcionó name (no es null)
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }

    // Verificar si se proporcionó age (no es null)
    if age != nil {
        us.Age = int(*age)
    }

    // Verificar si se proporcionó active (no es null)
    if active != nil {
        us.Active = *active
    }
}
```

**Puntos clave sobre parámetros nulos:**

- **Tipos primitivos nulos** (`?int`, `?float`, `?bool`) se convierten en punteros (`*int64`, `*float64`, `*bool`) en Go
- **Strings nulos** (`?string`) permanecen como `*C.zend_string` pero pueden ser `nil`
- **Verificar `nil`** antes de desreferenciar valores de puntero
- **`null` de PHP se convierte en `nil` de Go** - cuando PHP pasa `null`, tu función Go recibe un puntero `nil`

> [!CAUTION]
>
> Actualmente, los métodos de clase tienen las siguientes limitaciones. **Los objetos no están soportados** como tipos de parámetro o tipos de retorno. **Los arrays están completamente soportados** para ambos parámetros y tipos de retorno. Tipos soportados: `string`, `int`, `float`, `bool`, `array`, y `void` (para tipo de retorno). **Los tipos de parámetros nulos están completamente soportados** para todos los tipos escalares (`?string`, `?int`, `?float`, `?bool`).

Después de generar la extensión, podrás usar la clase y sus métodos en PHP. Ten en cuenta que **no puedes acceder a las propiedades directamente**:

```php
<?php

$user = new User();

// ✅ Esto funciona - usando métodos
$user->setAge(25);
echo $user->getName();           // Salida: (vacío, valor por defecto)
echo $user->getAge();            // Salida: 25
$user->setNamePrefix("Empleado");

// ✅ Esto también funciona - parámetros nulos
$user->updateInfo("John", 30, true);        // Todos los parámetros proporcionados
$user->updateInfo("Jane", null, false);     // Age es null
$user->updateInfo(null, 25, null);          // Name y active son null

// ❌ Esto NO funcionará - acceso directo a propiedades
// echo $user->name;             // Error: No se puede acceder a la propiedad privada
// $user->age = 30;              // Error: No se puede acceder a la propiedad privada
```

Este diseño asegura que tu código Go tenga control completo sobre cómo se accede y modifica el estado del objeto, proporcionando una mejor encapsulación y seguridad de tipos.

### Declarando Constantes

El generador soporta exportar constantes Go a PHP usando dos directivas: `//export_php:const` para constantes globales y `//export_php:classconst` para constantes de clase. Esto te permite compartir valores de configuración, códigos de estado y otras constantes entre código Go y PHP.

#### Constantes Globales

Usa la directiva `//export_php:const` para crear constantes globales de PHP:

```go
package ejemplo

//export_php:const
const MAX_CONNECTIONS = 100

//export_php:const
const API_VERSION = "1.2.3"

//export_php:const
const STATUS_OK = iota

//export_php:const
const STATUS_ERROR = iota
```

#### Constantes de Clase

Usa la directiva `//export_php:classconst ClassName` para crear constantes que pertenecen a una clase PHP específica:

```go
package ejemplo

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

Las constantes de clase son accesibles usando el ámbito del nombre de clase en PHP:

```php
<?php

// Constantes globales
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// Constantes de clase
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

La directiva soporta varios tipos de valores incluyendo strings, enteros, booleanos, floats y constantes iota. Cuando se usa `iota`, el generador asigna automáticamente valores secuenciales (0, 1, 2, etc.). Las constantes globales se vuelven disponibles en tu código PHP como constantes globales, mientras que las constantes de clase tienen alcance a sus respectivas clases usando visibilidad pública. Cuando se usan enteros, se soportan diferentes notaciones posibles (binario, hexadecimal, octal) y se vuelcan tal cual en el archivo stub de PHP.

Puedes usar constantes tal como estás acostumbrado en el código Go. Por ejemplo, tomemos la función `repeat_this()` que declaramos anteriormente y cambiemos el último argumento a un entero:

```go
package ejemplo

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
		// invertir la cadena
	}

	if mode == STR_NORMAL {
		// no hacer nada, solo para mostrar la constante
	}

	return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
	// campos internos
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

### Usando Espacios de Nombres

El generador soporta organizar las funciones, clases y constantes de tu extensión PHP bajo un espacio de nombres usando la directiva `//export_php:namespace`. Esto ayuda a evitar conflictos de nombres y proporciona una mejor organización para la API de tu extensión.

#### Declarando un Espacio de Nombres

Usa la directiva `//export_php:namespace` al inicio de tu archivo Go para colocar todos los símbolos exportados bajo un espacio de nombres específico:

```go
//export_php:namespace Mi\Extensión
package ejemplo

import (
    "unsafe"

    "github.com/dunglas/frankenphp"
)

//export_php:function hello(): string
func hello() string {
    return "Hola desde el espacio de nombres Mi\\Extensión!"
}

//export_php:class User
type UserStruct struct {
    // campos internos
}

//export_php:method User::getName(): string
func (u *UserStruct) GetName() unsafe.Pointer {
    return frankenphp.PHPString("John Doe", false)
}

//export_php:const
const STATUS_ACTIVE = 1
```

#### Usando la Extensión con Espacio de Nombres en PHP

Cuando se declara un espacio de nombres, todas las funciones, clases y constantes se colocan bajo ese espacio de nombres en PHP:

```php
<?php

echo Mi\Extensión\hello(); // "Hola desde el espacio de nombres Mi\Extensión!"

$user = new Mi\Extensión\User();
echo $user->getName(); // "John Doe"

echo Mi\Extensión\STATUS_ACTIVE; // 1
```

#### Notas Importantes

- Solo se permite **una** directiva de espacio de nombres por archivo. Si se encuentran múltiples directivas de espacio de nombres, el generador devolverá un error.
- El espacio de nombres se aplica a **todos** los símbolos exportados en el archivo: funciones, clases, métodos y constantes.
- Los nombres de espacios de nombres siguen las convenciones de espacios de nombres de PHP usando barras invertidas (`\`) como separadores.
- Si no se declara un espacio de nombres, los símbolos se exportan al espacio de nombres global como de costumbre.

### Generando la Extensión

Aquí es donde ocurre la magia, y tu extensión ahora puede ser generada. Puedes ejecutar el generador con el siguiente comando:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init mi_extensión.go
```

> [!NOTE]
> No olvides establecer la variable de entorno `GEN_STUB_SCRIPT` a la ruta del archivo `gen_stub.php` en las fuentes de PHP que descargaste anteriormente. Este es el mismo script `gen_stub.php` mencionado en la sección de implementación manual.

Si todo salió bien, se debería haber creado un nuevo directorio llamado `build`. Este directorio contiene los archivos generados para tu extensión, incluyendo el archivo `mi_extensión.go` con los stubs de funciones PHP generadas.

### Integrando la Extensión Generada en FrankenPHP

Nuestra extensión ahora está lista para ser compilada e integrada en FrankenPHP. Para hacerlo, consulta la documentación de [compilación de FrankenPHP](compile.md) para aprender cómo compilar FrankenPHP. Agrega el módulo usando la bandera `--with`, apuntando a la ruta de tu módulo:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/mi-cuenta/mi-módulo/build
```

Ten en cuenta que apuntas al subdirectorio `/build` que se creó durante el paso de generación. Sin embargo, esto no es obligatorio: también puedes copiar los archivos generados a tu directorio de módulo y apuntar a él directamente.

### Probando tu Extensión Generada

Puedes crear un archivo PHP para probar las funciones y clases que has creado. Por ejemplo, crea un archivo `index.php` con el siguiente contenido:

```php
<?php

// Usando constantes globales
var_dump(repeat_this('Hola Mundo', 5, STR_REVERSE));

// Usando constantes de clase
$processor = new StringProcessor();
echo $processor->process('Hola Mundo', StringProcessor::MODE_LOWERCASE);  // "hola mundo"
echo $processor->process('Hola Mundo', StringProcessor::MODE_UPPERCASE);  // "HOLA MUNDO"
```

Una vez que hayas integrado tu extensión en FrankenPHP como se demostró en la sección anterior, puedes ejecutar este archivo de prueba usando `./frankenphp php-server`, y deberías ver tu extensión funcionando.

## Implementación Manual

Si quieres entender cómo funcionan las extensiones o necesitas un control total sobre tu extensión, puedes escribirlas manualmente. Este enfoque te da control completo pero requiere más código repetitivo.

### Función Básica

Veremos cómo escribir una extensión PHP simple en Go que define una nueva función nativa. Esta función será llamada desde PHP y desencadenará una goroutine que registra un mensaje en los logs de Caddy. Esta función no toma ningún parámetro y no devuelve nada.

#### Definir la Función Go

En tu módulo, necesitas definir una nueva función nativa que será llamada desde PHP. Para esto, crea un archivo con el nombre que desees, por ejemplo, `extension.go`, y agrega el siguiente código:

```go
package ejemplo

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
		slog.Info("¡Hola desde una goroutine!")
	}()
}
```

La función `frankenphp.RegisterExtension()` simplifica el proceso de registro de la extensión manejando la lógica interna de registro de PHP. La función `go_print_something` usa la directiva `//export` para indicar que será accesible en el código C que escribiremos, gracias a CGO.

En este ejemplo, nuestra nueva función desencadenará una goroutine que registra un mensaje en los logs de Caddy.

#### Definir la Función PHP

Para permitir que PHP llame a nuestra función, necesitamos definir una función PHP correspondiente. Para esto, crearemos un archivo stub, por ejemplo, `extension.stub.php`, que contendrá el siguiente código:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

Este archivo define la firma de la función `go_print()`, que será llamada desde PHP. La directiva `@generate-class-entries` permite a PHP generar automáticamente entradas de funciones para nuestra extensión.

Esto no se hace manualmente, sino usando un script proporcionado en las fuentes de PHP (asegúrate de ajustar la ruta al script `gen_stub.php` según dónde se encuentren tus fuentes de PHP):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Este script generará un archivo llamado `extension_arginfo.h` que contiene la información necesaria para que PHP sepa cómo definir y llamar a nuestra función.

#### Escribir el Puente entre Go y C

Ahora, necesitamos escribir el puente entre Go y C. Crea un archivo llamado `extension.h` en el directorio de tu módulo con el siguiente contenido:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

A continuación, crea un archivo llamado `extension.c` que realizará los siguientes pasos:

- Incluir los encabezados de PHP;
- Declarar nuestra nueva función nativa de PHP `go_print()`;
- Declarar los metadatos de la extensión.

Comencemos incluyendo los encabezados requeridos:

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Contiene símbolos exportados por Go
#include "_cgo_export.h"
```

Luego definimos nuestra función PHP como una función de lenguaje nativo:

```c
PHP_FUNCTION(go_print)
{
    ZEND_PARSE_PARAMETERS_NONE();

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* Funciones */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

En este caso, nuestra función no toma parámetros y no devuelve nada. Simplemente llama a la función Go que definimos anteriormente, exportada usando la directiva `//export`.

Finalmente, definimos los metadatos de la extensión en una estructura `zend_module_entry`, como su nombre, versión y propiedades. Esta información es necesaria para que PHP reconozca y cargue nuestra extensión. Ten en cuenta que `ext_functions` es un array de punteros a las funciones PHP que definimos, y fue generado automáticamente por el script `gen_stub.php` en el archivo `extension_arginfo.h`.

El registro de la extensión es manejado automáticamente por la función `RegisterExtension()` de FrankenPHP que llamamos en nuestro código Go.

### Uso Avanzado

Ahora que sabemos cómo crear una extensión PHP básica en Go, compliquemos nuestro ejemplo. Ahora crearemos una función PHP que tome una cadena como parámetro y devuelva su versión en mayúsculas.

#### Definir el Stub de la Función PHP

Para definir la nueva función PHP, modificaremos nuestro archivo `extension.stub.php` para incluir la nueva firma de la función:

```php
<?php

/** @generate-class-entries */

/**
 * Convierte una cadena a mayúsculas.
 *
 * @param string $string La cadena a convertir.
 * @return string La versión en mayúsculas de la cadena.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> ¡No descuides la documentación de tus funciones! Es probable que compartas tus stubs de extensión con otros desarrolladores para documentar cómo usar tu extensión y qué características están disponibles.

Al regenerar el archivo stub con el script `gen_stub.php`, el archivo `extension_arginfo.h` debería verse así:

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

Podemos ver que la función `go_upper` está definida con un parámetro de tipo `string` y un tipo de retorno `string`.

#### Manipulación de Tipos entre Go y PHP/C

Tu función Go no puede aceptar directamente una cadena PHP como parámetro. Necesitas convertirla a una cadena Go. Afortunadamente, FrankenPHP proporciona funciones helper para manejar la conversión entre cadenas PHP y cadenas Go, similar a lo que vimos en el enfoque del generador.

El archivo de encabezado sigue siendo simple:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Ahora podemos escribir el puente entre Go y C en nuestro archivo `extension.c`. Pasaremos la cadena PHP directamente a nuestra función Go:

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

Puedes aprender más sobre `ZEND_PARSE_PARAMETERS_START` y el análisis de parámetros en la página dedicada del [Libro de Internals de PHP](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters). Aquí, le decimos a PHP que nuestra función toma un parámetro obligatorio de tipo `string` como `zend_string`. Luego pasamos esta cadena directamente a nuestra función Go y devolvemos el resultado usando `RETVAL_STR`.

Solo queda una cosa por hacer: implementar la función `go_upper` en Go.

#### Implementar la Función Go

Nuestra función Go tomará un `*C.zend_string` como parámetro, lo convertirá a una cadena Go usando la función helper de FrankenPHP, lo procesará y devolverá el resultado como un nuevo `*C.zend_string`. Las funciones helper manejan toda la complejidad de gestión de memoria y conversión por nosotros.

```go
package ejemplo

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

Este enfoque es mucho más limpio y seguro que la gestión manual de memoria.
Las funciones helper de FrankenPHP manejan la conversión entre el formato `zend_string` de PHP y las cadenas Go automáticamente.
El parámetro `false` en `PHPString()` indica que queremos crear una nueva cadena no persistente (liberada al final de la solicitud).

> [!TIP]
>
> En este ejemplo, no realizamos ningún manejo de errores, pero siempre debes verificar que los punteros no sean `nil` y que los datos sean válidos antes de usarlos en tus funciones Go.

### Integrando la Extensión en FrankenPHP

Nuestra extensión ahora está lista para ser compilada e integrada en FrankenPHP. Para hacerlo, consulta la documentación de [compilación de FrankenPHP](compile.md) para aprender cómo compilar FrankenPHP. Agrega el módulo usando la bandera `--with`, apuntando a la ruta de tu módulo:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/mi-cuenta/mi-módulo
```

¡Eso es todo! Tu extensión ahora está integrada en FrankenPHP y puede ser usada en tu código PHP.

### Probando tu Extensión

Después de integrar tu extensión en FrankenPHP, puedes crear un archivo `index.php` con ejemplos para las funciones que has implementado:

```php
<?php

// Probar función básica
go_print();

// Probar función avanzada
echo go_upper("hola mundo") . "\n";
```

Ahora puedes ejecutar FrankenPHP con este archivo usando `./frankenphp php-server`, y deberías ver tu extensión funcionando.
