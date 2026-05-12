# Scrivere estensioni PHP in Go

Con FrankenPHP, si possono **scrivere estensioni PHP in Go**, che consentono di creare **funzioni native ad alte prestazioni** che possono essere chiamate direttamente da PHP. Le applicazioni possono sfruttare qualsiasi libreria Go esistente o nuova, nonché il famoso modello di concorrenza di **goroutine direttamente dal codice PHP**.

La scrittura delle estensioni PHP viene solitamente eseguita in C, ma è anche possibile scriverle in altri linguaggi con un po' di lavoro extra. Le estensioni PHP consentono di sfruttare la potenza dei linguaggi di basso livello per estendere le funzionalità di PHP, ad esempio aggiungendo funzioni native o ottimizzando operazioni specifiche.

Grazie ai moduli Caddy, si possono scrivere estensioni PHP in Go e integrarle molto rapidamente in FrankenPHP.

## Due approcci

FrankenPHP offre due modi per creare estensioni PHP in Go:

1. **Utilizzo del generatore di estensioni**: l'approccio consigliato che genera tutti i boilerplate necessari per la maggior parte dei casi d'uso, consentendo di concentrarsi sulla scrittura del codice Go
2. **Implementazione manuale**: controllo completo sulla struttura dell'estensione per casi d'uso avanzati

Inizieremo con l'approccio del generatore poiché è il modo più semplice per iniziare, quindi mostreremo l'implementazione manuale per coloro che necessitano di un controllo completo.

## Usare il generatore di estensioni

FrankenPHP è fornito in bundle con uno strumento che consente di **creare un'estensione PHP** utilizzando solo Go. **Non è necessario scrivere codice C** o utilizzare direttamente CGO: FrankenPHP include anche un'**API di tipi pubblici** per aiutare a scrivere estensioni in Go senza doversi preoccupare **della conversione dei tipi tra PHP/C e Go**.

> [!TIP]
> Per capire come scrivere le estensioni in Go da zero, leggere la sezione di implementazione manuale di seguito, che mostra come scrivere un'estensione PHP in Go senza utilizzare il generatore.

È importante tenere presente che questo strumento **non è un generatore di estensioni completo**. È pensato per aiutare a scrivere semplici estensioni in Go, ma non fornisce le funzionalità più avanzate delle estensioni PHP. Se è necessario scrivere un'estensione più **complessa e ottimizzata**, potrebbe essere necessario scrivere del codice C o utilizzare direttamente CGO.

### Prerequisiti

