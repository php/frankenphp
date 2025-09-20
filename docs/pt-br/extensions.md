# Escrevendo extensões PHP em Go

Com o FrankenPHP, você pode **escrever extensões PHP em Go**, o que permite
criar **funções nativas de alto desempenho** que podem ser chamadas diretamente
do PHP.
Suas aplicações podem aproveitar qualquer biblioteca Go existente ou nova, bem
como o famoso modelo de concorrência de **goroutines diretamente do seu código
PHP**.

Escrever extensões PHP normalmente é feito em C, mas também é possível
escrevê-las em outras linguagens com um pouco de trabalho extra.
As extensões PHP permitem que você aproveite o poder das linguagens de baixo
nível para estender as funcionalidades do PHP, por exemplo, adicionando funções
nativas ou otimizando operações específicas.

Graças aos módulos Caddy, você pode escrever extensões PHP em Go e integrá-las
rapidamente ao FrankenPHP.

## Duas abordagens

O FrankenPHP oferece duas maneiras de criar extensões PHP em Go:

1. **Usando o gerador de extensões** - A abordagem recomendada que gera todo o
   código boilerplate necessário para a maioria dos casos de uso, permitindo que
   você se concentre em escrever seu código em Go.
2. **Implementação manual** - Controle total sobre a estrutura da extensão para
   casos de uso avançados.

Começaremos com a abordagem do gerador, pois é a maneira mais fácil de começar,
e, em seguida, mostraremos a implementação manual para aqueles que precisam de
controle total.

## Usando o gerador de extensões

O FrankenPHP vem com uma ferramenta que permite **criar uma extensão PHP**
usando apenas Go.
**Não é necessário escrever código C** ou usar CGO diretamente: o FrankenPHP
também inclui uma **API de tipos pública** para ajudar você a escrever suas
extensões em Go sem ter que se preocupar com **o malabarismo de tipos entre
PHP/C e Go**.

> [!TIP]
> Se quiser entender como as extensões podem ser escritas em Go do zero, leia a
> seção de implementação manual abaixo, que demonstra como escrever uma extensão
> PHP em Go sem usar o gerador.

Lembre-se de que esta ferramenta **não é um gerador de extensões completo**.
Ela foi criada para ajudar você a escrever extensões simples em Go, mas não
oferece os recursos mais avançados das extensões PHP.
Se precisar escrever uma extensão mais **complexa e otimizada**, talvez seja
necessário escrever algum código em C ou usar CGO diretamente.

### Pré-requisitos

