# GoでPHP拡張モジュールを作成する

FrankenPHPでは、**GoでPHP拡張モジュールを作成する**ことができます。これにより、PHPから直接呼び出せる**高パフォーマンスなネイティブ関数**を作成できます。アプリケーションは既存または新しいGoライブラリを活用でき、**PHPコードから直接goroutineの**強力な並行性モデルを使用できます。

PHP拡張モジュールの記述は通常Cで行われますが、少しの追加作業で他の言語でも作成可能です。PHP拡張モジュールは低レベル言語の力を活用してPHPの機能を拡張することができます。例えば、ネイティブ関数を追加したり、特定の操作を最適化したりできます。

Caddyモジュールのおかげで、GoでPHP拡張モジュールを書いてFrankenPHPに簡単に統合できます。

## 2つのアプローチ

FrankenPHPでは、GoでPHP拡張モジュールを作成する2つの方法を提供します：

1. **拡張モジュールジェネレーターを使用** - ほとんどのユースケースに必要なボイラープレートを自動生成する推奨アプローチで、Goコードの記述に集中できます
2. **手動実装** - 拡張モジュール構造を細かく制御したい高度なユースケース

最初に始めやすいジェネレーター方式を紹介し、その後で完全な制御が必要な場合の手動実装方式を説明します。

## 拡張モジュールジェネレーターを使用する

FrankenPHPにはGoのみを使用して**PHP拡張モジュールを作成する**ツールが付属しています。**Cコードを書く必要がなく**、CGOを直接使用する必要もありません。FrankenPHPには**パブリック型API**も含まれており、**PHP/CとGo間の型変換**を心配することなくGoでPHP拡張を書くのに役立ちます。

> [!TIP]
> 拡張モジュールをGoで一から書く方法を理解したい場合は、ジェネレーターを使用せずにGoでPHP拡張モジュールを書く方法を紹介する後述の手動実装セクションを参照してください。

注意すべきことは、このツールは**完全な拡張モジュールジェネレーター**ではないことです。GoでシンプルなPHP拡張モジュールを書くのには十分役立ちますが、高度なPHP拡張モジュールの機能には対応していません。より**複雑で最適化された**拡張モジュールを書く必要がある場合は、Cコードを書いたり、CGOを直接使用したりする必要があるかもしれません。

### 前提条件