Come spiegato anche nella sezione di implementazione manuale di seguito, è necessario [recuperare i sorgenti PHP](https://www.php.net/downloads.php) e creare un nuovo modulo Go.

#### Creare un nuovo modulo e ottieni i sorgenti PHP

Il primo passo per scrivere un'estensione PHP in Go è creare un nuovo modulo Go. È possibile utilizzare il seguente comando per questo:

```console
go mod init example.com/example
```

Il secondo passaggio consiste nel [recuperare i sorgenti PHP](https://www.php.net/downloads.php) per i passaggi successivi. Successivamente, basta decomprimerli nella cartella desiderata, non nel modulo Go:

```console
tar xf php-*
```

### Scrivere l'estensione

Ora tutto è impostato per scrivere una funzione nativa in Go. Creare un nuovo file denominato `stringext.go`. La nostra prima funzione prenderà una stringa come argomento, il numero di volte per ripeterla, un booleano per indicare se invertire la stringa e restituirà la stringa risultante. Qualcosa come:

```go
// stringext.go
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

Ci sono due cose importanti da notare:

- Un commento direttiva `//export_php:function` definisce la firma della funzione in PHP. In questo modo il generatore sa come generare la funzione PHP con i parametri e il tipo di ritorno corretti.
- La funzione deve restituire un `unsafe.Pointer`. FrankenPHP fornisce un'API per gestire il type juggling tra C e Go.

Mentre il primo punto parla da solo, il secondo potrebbe essere più difficile da comprendere. Approfondiremo meglio il type juggling più avanti.

### Generazione dell'estensione

È qui che avviene la magia e ora è possibile generare l'estensione. È possibile eseguire il generatore con il seguente comando:

```console
GEN_STUB_SCRIPT=php-src/build/gen_stub.php frankenphp extension-init my_extension.go
```

> [!NOTE]
> Non dimenticare di impostare la variabile d'ambiente `GEN_STUB_SCRIPT` sul percorso del file `gen_stub.php` nei sorgenti PHP scaricati in precedenza. Questo è lo stesso script `gen_stub.php` menzionato nella sezione di implementazione manuale.

Se tutto è andato bene, la cartella del progetto dovrebbe contenere i seguenti file per l'estensione:

- **`my_extension.go`** - Il file sorgente originale (rimane invariato)
- **`my_extension_generated.go`** - File generato con wrapper CGO che chiamano le funzioni personalizzate
- **`my_extension.stub.php`** - File stub PHP per il completamento automatico dell'IDE
- **`my_extension_arginfo.h`** - Informazioni sull'argomento PHP
- **`my_extension.h`** - File di intestazione C
- **`my_extension.c`** - File di implementazione C
- **`README.md`** - Documentazione

> [!IMPORTANT]
> **Il file sorgente (`my_extension.go`) non viene mai modificato.** Il generatore crea un file `_generated.go` separato, contenente wrapper CGO, che chiamano le funzioni originali. Ciò significa che si può controllare in sicurezza la versione del file sorgente senza preoccuparsi che il codice generato possa alterarlo.

### Integrazione dell'estensione generata in FrankenPHP

La nostra estensione è ora pronta per essere compilata e integrata in FrankenPHP. Per farlo, si veda la [documentazione di compilazione](compile.md) per sapere come compilare FrankenPHP. Aggiungere il modulo utilizzando il flag `--with`, che punta al percorso del modulo:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module/build
```

Si noti il puntamento alla sottocartella `/build`, creata durante il passaggio di generazione. Tuttavia, non è obbligatorio: si possono anche copiare i file generati nella cartella del modulo e puntarvi direttamente.

### Test dell'estensione generata

Si può usare un file PHP per testare le funzioni e le classi che appena create. Ad esempio, creare un file `index.php` con il seguente contenuto:

```php
<?php

// Costanti globali
var_dump(repeat_this('Hello World', 5, STR_REVERSE));

// Costanti di classe
$processor = new StringProcessor();
echo $processor->process('Hello World', StringProcessor::MODE_LOWERCASE);  // "hello world"
echo $processor->process('Hello World', StringProcessor::MODE_UPPERCASE);  // "HELLO WORLD"
```

Una volta integrata l'estensione in FrankenPHP, come mostrato nella sezione precedente, si può eseguire questo file di prova con `./frankenphp php-server` e l'estensione dovrebbe essere funzionante.

### Type juggling

Mentre alcuni tipi di variabili hanno la stessa rappresentazione della memoria tra C/PHP e Go, altri tipi richiedono un po' di logica per essere utilizzati direttamente. Questa è forse la parte più difficile quando si tratta di scrivere estensioni, perché richiede la comprensione di Zend Engine e di come le variabili vengono archiviate internamente in PHP.
Questa tabella riassume il necessario:

| Tipo PHP | Tipo Go | Conversione diretta | Helper C -> Go | Helper Go -> C | Supporto per i metodi di classe |
| ------------------ | ----------------------- | ----------------- | --------------------------------- | ---------------------------------- | --------------------- |
| `int` | `int64` | ✅ | - | - | ✅ |
| `?int` | `*int64` | ✅ | - | - | ✅ |
| `float` | `float64` | ✅ | - | - | ✅ |
| `?float` | `*float64` | ✅ | - | - | ✅ |
| `bool` | `bool` | ✅ | - | - | ✅ |
| `?bool` | `*bool` | ✅ | - | - | ✅ |
| `string`/`?string` | `*C.zend_string` | ❌| `frankenphp.GoString()` | `frankenphp.PHPString()` | ✅ |
| `array` | `frankenphp.AssociativeArray` | ❌| `frankenphp.GoAssociativeArray()` | `frankenphp.PHPAssociativeArray()` | ✅ |
| `array` | `map[string]any` | ❌| `frankenphp.GoMap()` | `frankenphp.PHPMap()` | ✅ |
| `array` | `[]any` | ❌| `frankenphp.GoPackedArray()` | `frankenphp.PHPPackedArray()` | ✅ |
| `mixed` | `any` | ❌| `GoValue()` | `PHPValue()` | ❌|
| `callable` | `*C.zval` | ❌| - | frankenphp.CallPHPCallable() | ❌|
| `object` | `struct` | ❌| _Non ancora implementato_ | _Non ancora implementato_ | ❌|

> [!NOTE]
>
> Questa tabella non è ancora esaustiva e verrà completata man mano che l'API dei tipi FrankenPHP diventerà più completa.
>
> Per i metodi di classe, in particolare, sono attualmente supportati i tipi primitivi e gli array. Gli oggetti non possono ancora essere utilizzati come parametri di metodo o tipi di ritorno.

Facendo riferimento allo snippet di codice della sezione precedente, si può vedere che gli helper vengono utilizzati per convertire il primo parametro e il valore restituito. Non è necessario convertire il secondo e il terzo parametro della nostra funzione `repeat_this()`, poiché la rappresentazione in memoria dei tipi sottostanti è la stessa sia per C sia per Go.

#### Lavorare con gli array

FrankenPHP fornisce supporto nativo per gli array PHP tramite `frankenphp.AssociativeArray` o conversione diretta per map e slice.

`AssociativeArray` rappresenta una [mappa hash](https://en.wikipedia.org/wiki/Hash_table) composta da un campo `Map: map[string]any` e un campo `Order: []string` opzionale (a differenza degli "array associativi" PHP, le mappe Go non sono ordinate).

Se l'ordine o l'associazione non sono necessari, è anche possibile convertire direttamente in una porzione `[]any` o in una mappa non ordinata `map[string]any`.

**Creazione e manipolazione di array in Go:**

```go
// Conversione tra array PHP e map/slice Go
package example

// #include <Zend/zend_types.h>
import "C"
import (
    "unsafe"

    "github.com/dunglas/frankenphp"
)

// export_php:function process_data_ordered(array $input): array
func process_data_ordered_map(arr *C.zend_array) unsafe.Pointer {
	// Converte un array associativo PHP in Go, conservando l'ordinamento
	associativeArray, err := frankenphp.GoAssociativeArray[any](unsafe.Pointer(arr))
    if err != nil {
        // gestione dell'errore
    }

	// cicla le voci in ordine
	for _, key := range associativeArray.Order {
		value, _ = associativeArray.Map[key]
		// fare qualcosa con key e value
	}

	// restituisce un array ordinato
	// se 'Order' non è vuoto, saranno rispettate solo le coppie chiave/valore in 'Order'
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
	// Converte array associativi PHP in GO, senza conservare l'ordinamento
	// tralasciare l'ordinamento aumenta le prestazioni
	goMap, err := frankenphp.GoMap[any](unsafe.Pointer(arr))
    if err != nil {
        // gestione dell'errore
    }

	// cicla le voci in ordine sparso
	for key, value := range goMap {
		// fare qualcosa con key e value
	}

	// restituisce un array non ordinato
	return frankenphp.PHPMap(map[string]string {
		"key1": "value1",
		"key2": "value2",
	})
}

// export_php:function process_data_packed(array $input): array
func process_data_packed(arr *C.zend_array) unsafe.Pointer {
	// Converte array compatti PHP in Go
	goSlice, err := frankenphp.GoPackedArray(unsafe.Pointer(arr))
    if err != nil {
        // gestione dell'errore
    }

	// cicla le voci in ordine
	for index, value := range goSlice {
		// fare qualcosa con key e value
	}

	// restituisce un array compatto (senza "buchi")
	return frankenphp.PHPPackedArray([]string{"value1", "value2", "value3"})
}
```

**Caratteristiche principali della conversione di array:**

- **Coppie chiave-valore ordinate** - Opzione per mantenere l'ordine dell'array associativo
- **Ottimizzato per più casi** - Opzione per abbandonare l'ordine per prestazioni migliori o convertirlo direttamente in una sezione
- **Rilevamento automatico di liste** - Durante la conversione in PHP, rileva automaticamente se l'array deve essere un elenco compresso o una mappa hash
- **Array annidati** - Gli array possono essere annidati e convertiranno automaticamente tutti i tipi supportati (`int64`, `float64`, `string`, `bool`, `nil`, `AssociativeArray`, `map[string]any`, `[]any`)
- **Gli oggetti non sono supportati** - Attualmente, solo i tipi scalari e gli array possono essere utilizzati come valori. Fornire un oggetto risulterà in un valore `null` nell'array PHP.

##### Metodi disponibili: compresso e associativo

- `frankenphp.PHPAssociativeArray(arr frankenphp.AssociativeArray) unsafe.Pointer` - Converti in un array PHP ordinato con coppie chiave-valore
- `frankenphp.PHPMap(arr map[string]any) unsafe.Pointer` - Converte una mappa in un array PHP non ordinato con coppie chiave-valore
- `frankenphp.PHPPackedArray(slice []any) unsafe.Pointer` - Converte una sezione in un array compresso PHP con solo valori indicizzati
- `frankenphp.GoAssociativeArray(arr unsafe.Pointer, ordered bool) frankenphp.AssociativeArray` - Converte un array PHP in un Go ordinato `AssociativeArray` (mappa con ordine)
- `frankenphp.GoMap(arr unsafe.Pointer) map[string]any` - Converte un array PHP in una mappa Go non ordinata
- `frankenphp.GoPackedArray(arr unsafe.Pointer) []any` - Converte un array PHP in una sezione Go
- `frankenphp.IsPacked(zval *C.zend_array) bool` - Controlla se un array PHP è compresso (solo indicizzato) o associativo (coppie chiave-valore)

### Lavorare con callable

FrankenPHP fornisce un modo per lavorare con callablePHP utilizzando l'helper `frankenphp.CallPHPCallable`. Ciò consente di chiamare funzioni o metodi PHP dal codice Go.

Per dimostrarlo, creiamo la nostra funzione `array_map()` che accetta un callable e un array, applica il richiamabile a ciascun elemento dell'array e restituisce un nuovo array con i risultati:

```go
// Chiamando un callable PHP da una funzione estensione definita in Go
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

Si noti come utilizziamo `frankenphp.CallPHPCallable()` per chiamare il callable PHP passato come parametro. Questa funzione accetta un puntatore al richiamabile e un array di argomenti e restituisce il risultato dell'esecuzione del callable. Si possono usare tutte le sintassi per i callable:

```php
<?php

$result = my_array_map([1, 2, 3], function($x) { return $x * 2; });
// $result sarà [2, 4, 6]

$result = my_array_map(['hello', 'world'], 'strtoupper');
// $result sarà ['HELLO', 'WORLD']
```

### Dichiarare una classe PHP nativa

Il generatore supporta la dichiarazione di **classi opache** come strutture Go, che possono essere utilizzate per creare oggetti PHP. È possibile utilizzare una direttiva commento `//export_php:class` per definire una classe PHP. Per esempio:

```go
// Dichiarazione di classe PHP supportata da uno struct Go
package example

//export_php:class User
type UserStruct struct {
    Name string
    Age  int
}
```

#### Cosa sono le classi opache?

Le **classi opache** sono classi in cui la struttura interna (proprietà) è nascosta dal codice PHP. Ciò significa:

- **Nessun accesso diretto alle proprietà**: non è possibile leggere o scrivere proprietà direttamente da PHP (`$user->name` non funziona)
- **Interfaccia solo con metodi** - Tutte le interazioni devono passare attraverso i metodi definiti dall'utente
- **Migliore incapsulamento** - La struttura dei dati interni è completamente controllata dal codice Go
- **Sicurezza dei tipi** - Nessun rischio che il codice PHP corrompa lo stato interno con tipi errati
- **API più pulita**: obbliga a progettare un'interfaccia pubblica adeguata

Questo approccio fornisce un migliore incapsulamento e impedisce al codice PHP di corrompere accidentalmente lo stato interno degli oggetti Go. Tutte le interazioni con l'oggetto devono passare attraverso i metodi definiti esplicitamente.

#### Aggiunta di metodi alle classi

Poiché le proprietà non sono direttamente accessibili, **è necessario definire metodi** per interagire con le classi opache. Utilizzare la direttiva `//export_php:method` per definire il comportamento:

```go
// Definire dei metodi su una class PHP in Go
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

#### Parametri nullabili

Il generatore supporta parametri nullabili utilizzando il prefisso `?` nelle firme PHP. Quando un parametro è nullabile, diventa un puntatore nella funzione Go, permettendo di verificare se il valore era `null` in PHP:

```go
// Gestione di parametri nullabili in PHP in un metodo Go
package example

// #include <Zend/zend_types.h>
import "C"
import (
	"unsafe"

	"github.com/dunglas/frankenphp"
)

//export_php:method User::updateInfo(?string $name, ?int $age, ?bool $active): void
func (us *UserStruct) UpdateInfo(name *C.zend_string, age *int64, active *bool) {
    // Controlla se name sia non nullo
    if name != nil {
        us.Name = frankenphp.GoString(unsafe.Pointer(name))
    }

    // Controlla se age sia non nullo
    if age != nil {
        us.Age = int(*age)
    }

    // Controlla se active sia non nullo
    if active != nil {
        us.Active = *active
    }
}
```

**Punti chiave sui parametri nullabili:**

- **Tipi primitivi nullabili** (`?int`, `?float`, `?bool`) diventano puntatori (`*int64`, `*float64`, `*bool`) in Go
- Le **stringhe nullabili** (`?string`) rimangono come `*C.zend_string` ma possono essere `nil`
- **Verifica di `nil`** prima di dereferenziare i valori del puntatore
- **PHP `null` diventa Go `nil`** - quando PHP passa `null`, la funzione Go riceve un puntatore `nil`

> [!WARNING]
>
> Attualmente, i metodi delle classi presentano le seguenti limitazioni. **Gli oggetti non sono supportati** come tipi di parametri o tipi restituiti. **Gli array sono completamente supportati** sia per i parametri sia per i tipi restituiti. Tipi supportati: `string`, `int`, `float`, `bool`, `array` e `void` (per il tipo restituito). **I tipi di parametri Nullable sono completamente supportati** per tutti i tipi scalari (`?string`, `?int`, `?float`, `?bool`).

Dopo aver generato l'estensione, sarà consentito utilizzare la classe e i suoi metodi in PHP. Si tenga presente che **non si può accedere direttamente alle proprietà**:

```php
<?php

$user = new User();

// ✅ Funziona perché usa i metodi
$user->setAge(25);
echo $user->getName();           // Output: (vuoto, valore predefinito)
echo $user->getAge();            // Output: 25
$user->setNamePrefix("Employee");

// ✅ Anche questo funziona, ha parametri nullabili
$user->updateInfo("John", 30, true);        // Tutti i parametri forniti
$user->updateInfo("Jane", null, false);     // Age è null
$user->updateInfo(null, 25, null);          // Name ed active sono null

// ❌ NON funziona, cerca di accedere direttamente alle proprietà
// echo $user->name;             // Error: Cannot access private property
// $user->age = 30;              // Error: Cannot access private property
```

Questa progettazione garantisce che il codice Go abbia il controllo completo sul modo in cui si accede e si modifica lo stato dell'oggetto, fornendo un migliore incapsulamento e indipendenza dai tipi.

### Dichiarare costanti

Il generatore supporta l'esportazione di costanti Go in PHP utilizzando due direttive: `//export_php:const` per costanti globali e `//export_php:classconst` per costanti di classe. Ciò consente di condividere valori di configurazione, codici di stato e altre costanti tra Go e il codice PHP.

#### Costanti globali

Utilizzare la direttiva `//export_php:const` per creare costanti PHP globali:

```go
// Esportare costanti globali PHP da Go
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
>
> Le costanti PHP prenderanno il nome della costante Go, quindi è consigliabile utilizzare le lettere maiuscole.

#### Costanti di classe

Utilizzare la direttiva `//export_php:classconst ClassName` per creare costanti che appartengono a una specifica classe PHP:

```go
// Esportare costanti di classe PHP da Go
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
>
> Proprio come le costanti globali, le costanti di classe prenderanno il nome della costante Go.

Le costanti della classe sono accessibili utilizzando l'ambito del nome della classe in PHP:

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

La direttiva supporta vari tipi di valore, tra cui stringhe, numeri interi, booleani, numeri in virgola mobile e costanti iota. Quando si utilizza `iota`, il generatore assegna automaticamente valori sequenziali (0, 1, 2, ecc.). Le costanti globali diventano disponibili nel codice PHP come costanti globali, mentre le costanti di classe hanno come ambito le rispettive classi utilizzando la visibilità pubblica. Quando si utilizzano numeri interi, diverse possibili notazioni (binaria, esadecimale, ottale) sono supportate e scaricate così come nel file stub PHP.

Si possono utilizzare le costanti come nel codice Go. Ad esempio, prendiamo la funzione `repeat_this()` dichiarata in precedenza e cambiamo l'ultimo argomento in un numero intero:

```go
// Combinare funzioni, classi, metodi e costanti in una sola estensione
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
		// inverte la stringa
	}

	if mode == STR_NORMAL {
		// nessuna operazione, solo per mostrare la costante
	}

	return frankenphp.PHPString(result, false)
}