Conforme abordado na seção de implementação manual abaixo, você precisa
[obter o código-fonte do PHP](https://www.php.net/downloads.php) e criar um novo
módulo Go.

#### Criando um novo módulo e obtendo o código-fonte do PHP

O primeiro passo para escrever uma extensão PHP em Go é criar um novo módulo Go.
Você pode usar o seguinte comando para isso:

```console
go mod init github.com/<minha-conta>/<meu-modulo>
```

O segundo passo é
[obter o código-fonte do PHP](https://www.php.net/downloads.php) para os
próximos passos.
Depois de obtê-los, descompacte-os no diretório de sua escolha, não dentro do
seu módulo Go:

```console
tar xf php-*
```

### Escrevendo a extensão

Agora tudo está configurado para escrever sua função nativa em Go.
Crie um novo arquivo chamado `stringext.go`.
Nossa primeira função receberá uma string como argumento, o número de vezes que
ela será repetida, um booleano para indicar se a string deve ser invertida e
retornará a string resultante.
Deve ficar assim:

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

Há duas coisas importantes a serem observadas aqui:

- Um comentário de diretiva `//export_php:function` define a assinatura da
  função no PHP.
  É assim que o gerador sabe como gerar a função PHP com os parâmetros e o tipo
  de retorno corretos;
- A função deve retornar um `unsafe.Pointer`.
  O FrankenPHP fornece uma API para ajudar você com o malabarismo de tipos entre
  C e Go.

Embora o primeiro ponto fale por si, o segundo pode ser mais difícil de
entender.
Vamos nos aprofundar no malabarismo de tipos na próxima seção.

### Malabarismo de tipos

Embora alguns tipos de variáveis tenham a mesma representação de memória entre
C/PHP e Go, alguns tipos exigem mais lógica para serem usados diretamente.
Esta talvez seja a parte mais difícil quando se trata de escrever extensões,
pois requer a compreensão dos componentes internos da Zend Engine e de como as
variáveis são armazenadas internamente no PHP.
Esta tabela resume o que você precisa saber:

| Tipo PHP           | Tipo Go                       | Conversão direta | Auxiliar de C para Go             | Auxiliar de Go para C              | Suporte a métodos de classe |
| ------------------ | ----------------------------- | ---------------- | --------------------------------- | ---------------------------------- | --------------------------- |
| `int`              | `int64`                       | ✅               | -                                 | -                                  | ✅                          |
| `?int`             | `*int64`                      | ✅               | -                                 | -                                  | ✅                          |
| `float`            | `float64`                     | ✅               | -                                 | -                                  | ✅                          |
| `?float`           | `*float64`                    | ✅               | -                                 | -                                  | ✅                          |
| `bool`             | `bool`                        | ✅               | -                                 | -                                  | ✅                          |
| `?bool`            | `*bool`                       | ✅               | -                                 | -                                  | ✅                          |
| `?bool`            | `*bool`                       | ✅               | -                                 | -                                  | ✅                          |
| `string`/`?string` | `*C.zend_string`              | ❌               | `frankenphp.GoString()`           | `frankenphp.PHPString()`           | ✅                          |
| `array`            | `frankenphp.AssociativeArray` | ❌               | `frankenphp.GoAssociativeArray()` | `frankenphp.PHPAssociativeArray()` | ✅                          |
| `array`            | `map[string]any`              | ❌               | `frankenphp.GoMap()`              | `frankenphp.PHPMap()`              | ✅                          |
| `array`            | `[]any`                       | ❌               | `frankenphp.GoPackedArray()`      | `frankenphp.PHPPackedArray()`      | ✅                          |
| `mixed`            | `any`                         | ❌               | `GoValue()`                       | `PHPValue()`                       | ❌                          |
| `object`           | `struct`                      | ❌               | _Ainda não implementado_          | _Ainda não implementado_           | ❌                          |

> [!NOTE]
> Esta tabela ainda não é exaustiva e será completada à medida que a API de
> tipos do FrankenPHP se tornar mais completa.
>
> Tipos primitivos e arrays são suportados atualmente, especificamente para
> métodos de classe.
> Objetos ainda não podem ser usados como parâmetros de métodos ou tipos de
> retorno.

Se você consultar o trecho de código da seção anterior, poderá ver que os
auxiliares são usados para converter o primeiro parâmetro e o valor de retorno.
O segundo e o terceiro parâmetros da nossa função `repeat_this()` não precisam
ser convertidos, pois a representação em memória dos tipos subjacentes é a mesma
para C e Go.

#### Trabalhando com arrays

O FrankenPHP oferece suporte nativo para arrays PHP por meio de
`frankenphp.AssociativeArray` ou conversão direta para um mapa ou slice.

`AssociativeArray` representa um
[hashmap](https://en.wikipedia.org/wiki/Hash_table) composto por um campo
`Map: map[string]any` e um campo opcional `Order: []string` (ao contrário dos
arrays associativos do PHP, os mapas em Go não são ordenados).

Se a ordem ou a associação não forem necessárias, também é possível converter
diretamente para um slice `[]any` ou um mapa não ordenado `map[string]any`.

**Criando e manipulando arrays em Go:**

```go
//export_php:function process_data_ordered(array $input): array
func process_data_ordered_map(arr *C.zval) unsafe.Pointer {
    // Converte um array associativo PHP para Go, mantendo a ordem
    associativeArray := frankenphp.GoAssociativeArray(unsafe.Pointer(arr))

    // percorre as entradas em ordem
    for _, key := range associativeArray.Order {
        value, _ = associativeArray.Map[key]
        // faz algo com a chave e o valor
    }

    // retorna um array ordenado
    // se 'Order' não estiver vazio, apenas os pares chave-valor em 'Order'
    // serão respeitados
    return frankenphp.PHPAssociativeArray(AssociativeArray{
        Map: map[string]any{
            "chave1": "valor1",
            "chave2": "valor2",
        },
        Order: []string{"chave1", "chave2"},
    })
}

//export_php:function process_data_unordered(array $input): array
func process_data_unordered_map(arr *C.zval) unsafe.Pointer {
    // Converte um array associativo PHP em um mapa Go sem manter a ordem
    // Ignorar a ordem terá melhor desempenho
    goMap := frankenphp.GoMap(unsafe.Pointer(arr))

    // percorre as entradas sem nenhuma ordem específica
    for key, value := range goMap {
        // faz algo com a chave e o valor
    }

    // retorna um array não ordenado
    return frankenphp.PHPMap(map[string]any{
        "chave1": "valor1",
        "chave2": "valor2",
    })
}

//export_php:function process_data_packed(array $input): array
func process_data_packed(arr *C.zval) unsafe.Pointer {
    // Converte um array compactado PHP para Go
    goSlice := frankenphp.GoPackedArray(unsafe.Pointer(arr), false)

    // percorre o slice em ordem
    for index, value := range goSlice {
        // faz algo com a chave e o valor
    }

    // retorna um array compactado
    return frankenphp.PHPackedArray([]any{"valor1", "valor2", "value3"})
}
```

**Principais recursos da conversão de arrays:**

- **Pares chave-valor ordenados** - Opção para manter a ordem do array
  associativo;
- **Otimizado para múltiplos casos** - Opção para ignorar a ordem para melhor
  desempenho ou converter diretamente para um slice;
- **Detecção automática de listas** - Ao converter para PHP, detecta
  automaticamente se o array deve ser uma lista compactada ou um hashmap;
- **Arrays aninhados** - Os arrays podem ser aninhados e converterão todos os
  tipos suportados automaticamente (`int64`, `float64`, `string`, `bool`, `nil`,
  `AssociativeArray`, `map[string]any`, `[]any`);
- **Objetos não são suportados** - Atualmente, apenas tipos escalares e arrays
  podem ser usados como valores.
  Fornecer um objeto resultará em um valor `null` no array PHP.

##### Métodos disponíveis: empacotado e associativo

- `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer`
  \- Converte para um array PHP ordenado com pares chave-valor;
- `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` - Converte um mapa em
  um array PHP não ordenado com pares chave-valor;
- `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` - Converte um slice
  em um array PHP compactado apenas com valores indexados;
- `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray`
  \- Converte um array PHP em um `AssociativeArray` Go ordenado (mapa com ordem);
- `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` - Converte um array PHP
  em um mapa Go não ordenado;
- `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` - Converte um array PHP
  em um slice Go.

### Declarando uma classe PHP nativa

O gerador suporta a declaração de **classes opacas** como estruturas Go, que
podem ser usadas para criar objetos PHP.
Você pode usar o comentário de diretiva `//export_php:class` para definir uma
classe PHP.
Por exemplo:

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### O que são classes opacas?

**Classes Opacas** são classes cuja estrutura interna (propriedades) é ocultada
do código PHP.
Isso significa:

- **Sem acesso direto às propriedades**: Você não pode ler ou escrever
  propriedades diretamente do PHP (`$user->name` não funcionará);
- **Interface somente para métodos** - Todas as interações devem passar pelos
  métodos que você definir;
- **Melhor encapsulamento** - A estrutura interna de dados é completamente
  controlada pelo código Go;
- **Segurança de tipos** - Sem risco do código PHP corromper o estado interno
  com tipos incorretos;
- **API mais limpa** - Força o design de uma interface pública adequada.

Essa abordagem fornece melhor encapsulamento e evita que o código PHP corrompa
acidentalmente o estado interno dos seus objetos Go.
Todas as interações com o objeto devem passar pelos métodos que você definir
explicitamente.

#### Adicionando métodos às classes

Como as propriedades não são diretamente acessíveis, você **deve definir
métodos** para interagir com suas classes opacas.
Use a diretiva `//export_php:method` para definir o comportamento:

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

#### Parâmetros anuláveis

O gerador suporta parâmetros anuláveis usando o prefixo `?` em assinaturas PHP.
Quando um parâmetro é anulável, ele se torna um ponteiro na sua função Go,
permitindo que você verifique se o valor era `null` no PHP:

```go
//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // Verifica se o parâmetro name foi fornecido (não nulo)
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }

    // Verifica se o parâmetro age foi fornecido (não nulo)
    if age != nil {
        us.Age = int(*age)
    }

    // Verifique se o parâmetro active foi fornecido (não nulo)
    if active != nil {
        us.Active = *active
    }
}
```

**Pontos-chave sobre parâmetros anuláveis:**

- **Tipos primitivos anuláveis** (`?int`, `?float`, `?bool`) tornam-se ponteiros
  (`*int64`, `*float64`, `*bool`) em Go;
- **Strings anuláveis** (`?string`) permanecem como `*C.zend_string`, mas podem
  ser `*nil`;
- **Verifique `nil`** antes de dereferenciar valores de ponteiro;
- **`null` do PHP torna-se `nil` do Go** - quando o PHP passa `null`, sua função
  em Go recebe um ponteiro `nil`.

> [!WARNING]
> Atualmente, os métodos de classe têm as seguintes limitações.
> **Objetos não são suportados** como tipos de parâmetros ou tipos de retorno.
> **Arrays são totalmente suportados** tanto para parâmetros quanto para tipos
> de retorno.
> Tipos suportados: `string`, `int`, `float`, `bool`, `array` e `void` (para
> tipo de retorno).
> **Tipos de parâmetros anuláveis são totalmente suportados** para todos os
> tipos escalares (`?string`, `?int`, `?float`, `?bool`).

Após gerar a extensão, você poderá usar a classe e seus métodos no PHP.
Observe que você **não pode acessar propriedades diretamente**:

```php
<?php

$user = new User();

// ✅ Isso funciona - usando métodos
$user->setAge(25);
echo $user->getName();           // Saída: (vazio, valor padrão)
echo $user->getAge();            // Saída: 25
$user->setNamePrefix("Funcionária");

// ✅ Isso também funciona - parâmetros anuláveis
$user->updateInfo("João", 30, true);        // Todos os parâmetros fornecidos
$user->updateInfo("Joana", null, false);     // Age é nulo
$user->updateInfo(null, 25, null);          // Name e active são nulos

// ❌ Isso NÃO funcionará - acesso direto à propriedade
// echo $user->name;             // Error: Cannot access private property
// $user->age = 30;              // Error: Cannot access private property
```

Este design garante que seu código Go tenha controle total sobre como o estado
do objeto é acessado e modificado, proporcionando melhor encapsulamento e
segurança de tipos.

### Declarando constantes

O gerador suporta a exportação de constantes Go para PHP usando duas diretivas:
`//export_php:const` para constantes globais e `//export_php:classconstant` para
constantes de classe.
Isso permite que você compartilhe valores de configuração, códigos de status e
outras constantes entre código Go e PHP.

#### Constantes globais

Use a diretiva `//export_php:const` para criar constantes PHP globais:

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

#### Constantes de classe

Use a diretiva `//export_php:classconstant ClassName` para criar constantes que
pertencem a uma classe PHP específica:

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

Constantes de classe são acessíveis usando o escopo do nome da classe no PHP:

```php
<?php

// Constantes globais
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// Constantes de classe
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

A diretiva suporta vários tipos de valor, incluindo strings, inteiros,
booleanos, floats e constantes `iota`.
Ao usar `iota`, o gerador atribui automaticamente valores sequenciais (0, 1, 2,
etc.).
As constantes globais ficam disponíveis no seu código PHP como constantes
globais, enquanto as constantes de classe são delimitadas para suas respectivas
classes usando a visibilidade pública.
Ao usar inteiros, diferentes notações possíveis (binária, hexadecimal, octal)
são suportadas e exibidas como estão no arquivo stub do PHP.

Você pode usar constantes da mesma forma que está acostumado no código Go.
Por exemplo, vamos pegar a função `repeat_this()` que declaramos anteriormente e
alterar o último argumento para um inteiro:

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
        // inverte a string
    }

    if mode == STR_NORMAL {
        // sem operação, apenas para mostrar a constante
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

### Usando namespaces

O gerador suporta a organização das funções, classes e constantes da sua
extensão PHP em um namespace usando a diretiva `//export_php:namespace`.
Isso ajuda a evitar conflitos de nomenclatura e proporciona uma melhor
organização para a API da sua extensão.

#### Declarando um namespace

Use a diretiva `//export_php:namespace` no topo do seu arquivo Go para colocar
todos os símbolos exportados em um namespace específico:

```go
//export_php:namespace My\Extension
package main

import "C"

//export_php:function hello(): string
func hello() string {
    return "Olá do namespace My\\Extension!"
}

//export_php:class User
type UserStruct struct {
    // campos internos
}

//export_php:method User::getName(): string
func (u *UserStruct) GetName() unsafe.Pointer {
    return frankenphp.PHPString("João Ninguém", false)
}

//export_php:const
const STATUS_ACTIVE = 1
```

#### Usando extensões com namespace no PHP

Quando um namespace é declarado, todas as funções, classes e constantes são
colocadas sob esse namespace no PHP:

```php
<?php

echo My\Extension\hello(); // "Olá do namespace My\Extension!"

$user = new My\Extension\User();
echo $user->getName(); // "João Ninguém"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### Notas importantes

- Apenas **uma** diretiva de namespace é permitida por arquivo.
  Se várias diretivas de namespace forem encontradas, o gerador retornará um
  erro;
- O namespace se aplica a **todos** os símbolos exportados no arquivo: funções,
  classes, métodos e constantes;
- Os nomes de namespace seguem as convenções de namespace do PHP, usando barras
  invertidas (`\`) como separadores;
- Se nenhum namespace for declarado, os símbolos serão exportados para o
  namespace global como de costume.

### Gerando a extensão

É aqui que a mágica acontece e sua extensão agora pode ser gerada.
Você pode executar o gerador com o seguinte comando:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go
```

> [!NOTE]
> Não se esqueça de definir a variável de ambiente `GEN_STUB_SCRIPT` para o
> caminho do arquivo `gen_stub.php` no código-fonte PHP que você baixou
> anteriormente.
> Este é o mesmo script `gen_stub.php` mencionado na seção de implementação
> manual.

Se tudo correu bem, um novo diretório chamado `build` deve ter sido criado.
Este diretório contém os arquivos gerados para sua extensão, incluindo o arquivo
`my_extension.go` com os stubs de funções PHP gerados.

### Integrando a extensão gerada ao FrankenPHP

Nossa extensão agora está pronta para ser compilada e integrada ao FrankenPHP.
Para fazer isso, consulte a [documentação de compilação](compile.md) do
FrankenPHP para aprender como compilar o FrankenPHP.
Adicione o módulo usando a flag `--with`, apontando para o caminho do seu
módulo:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/<minha-conta>/<meu-modulo>/build
```

Observe que você aponta para o subdiretório `/build` que foi criado durante a
etapa de geração.
Entretanto, isso não é obrigatório: você também pode copiar os arquivos gerados
para o diretório do seu módulo e apontar diretamente para ele.

### Testando sua extensão gerada

Você pode criar um arquivo PHP para testar as funções e classes que criou.
Por exemplo, crie um arquivo `index.php` com o seguinte conteúdo:

```php
<?php

// Usando constantes globais
var_dump(repeat_this('Olá mundo', 5, STR_REVERSE));

// Usando constantes de classe
$processor = new StringProcessor();
echo $processor->process('Olá mundo', StringProcessor::MODE_LOWERCASE);  // "olá mundo"
echo $processor->process('Olá mundo', StringProcessor::MODE_UPPERCASE);  // "OLÁ MUNDO"
```

Depois de integrar sua extensão ao FrankenPHP, como demonstrado na seção
anterior, você pode executar este arquivo de teste usando
`./frankenphp php-server` e deverá ver sua extensão funcionando.

## Implementação manual

Se você quiser entender como as extensões funcionam ou precisar de controle
total sobre elas, pode escrevê-las manualmente.
Essa abordagem oferece controle total, mas requer mais código boilerplate.

### Função básica

Veremos como escrever uma extensão PHP simples em Go que define uma nova função
nativa.
Essa função será chamada do PHP e disparará uma goroutine que registra uma
mensagem nos logs do Caddy.
Essa função não recebe parâmetros e não retorna nada.

#### Definindo a função Go

No seu módulo, você precisa definir uma nova função nativa que será chamada do
PHP.
Para fazer isso, crie um arquivo com o nome desejado, por exemplo,
`extension.go`, e adicione o seguinte código:

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
        caddy.Log().Info("Olá de uma goroutine!")
    }()
}
```

A função `frankenphp.RegisterExtension()` simplifica o processo de registro de
extensões, manipulando a lógica interna de registro do PHP.
A função `go_print_something` usa a diretiva `//export` para indicar que estará
acessível no código C que escreveremos, graças ao CGO.

Neste exemplo, nossa nova função disparará uma goroutine que registra uma
mensagem nos logs do Caddy.

#### Definindo a função PHP

Para permitir que o PHP chame nossa função, precisamos definir uma função PHP
correspondente.
Para isso, criaremos um arquivo stub, por exemplo, `extension.stub.php`, que
conterá o seguinte código:

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

Este arquivo define a assinatura da função `go_print()`, que será chamada do
PHP.
A diretiva `@generate-class-entries` permite que o PHP gere automaticamente
entradas de função para nossa extensão.

Isso não é feito manualmente, mas usando um script fornecido no código-fonte do
PHP (certifique-se de ajustar o caminho para o script `gen_stub.php` com base em
onde o código-fonte do PHP está localizado):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Este script gerará um arquivo chamado `extension_arginfo.h` que contém as
informações necessárias para que o PHP saiba como definir e chamar nossa função.

#### Escrevendo a ponte entre Go e C

Agora, precisamos escrever a ponte entre Go e C.
Crie um arquivo chamado `extension.h` no diretório do seu módulo com o seguinte
conteúdo:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Em seguida, crie um arquivo chamado `extension.c` que executará as seguintes
etapas:

- Incluir cabeçalhos PHP;
- Declarar nossa nova função nativa PHP `go_print()`;
- Declarar os metadados da extensão.

Vamos começar incluindo os cabeçalhos necessários:

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Contém símbolos exportados pelo Go
#include "_cgo_export.h"
```

Em seguida, definimos nossa função PHP como uma função nativa da linguagem:

```c
PHP_FUNCTION(go_print)
{
    ZEND_PARSE_PARAMETERS_NONE();

    go_print_something();
}

zend_module_entry ext_module_entry = {
    STANDARD_MODULE_HEADER,
    "ext_go",
    ext_functions, /* Funções */
    NULL,          /* MINIT */
    NULL,          /* MSHUTDOWN */
    NULL,          /* RINIT */
    NULL,          /* RSHUTDOWN */
    NULL,          /* MINFO */
    "0.1.1",
    STANDARD_MODULE_PROPERTIES
};
```

Neste caso, nossa função não recebe parâmetros e não retorna nada.
Ela simplesmente chama a função Go que definimos anteriormente, exportada usando
a diretiva `//export`.

Finalmente, definimos os metadados da extensão em uma estrutura
`zend_module_entry`, como seu nome, versão e propriedades.
Essas informações são necessárias para que o PHP reconheça e carregue nossa
extensão.
Observe que `ext_functions` é um array de ponteiros para as funções PHP que
definimos e foi gerado automaticamente pelo script `gen_stub.php` no arquivo
`extension_arginfo.h`.

O registro da extensão é tratado automaticamente pela função
`RegisterExtension()` do FrankenPHP, que chamamos em nosso código Go.

### Uso avançado

Agora que sabemos como criar uma extensão PHP básica em Go, vamos tornar nosso
exemplo mais complexo.
Agora, criaremos uma função PHP que recebe uma string como parâmetro e retorna
sua versão em letras maiúsculas.

#### Definindo o stub da função PHP

Para definir a nova função PHP, modificaremos nosso arquivo `extension.stub.php`
para incluir a nova assinatura da função:

```php
<?php

/** @generate-class-entries */

/**
 * Converte uma string para letras maiúsculas.
 *
 * @param string $string A string a ser convertida.
 * @return string A versão em letras maiúsculas da string.
 */
function go_upper(string $string): string {}
```

> [!TIP]
> Não negligencie a documentação das suas funções!
> Você provavelmente compartilhará os stubs da sua extensão com outras
> pessoas desenvolvedoras para documentar como usar a sua extensão e quais
> recursos estão disponíveis.

Ao gerar novamente o arquivo stub com o script `gen_stub.php`, o arquivo
`extension_arginfo.h` deverá ficar assim:

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

Podemos ver que a função `go_upper` é definida com um parâmetro do tipo `string`
e um tipo de retorno `string`.

#### Malabarismo de tipos entre Go e PHP/C

Sua função Go não pode aceitar diretamente uma string PHP como parâmetro.
Você precisa convertê-la para uma string Go.
Felizmente, o FrankenPHP fornece funções auxiliares para lidar com a conversão
entre strings PHP e strings Go, semelhante ao que vimos na abordagem do gerador.

O arquivo de cabeçalho permanece simples:

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Agora podemos escrever a ponte entre Go e C no nosso arquivo `extension.c`.
Passaremos a string PHP diretamente para a nossa função Go:

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

Você pode aprender mais sobre `ZEND_PARSE_PARAMETERS_START` e análise de
parâmetros na página dedicada do
[PHP Internals Book](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters).
Aqui, informamos ao PHP que nossa função recebe um parâmetro obrigatório do tipo
`string` como uma `zend_string`.
Em seguida, passamos essa string diretamente para nossa função Go e retornamos o
resultado usando `RETVAL_STR`.

Só resta uma coisa a fazer: implementar a função `go_upper` em Go.

#### Implementando a função Go

Nossa função Go receberá uma `*C.zend_string` como parâmetro, a converterá em
uma string Go usando a função auxiliar do FrankenPHP, a processará e retornará o
resultado como uma nova `*C.zend_string`.
As funções auxiliares cuidam de todo o gerenciamento de memória e da
complexidade da conversão para nós.

```go
import "strings"

//export go_upper
func go_upper(s *C.zend_string) *C.zend_string {
    str := frankenphp.GoString(unsafe.Pointer(s))

    upper := strings.ToUpper(str)

    return (*C.zend_string)(frankenphp.PHPString(upper, false))
}
```

Essa abordagem é muito mais limpa e segura do que o gerenciamento manual de
memória.
As funções auxiliares do FrankenPHP lidam automaticamente com a conversão entre
o formato `zend_string` do PHP e strings em Go.
O parâmetro `false` em `PHPString()` indica que queremos criar uma nova string
não persistente (liberada ao final da requisição).

> [!TIP]
> Neste exemplo, não realizamos nenhum tratamento de erro, mas você deve sempre
> verificar se os ponteiros não são `nil` e se os dados são válidos antes de
> usá-los em suas funções em Go.

### Integrando a extensão ao FrankenPHP

Nossa extensão agora está pronta para ser compilada e integrada ao FrankenPHP.
Para isso, consulte a [documentação de compilação](compile.md) do FrankenPHP
para aprender como compilar o FrankenPHP.
Adicione o módulo usando a flag `--with`, apontando para o caminho do seu
módulo:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/<minha-conta>/<meu-modulo>
```

Pronto!
Sua extensão agora está integrada ao FrankenPHP e pode ser usada no seu código
PHP.

### Testando sua extensão

Após integrar sua extensão ao FrankenPHP, você pode criar um arquivo `index.php`
com exemplos para as funções que você implementou:

```php
<?php

// Testa a função básica
go_print();

// Testa a função avançada
echo go_upper("olá mundo") . "\n";
```

Agora você pode executar o FrankenPHP com este arquivo usando
`./frankenphp php-server` e deverá ver sua extensão funcionando.