以下の手動実装セクションでも説明しているように、[PHPのソースを取得](https://www.php.net/downloads.php)し、新しいGoモジュールを作成する必要があります。

#### 新しいモジュールの作成とPHPソースの取得

GoでPHP拡張モジュールを書く最初のステップは、新しいGoモジュールの作成です。以下のコマンドを使用できます：

```console
go mod init github.com/my-account/my-module
```

2番目のステップは、次のステップのために[PHPのソースを取得](https://www.php.net/downloads.php)することです。取得したら、Goモジュールのディレクトリ内ではなく、任意のディレクトリに展開します：

```console
tar xf php-*
```

### 拡張モジュールの記述

これでGoでネイティブ関数を書く準備が整いました。`stringext.go`という名前の新しいファイルを作成します。最初の関数は文字列を引数として取り、それを指定された回数だけ繰り返し、文字列を逆転するかどうかを示すブール値を受け取り、結果の文字列を返します。これは以下のようになります：

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

ここで重要なポイントが2つあります：

* ディレクティブコメント`//export_php:function`はPHPでの関数シグネチャを定義します。これにより、ジェネレーターは適切なパラメータと戻り値の型でPHP関数を生成する方法を知ることができます。
* 関数は`unsafe.Pointer`を返さなければなりません。FrankenPHPはCとGo間の型変換を支援するAPIを提供しています。

前者は理解しやすいですが、後者は少し複雑かもしれません。次のセクションで型変換について詳しく説明します。

### 型変換

C/PHPとGoの間でメモリ表現が同じ変数型もありますが、直接使用するにはより多くのロジックが必要な型もあります。これは拡張モジュールを書く際の最も挑戦的な部分かもしれません。Zendエンジンの内部仕組みや、変数がPHP内でどのように格納されているかを理解する必要があるためです。以下の表は、知っておくべき重要な情報をまとめています：

| PHP型           | Go型          | 直接変換 | CからGoヘルパー       | GoからCヘルパー        | クラスメソッドサポート |
|--------------------|------------------|-------------------|-----------------------|------------------------|-----------------------|
| `int`              | `int64`          | ✅                 | -                     | -                      | ✅                     |
| `?int`             | `*int64`         | ✅                 | -                     | -                      | ✅                     |
| `float`            | `float64`        | ✅                 | -                     | -                      | ✅                     |
| `?float`           | `*float64`       | ✅                 | -                     | -                      | ✅                     |
| `bool`             | `bool`           | ✅                 | -                     | -                      | ✅                     |
| `?bool`            | `*bool`          | ✅                 | -                     | -                      | ✅                     |
| `string`/`?string` | `*C.zend_string` | ❌                 | frankenphp.GoString() | frankenphp.PHPString() | ✅                     |
| `array`            | `slice`/`map`    | ❌                 | _未実装_ | _未実装_  | ❌                     |
| `object`           | `struct`         | ❌                 | _未実装_ | _未実装_  | ❌                     |

> [!NOTE]
> この表はまだ完全ではなく、FrankenPHPの型APIがより完全になるにつれて完成されます。
>
> クラスメソッドについては、現在プリミティブ型のみがサポートされています。配列とオブジェクトはまだメソッドパラメータや戻り値の型として使用できません。

前のセクションのコードスニペットを参照すると、最初のパラメータと戻り値の変換にヘルパーが使用されていることがわかります。 `repeat_this()`関数の2番目と3番目の引数は、基礎となる型のメモリ表現がCとGoで同じであるため、変換する必要がありません。

### ネイティブPHPクラスの宣言

ジェネレーターは、PHPオブジェクトを作成するために使用できる**不透明クラス（opaque classes）**をGo構造体として宣言することをサポートしています。`//export_php:class`ディレクティブコメントを使用してPHPクラスを定義できます。例：

```go
//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### 不透明クラスとは何ですか？

**不透明クラス（opaque classes）**は、内部構造（プロパティ）がPHPコードから隠されているクラスです。これは以下を意味します：

* **プロパティへの直接アクセス不可** ：PHPから直接プロパティを読み書きできません（`$user->name`は機能しません）
* **メソッド経由のみで操作** - すべてのやりとりはGoで定義したメソッドを通じて行う必要があります
* **より良いカプセル化** - 内部データ構造は完全にGoコードによって制御されます
* **型安全性** - PHP側から誤った型で内部状態が破壊されるリスクがありません
* **よりクリーンなAPI** - 適切な公開インターフェースを設計することを強制します

このアプローチは優れたカプセル化を実現し、PHPコードがGoオブジェクトの内部状態を意図せずに破壊してしまうことを防ぎます。オブジェクトとのすべてのやりとりは、明示的に定義したメソッドを通じて行う必要があります。

#### クラスにメソッドを追加する

プロパティは直接アクセスできないため、不透明クラスとやりとりするには **メソッドを定義する必要があります** 。`//export_php:method`ディレクティブを使用して動作を定義します：

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

#### Nullableパラメータ

ジェネレーターは、PHPシグネチャにおける`?`プレフィックスを使用ったnullableパラメータをサポートしています。パラメータがnullableの場合、Go関数内ではポインタとして扱われ、PHP側で値が`null`だったかどうかを確認できます：

```go
//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // nameが渡された（nullではない）かチェック
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }
    
    // ageが渡された（nullではない）かチェック
    if age != nil {
        us.Age = int(*age)
    }
    
    // activeが渡された（nullではない）かチェック
    if active != nil {
        us.Active = *active
    }
}
```

**Nullableパラメータの重要なポイント：**

* **プリミティブ型のnullable** (`?int`, `?float`, `?bool`) はGoではそれぞれポインタ (`*int64`, `*float64`, `*bool`) になります
* **nullable文字列** (`?string`) は `*C.zend_string` のままですが、`nil` になることがあります
* ポインタ値を逆参照する前に **`nil`をチェック** してください
* **PHPの`null`はGoの`nil`になります** - PHPが`null`を渡すと、Go関数は`nil`ポインタを受け取ります

> [!WARNING]
> 現在、クラスメソッドには次の制限があります。**配列とオブジェクトはパラメータ型や戻り値の型としてサポートされていません**。サポートされるのは`string`、`int`、`float`、`bool`、`void`（戻り値の型）といったスカラー型のみです。**nullableなスカラー型はすべてサポートされています** （`?string`、`?int`、`?float`、`?bool`）。

拡張を生成した後、PHP側でクラスとそのメソッドを使用できるようになります。ただし**プロパティに直接アクセスできない**ことに注意してください：

```php
<?php

$user = new User();

// ✅ これは動作します - メソッドの使用
$user->setAge(25);
echo $user->getName();           // 出力: (empty、デフォルト値)
echo $user->getAge();            // 出力: 25
$user->setNamePrefix("Employee");

// ✅ これも動作します - nullableパラメータ
$user->updateInfo("John", 30, true);        // すべて指定
$user->updateInfo("Jane", null, false);     // Ageがnull
$user->updateInfo(null, 25, null);          // Nameとactiveがnull

// ❌ これは動作しません - プロパティへの直接アクセス
// echo $user->name;             // エラー: privateプロパティにアクセスできません
// $user->age = 30;              // エラー: privateプロパティにアクセスできません
```

この設計により、Goコードがオブジェクトの状態へのアクセスと変更方法を完全に制御でき、より良いカプセル化と型安全性を提供します。

### 定数の宣言

ジェネレーターは、2つのディレクティブを使用してGo定数をPHPにエクスポートすることをサポートしています：グローバル定数用の`//export_php:const`とクラス定数用の`//export_php:classconstant`です。これにより、GoとPHPコード間で設定値、ステータスコード、その他の定数を共有できます。

#### グローバル定数

`//export_php:const`ディレクティブを使用してグローバルなPHP定数を作成できます：

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

#### クラス定数

`//export_php:classconstant ClassName`ディレクティブを使用して、特定のPHPクラスに属する定数を作成できます：

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

クラス定数は、PHPでクラス名スコープを使用してアクセスできます：

```php
<?php

// グローバル定数
echo MAX_CONNECTIONS;    // 100
echo API_VERSION;        // "1.2.3"

// クラス定数
echo User::STATUS_ACTIVE;    // 1
echo User::ROLE_ADMIN;       // "admin"
echo Order::STATE_PENDING;   // 0
```

ディレクティブは、文字列、整数、ブール値、浮動小数点数、iota定数など、さまざまな値の型をサポートしています。`iota`を使用する場合、ジェネレーターは自動的に連続した値（0, 1, 2など）を割り当てます。グローバル定数はPHPコードでグローバル定数として利用可能になり、クラス定数はpublicとしてそれぞれのクラスにスコープされます。整数を使用する場合、異なる記法（バイナリ、16進数、8進数）がサポートされ、PHPのスタブファイルにそのまま出力されます。

Go側のコードでは、いつも通り定数を使用できます。例えば、先ほど作成した`repeat_this()`関数を取り上げ、最後の引数を整数に変更してみましょう：

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
        // 文字列を逆転
    }

    if mode == STR_NORMAL {
        // 何もしない、定数を示すためのみ記載
    }

    return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
    // 内部フィールド
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

### 拡張モジュールの生成

ここでいよいよ、拡張モジュールを生成できるようになります。以下のコマンドでジェネレーターを実行できます：

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go 
```

> [!NOTE]
> `GEN_STUB_SCRIPT`環境変数に、先ほどダウンロードしたPHPソースの`gen_stub.php`ファイルのパスを設定するのを忘れないでください。これは手動実装セクションで言及されているのと同じ`gen_stub.php`スクリプトです。

すべてがうまくいけば、`build`という名前の新しいディレクトリが作成されているはずです。このディレクトリには、生成されたPHP関数スタブを含む`my_extension.go`ファイルなど、拡張用の生成されたファイルが含まれています。

### 生成された拡張モジュールをFrankenPHPへ統合する

拡張モジュールがコンパイルされ、FrankenPHPに統合される準備が整いました。これを行うには、FrankenPHPのコンパイル方法を学ぶために、FrankenPHPの[コンパイルドキュメント](compile.md)を参照してください。`--with`フラグを使用してモジュールを追加し、モジュールのパスを指定します：

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

このとき、生成ステップで作成された`/build`サブディレクトリを指していることに注意してください。ただし、これは必須ではなく、生成されたファイルをモジュールのディレクトリにコピーして、直接それを指定することも可能です。

### 生成された拡張モジュールのテスト

作成した関数とクラスをテストするPHPファイルを作成しましょう。例えば、以下の内容で`index.php`ファイルを作成します：

```php
<?php

// グローバル定数を使用
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// クラス定数を使用
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

前のセクションで示したように拡張モジュールをFrankenPHPに統合し、`./frankenphp php-server`を使用してこのテストファイルを実行することで、拡張モジュールが動作しているのを確認できるはずです。

## 手動実装

拡張モジュールの仕組みを理解したい、または拡張モジュールを完全に制御したい場合は、手動で書くこともできます。このアプローチは完全な制御を実現できますが、より多くのボイラープレートコードが必要になります。

### 基本的な関数

ここでは、新しいネイティブ関数を定義するシンプルなPHP拡張モジュールをGoで手動実装する方法を紹介します。この関数はPHPから呼び出され、その関数がgoroutineを使ってCaddyのログにメッセージ出力するという処理を行います。この関数は引数を取らず、戻り値もありません。

#### Go関数の定義

モジュール内で、PHPから呼び出される新しいネイティブ関数を定義する必要があります。これを行うには、例えば`extension.go`のように任意の名前でファイルを作成し、以下のコードを追加します：

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

`frankenphp.RegisterExtension()`関数は、内部のPHP登録ロジックを処理することで拡張登録プロセスを簡素化します。`go_print_something`関数は`//export`ディレクティブを使用して、CGOのおかげで、これから書くCコードでアクセスできるようになることを示しています。

この例では、新しい関数がCaddyのログにメッセージ出力するgoroutineをトリガーします。

#### PHP関数の定義

PHPがGo関数を呼び出せるようにするには、対応するPHP関数を定義する必要があります。このために、例えば`extension.stub.php`のようにスタブファイルを作成し、以下のコードを記述します：

```php
<?php

/** @generate-class-entries */

function go_print(): void {}
```

このファイルはPHPから呼び出される`go_print()`関数のシグネチャを定義します。`@generate-class-entries`ディレクティブは、PHPがこの拡張モジュールのために関数エントリを自動生成することを可能にします。

これは手動ではなく、PHPソースで提供されるスクリプトを使用して行います（PHPソースが置かれている場所に基づいて`gen_stub.php`スクリプトのパスを調整してください）：

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

このスクリプトは、PHPがこの関数の定義および呼び出し方法を知るのに必要な情報を含む`extension_arginfo.h`という名前のファイルを生成します。

#### GoとC間のブリッジの作成

今度は、GoとC間をつなぐブリッジを書く必要があります。モジュールディレクトリに`extension.h`という名前のファイルを作成し、以下の内容を書きます：

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

次に、以下のステップを実行する`extension.c`という名前のファイルを作成します：

* PHPヘッダーをインクルードする
* 新しいネイティブPHP関数`go_print()`を宣言する
* 拡張モジュールのメタデータを宣言する

まずは必要なヘッダーのインクルードから始めましょう：

```c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Goによってエクスポートされたシンボルを含みます
#include "_cgo_export.h"
```

次に、PHP関数をネイティブ言語関数として定義します：

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

この場合、関数はパラメータを取らず、何も返しません。単に`//export`ディレクティブを使用してエクスポートした、先ほど定義したGo関数を呼び出します。

最後に、名前、バージョン、プロパティなど、拡張のメタデータを`zend_module_entry`構造体で定義します。この情報はPHPが私たちの拡張モジュールを認識してロードするために必要です。`ext_functions`は定義したPHP関数へのポインタの配列であり、`gen_stub.php`スクリプトによって自動生成された`extension_arginfo.h`ファイル内に定義されています。

拡張モジュールの登録は、Goコード内で呼び出しているFrankenPHPの`RegisterExtension()`関数によって自動的に処理されます。

### 高度な使用方法

基本的なPHP拡張をGoで作成する方法が分かったところで、少し例を複雑にしてみましょう。今度は文字列を引数として受け取り、その大文字版を返すPHP関数を作成します。

#### PHP関数スタブの定義

新しいPHP関数を定義するために、`extension.stub.php`ファイルを修正し、次の関数シグネチャを含めます：

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
> 関数のドキュメントを軽視しないでください！拡張スタブを他の開発者と共有する際、拡張機能の使い方や提供している機能を伝えるための重要な手段になります。

`gen_stub.php`スクリプトでスタブファイルを再生成すると、`extension_arginfo.h`ファイルは以下のようになるはずです：

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

この出力から、`go_upper`関数が`string`型の引数を1つ受け取り、`string`型の戻り値を返すことが定義されていのがわかります。

#### GoとPHP/C間の型変換（Type Juggling）

Go関数はPHPの文字列を引数として直接受け取ることはできません。そのためPHPの文字列をGoの文字列へ変換する必要があります。幸いなことに、FrankenPHPは、ジェネレーターアプローチで見たものと同様に、PHP文字列とGo文字列間の変換を処理するヘルパー関数を提供しています。

ヘッダーファイルはシンプルなままです：

```c
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

次に、`extension.c`ファイルにGoとC間のブリッジを書きます。ここではPHPの文字列を直接Go関数に渡します：

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

`ZEND_PARSE_PARAMETERS_START`や引数のパースについては、[PHP Internals Book](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters)の該当ページで詳しく学ぶことができます。この例では、関数が`zend_string`として`string`型の必須引数を1つ取ることをPHPに伝えています。その後、この文字列を直接Go関数に渡し、`RETVAL_STR`を使用して結果を返します。

残るはただ一つ、Go側で`go_upper`関数を実装するだけです。

#### Go関数の実装

Go側の関数では`*C.zend_string`を引数として受け取り、FrankenPHPのヘルパー関数を使用してGoの文字列に変換し、処理を行ったうえで、結果を新たな`*C.zend_string`として返します。メモリ管理と変換の複雑さは、ヘルパー関数がすべて対応してくれます。

```go
import "strings"

//export go_upper
func go_upper(s *C.zend_string) *C.zend_string {
    str := frankenphp.GoString(unsafe.Pointer(s))
    
    upper := strings.ToUpper(str)
    
    return (*C.zend_string)(frankenphp.PHPString(upper, false))
}
```

このアプローチは、手動メモリ管理よりもはるかにクリーンで安全です。FrankenPHPのヘルパー関数は、PHPの`zend_string`形式とGoの文字列間の変換を自動的に処理してくれます。`PHPString()`に`false`引数を指定していることで、新しい非永続文字列（リクエストの終了時に解放される）を作成したいことを示しています。

> [!TIP]
> この例ではエラーハンドリングを省略していますが、Go関数内でポインタが`nil`ではないこと、渡されたデータが有効であることを常に確認するべきです。

### 拡張モジュールのFrankenPHPへの統合

拡張モジュールがコンパイルされ、FrankenPHPに統合される準備が整いました。手順についてはFrankenPHPのコンパイル方法を学ぶために、FrankenPHPの[コンパイルドキュメント](compile.md)を参照してください。`--with`フラグを使用してモジュールを追加し、モジュールのパスを指定します：

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

これで完了です！拡張モジュールがFrankenPHPに統合され、PHPコードで利用できるようになりました。

### 拡張モジュールのテスト

拡張モジュールをFrankenPHPに統合したら、実装した関数を試すための`index.php`ファイルを作成します：

```php
<?php

// 基本関数のテスト
go_print();

// 高度な関数のテスト
echo go_upper("hello world") . "\n";
```

このファイルを使用して`./frankenphp php-server`でFrankenPHPを実行でき、拡張モジュールが動作しているのを確認できるはずです。