//export_php:class StringProcessor
type StringProcessorStruct struct {
	// campi interni
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

### Uso dei namespace

Il generatore supporta l'organizzazione delle funzioni, delle classi e delle costanti dell'estensione PHP in un namespace, utilizzando la direttiva `//export_php:namespace`. Ciò aiuta a evitare conflitti di denominazione e fornisce una migliore organizzazione per l'API dell'estensione.

#### Dichiarare un namespace

Utilizzare la direttiva `//export_php:namespace` nella parte superiore del file Go per mettere tutti i simboli esportati in un namespace specifico:

```go
// Mette i simboli esportati in namespace PHP
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
	// campi interni
}

//export_php:method User::getName(): string
func (u *UserStruct) GetName() unsafe.Pointer {
    return frankenphp.PHPString("John Doe", false)
}

//export_php:const
const STATUS_ACTIVE = 1
```

#### Utilizzo dell'estensione con namespace in PHP

Quando viene dichiarato un namespace, tutte le funzioni, classi e costanti vengono inserite sotto quel namespace in PHP:

```php
<?php

echo My\Extension\hello(); // "Hello from My\Extension namespace!"

$user = new My\Extension\User();
echo $user->getName(); // "John Doe"

echo My\Extension\STATUS_ACTIVE; // 1
```

#### Note importanti

- È consentita solo **una** direttiva namespace per file. Se vengono trovate più direttive namespace, il generatore restituirà un errore.
- Il namespace si applica a **tutti** i simboli esportati nel file: funzioni, classi, metodi e costanti.
- I namespace seguono le convenzioni dei namespace PHP con le barre rovesciate (`\`) come separatori.
- Se non viene dichiarato alcun namespace, i simboli vengono esportati nel namespace globale, come al solito.

## Implementazione manuale

Se si vuole capire come funzionano le estensioni o si ha bisogno del pieno controllo sull'estensione, le si può scriverle manualmente. Questo approccio offre il controllo completo ma richiede più codice standard.

### Funzione di base

Vedremo come scrivere una semplice estensione PHP in Go che definisce una nuova funzione nativa. Questa funzione verrà chiamata da PHP e attiverà una goroutine che registra un messaggio nei log di Caddy. Questa funzione non accetta alcun parametro e non restituisce nulla.

#### Definire la funzione Go

Nel modulo, va definita una nuova funzione nativa che verrà chiamata da PHP. Per farlo, creare un file con il nome desiderato, ad esempio `extension.go`, e aggiungere il seguente codice:

```go
// extension.go
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

La funzione `frankenphp.RegisterExtension()` semplifica il processo di registrazione dell'estensione gestendo la logica di registrazione PHP interna. La funzione `go_print_something` utilizza la direttiva `//export` per indicare che sarà accessibile nel codice C che scriveremo, grazie a CGO.

In questo esempio, la nostra nuova funzione attiverà una goroutine che registra un messaggio nei log di Caddy.

#### Definire la funzione PHP

Per consentire a PHP di chiamare la nostra funzione, dobbiamo definire una funzione PHP corrispondente. Per questo creeremo un file stub, ad esempio `extension.stub.php`, che conterrà il seguente codice:

```php
<?php
// extension.stub.php

/** @generate-class-entries */

function go_print(): void {}
```

Questo file definisce la firma della funzione `go_print()`, che verrà chiamata da PHP. La direttiva `@generate-class-entries` consente a PHP di generare automaticamente voci di funzioni per la nostra estensione.

Questo non viene fatto manualmente ma utilizzando uno script fornito nei sorgenti PHP (adattare il percorso allo script `gen_stub.php` in base alla posizione dei sorgenti PHP):

```bash
php ../php-src/build/gen_stub.php extension.stub.php
```

Questo script genererà un file denominato `extension_arginfo.h` che contiene le informazioni necessarie affinché PHP sappia come definire e chiamare la nostra funzione.

#### Scrivere il bridge tra Go e C

Ora dobbiamo scrivere il bridge tra Go e C. Creare un file denominato `extension.h` nella cartella del modulo con il seguente contenuto:

```c
// extension.h
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Successivamente, creare un file denominato `extension.c` che eseguirà i seguenti passaggi:

- Includere header PHP;
- Dichiarare la nostra nuova funzione PHP nativa `go_print()`;
- Dichiarare i metadati dell'estensione.

Iniziamo includendo gli header richiesti:

```c
// extension.c
#include <php.h>
#include "extension.h"
#include "extension_arginfo.h"

// Contains symbols exported by Go
#include "_cgo_export.h"
```

Definiamo quindi la nostra funzione PHP come una funzione del linguaggio nativo:

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

In questo caso, la nostra funzione non accetta parametri e non restituisce nulla. Chiama semplicemente la funzione Go definita in precedenza, esportata utilizzando la direttiva `//export`.

Infine, definiamo i metadati dell'estensione in una struttura `zend_module_entry`, come nome, versione e proprietà. Queste informazioni sono necessarie affinché PHP riconosca e carichi la nostra estensione. Tieni presente che `ext_functions` è un array di puntatori alle funzioni PHP che abbiamo definito ed è stato generato automaticamente dallo script `gen_stub.php` nel file `extension_arginfo.h`.

La registrazione dell'estensione viene gestita automaticamente dalla funzione `RegisterExtension()` di FrankenPHP che chiamiamo nel nostro codice Go.

### Utilizzo avanzato

Ora che sappiamo come creare un'estensione PHP di base in Go, rendiamo più complesso il nostro esempio. Creeremo ora una funzione PHP che accetta una stringa come parametro e restituisce la sua versione in maiuscolo.

#### Definire lo stub della funzione PHP

Per definire la nuova funzione PHP, modificheremo il nostro file `extension.stub.php` per includere la nuova firma della funzione:

```php
<?php
// extension.stub.php

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
> Non trascurare la documentazione delle funzioni! È probabile che gli stub delle estensioni saranno condivisi con altri sviluppatori per documentare come utilizzare l'estensione e quali funzionalità sono disponibili.

Rigenerando il file stub con lo script `gen_stub.php`, il file `extension_arginfo.h` dovrebbe assomigliare a questo:

```c
// extension_arginfo.h (generated)
ZEND_BEGIN_ARG_WITH_RETURN_TYPE_INFO_EX(arginfo_go_upper, 0, 1, IS_STRING, 0)
    ZEND_ARG_TYPE_INFO(0, string, IS_STRING, 0)
ZEND_END_ARG_INFO()

ZEND_FUNCTION(go_upper);

static const zend_function_entry ext_functions[] = {
    ZEND_FE(go_upper, arginfo_go_upper)
    ZEND_FE_END
};
```

Possiamo vedere che la funzione `go_upper` è definita con un parametro di tipo `string` e un tipo restituito di `string`.

#### Type juggling tra Go e PHP/C

La funzione Go non può accettare direttamente una stringa PHP come parametro. È necessario convertirlo in una stringa Go. Fortunatamente, FrankenPHP fornisce funzioni di supporto per gestire la conversione tra stringhe PHP e stringhe Go, simili a quanto visto nell'approccio del generatore.

Il file header rimane semplice:

```c
// extension.h
#ifndef _EXTENSION_H
#define _EXTENSION_H

#include <php.h>

extern zend_module_entry ext_module_entry;

#endif
```

Ora possiamo scrivere il bridge tra Go e C nel nostro file `extension.c`. Passeremo la stringa PHP direttamente alla nostra funzione Go:

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

Puoi saperne di più su `ZEND_PARSE_PARAMETERS_START` e sull'analisi dei parametri nella pagina dedicata del libro [PHP Internals](https://www.phpinternalsbook.com/php7/extensions_design/php_functions.html#parsing-parameters-zend-parse-parameters). Qui diciamo a PHP che la nostra funzione accetta un parametro obbligatorio di tipo `string` come `zend_string`. Passiamo quindi questa stringa direttamente alla nostra funzione Go e restituiamo il risultato utilizzando `RETVAL_STR`.

Resta solo una cosa da fare: implementare la funzione `go_upper` in Go.

#### Implementa la funzione Vai

La nostra funzione Go prenderà un `*C.zend_string` come parametro, lo convertirà in una stringa Go utilizzando la funzione helper di FrankenPHP, lo elaborerà e restituirà il risultato come un nuovo `*C.zend_string`. Le funzioni di supporto gestiscono per noi tutta la complessità della gestione della memoria e della conversione.

```go
// extension.go
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

Questo approccio è molto più pulito e sicuro della gestione manuale della memoria.
Le funzioni di supporto di FrankenPHP gestiscono automaticamente la conversione tra il formato `zend_string` di PHP e le stringhe Go.
Il parametro `false` in `PHPString()` indica che vogliamo creare una nuova stringa non persistente (liberata alla fine della richiesta).

> [!TIP]
>
> In questo esempio, non  gestiamo gli errori, ma si dovrebbe sempre verificare che i puntatori non siano `nil` e che i dati siano validi prima di utilizzarli nelle funzioni Go.

### Integrazione dell'estensione in FrankenPHP

La nostra estensione è ora pronta per essere compilata e integrata in FrankenPHP. Per fare ciò, fare riferimento alla [documentazione di compilazione] di FrankenPHP(compile.md) per sapere come compilare FrankenPHP. Aggiungere il modulo utilizzando il flag `--with`, che punta al percorso del modulo:

```console
CGO_ENABLED=1 \
XCADDY_GO_BUILD_FLAGS="-ldflags='-w -s' -tags=nobadger,nomysql,nopgx" \
CGO_CFLAGS=$(php-config --includes) \
CGO_LDFLAGS="$(php-config --ldflags) $(php-config --libs)" \
xcaddy build \
    --output frankenphp \
    --with github.com/my-account/my-module
```

Questo è tutto! L'estensione è ora integrata in FrankenPHP e può essere utilizzata nel codice PHP.

### Testare l'estensione

Dopo aver integrato l'estensione in FrankenPHP, si può creare un file `index.php` con esempi per le funzioni che appena implementate:

```php
<?php

// Test di base
go_print();

// Test avanzato
echo go_upper("hello world") . "\n";
```

Ora si può eseguire FrankenPHP con questo file utilizzando `./frankenphp php-server` e l'estensione dovrebbe funzionare.
