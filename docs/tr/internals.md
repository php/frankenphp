# İç yapı

Bu belge FrankenPHP'nin iç mimarisini açıklar; odak noktası iş parçacığı yönetimi, durum makinesi ve Go ile C/PHP arasındaki CGO sınırıdır.

## FrankenPHP mimarisine genel bakış

FrankenPHP, PHP yorumlayıcısını CGO aracılığıyla doğrudan Go'ya gömer. Her PHP yürütmesi gerçek bir POSIX iş parçacığında çalışır (goroutine değil); çünkü PHP'nin ZTS (Zend Thread Safety) modeli bunu gerektirir. Go bu iş parçacıklarını bir durum makinesi üzerinden yönetir; C ise PHP SAPI yaşam döngüsünü üstlenir.

Ana katmanlar şunlardır:

1. **Go katmanı** (`frankenphp.go`, `phpthread.go`, `thread*.go`, `scaling.go` gibi üst düzey `*.go` dosyaları): İş parçacığı havuzu yönetimi, istek yönlendirme, otomatik ölçekleme
2. **C katmanı** (`frankenphp.c`, `frankenphp.h`): PHP SAPI uygulaması, betik yürütme döngüsü, süper küresel yönetimi
3. **Durum makinesi** (`internal/state/`): Go goroutine'leri ile C iş parçacıkları arasında eşzamanlama

## FrankenPHP iş parçacığı türleri

### Ana iş parçacığı (`phpmainthread.go`)

Ana PHP iş parçacığı (`phpMainThread`) PHP çalışma zamanını başlatır:

1. `php.ini` geçersiz kılmalarını uygular
2. Sanal alan için ortamın bir anlık görüntüsünü (`main_thread_env`) alır
3. PHP SAPI modülünü başlatır
4. Go tarafına hazır olduğunu bildirir

Sunucunun ömrü boyunca hayatta kalır. Diğer tüm iş parçacıkları o `Ready` sinyalini verdikten sonra başlatılır.

### Düzenli iş parçacıkları (`threadregular.go`)

Klasik, çağrı başına bir istek PHP betiklerini işler. Her istek:

1. `requestChan` veya paylaşılan `regularRequestChan` üzerinden bir istek alır
2. `beforeScriptExecution()` ile betik dosya adını döndürür
3. C katmanı PHP betiğini yürütür
4. `afterScriptExecution()` istek bağlamını kapatır

### Worker iş parçacıkları (`threadworker.go`)

Bir PHP betiğini birden fazla istek boyunca canlı tutar. PHP betiği bir döngüde `frankenphp_handle_request()` çağırır:

1. `beforeScriptExecution()` worker betik dosya adını döndürür
2. C katmanı PHP betiğini yürütmeye başlar
3. PHP betiği `frankenphp_handle_request()` çağırır; bu da Go'da `waitForWorkerRequest()` çağırır
4. Go bir istek gelene kadar bekler, ardından istek bağlamını kurar
5. PHP geri çağrısı isteği işler
6. `go_frankenphp_finish_worker_request()` istek bağlamını temizler
7. PHP betiği 3. adıma döner

Betik çıktıktan sonra, worker en az bir kez `frankenphp_handle_request()`'e ulaşmışsa (çıkış temiz olsa da ölümcül bir hatanın sonucu olsa da) hemen yeniden başlatılır. Üstel geri çekme yalnızca ardışık başlatma hatalarına uygulanır; burada betik `frankenphp_handle_request()`'e hiç ulaşmadan çıkar.

## FrankenPHP iş parçacığı durum makinesi

Her iş parçacığının yaşam döngüsünü yöneten bir `ThreadState`'i vardır (`internal/state/state.go` içinde tanımlanır). Durum makinesi tüm durum geçişleri için bir `sync.RWMutex` ve engelleyen beklemeler için kanal tabanlı bir abone kalıbı kullanır.

### FrankenPHP iş parçacığı durumları

```text
Lifecycle:        Reserved → BootRequested → Booting → Inactive → Ready ⇄ (processing)
                                                                    ↓
Shutdown:                                                     ShuttingDown → Done → Reserved
                                                                    ↑
Restart (admin/watcher):                                      Restarting → Yielding → Ready
                                                                    ↑
ZTS reboot (max_requests):                                    Rebooting → RebootReady → Ready
                                                                    ↑
Handler transition:                                       TransitionRequested → TransitionInProgress → TransitionComplete
```

Durumların tam kümesi `internal/state/state.go` içinde tanımlanır:

| Durum                  | Açıklama                                                                                          |
| ---------------------- | ------------------------------------------------------------------------------------------------- |
| `Reserved`             | İş parçacığı yuvası ayrılmıştır ancak henüz başlatılmamıştır. İstek üzerine başlatılabilir.       |
| `BootRequested`        | Başlatma kuyruğa alınmıştır (ör. ana iş parçacığı tarafından) ancak POSIX iş parçacığı henüz başlamamıştır. |
| `Booting`              | Alttaki POSIX iş parçacığı başlıyordur.                                                           |
| `Inactive`             | İş parçacığı canlıdır ancak atanmış bir işleyicisi yoktur. Bellek ayak izi küçüktür.              |
| `Ready`                | İş parçacığının bir işleyicisi vardır ve iş kabul etmeye hazırdır.                                |
| `ShuttingDown`         | İş parçacığı kapanıyordur.                                                                        |
| `Done`                 | İş parçacığı tamamen kapanmıştır. Olası yeniden kullanım için `Reserved`'a döner.                 |
| `Restarting`           | Worker iş parçacığı yeniden başlatılıyordur (ör. admin API veya dosya izleyici aracılığıyla).     |
| `Yielding`             | Worker iş parçacığı denetimi bırakmıştır ve yeniden etkinleştirilmeyi bekliyordur.                |
| `Rebooting`            | Worker iş parçacığı tam bir ZTS yeniden başlatması için C döngüsünden çıkıyordur (ör. `max_requests`). |
| `RebootReady`          | C iş parçacığı çıkmıştır ve ZTS durumu temizlenmiştir; yeni bir C iş parçacığı doğurmaya hazırdır. |
| `TransitionRequested`  | Go tarafından bir işleyici değişikliği istenmiştir.                                               |
| `TransitionInProgress` | C iş parçacığı geçiş isteğini kabul etmiştir.                                                     |
| `TransitionComplete`   | Go tarafı yeni işleyiciyi kurmuştur.                                                              |

### Temel durum makinesi işlemleri

**`RequestSafeStateChange(nextState)`**: Dış goroutine'lerin durum değişikliği istemesinin birincil yoludur. Şunları yapar:

- `Ready` veya `Inactive`'ten atomik olarak başarılı olur (mutex altında)
- `ShuttingDown`, `Done` veya `Reserved`'tan hemen `false` döndürür
- Diğer herhangi bir durumdan `Ready`, `Inactive` veya `ShuttingDown` bekleyerek engeller ve yeniden dener

Bu karşılıklı dışlamayı güvence altına alır: belirli bir iş parçacığında aynı anda `shutdown()`, `setHandler()` veya `drainWorkerThreads()`'ten yalnızca biri başarılı olabilir.

**`WaitFor(states...)`**: İş parçacığı belirtilen durumlardan birine ulaşana kadar engeller. Bekleyenlerin verimli bildirilmesi için kanal tabanlı bir abone kalıbı kullanır.

**`Set(nextState)`**: Koşulsuz durum değişikliği. İş parçacığının kendisi (C geri çağrılarından) durum geçişlerini bildirmek için kullanır.

**`CompareAndSwap(compareTo, swapTo)`**: Atomik karşılaştır-ve-değiştir. Başlatma ilklendirmesi için kullanılır.

### İşleyici geçiş protokolü

Bir iş parçacığının işleyicisini değiştirmesi gerektiğinde (ör. inactive'den worker'a):

```text
Go side (setHandler)           C side (PHP thread)
─────────────────              ─────────────────
RequestSafeStateChange(
  TransitionRequested)
close(drainChan)
                               detects drain
                               Set(TransitionInProgress)
WaitFor(TransitionInProgress)
  → unblocked                  WaitFor(TransitionComplete)
handler = newHandler
drainChan = make(chan struct{})
Set(TransitionComplete)
                                 → unblocked
                               newHandler.beforeScriptExecution()
```

Bu protokol, işleyici işaretçisinin asla eşzamanlı okunup yazılmamasını sağlar.

### Worker yeniden başlatma protokolü

Worker'lar yeniden başlatıldığında (ör. admin API aracılığıyla):

```text
Go side (RestartWorkers)       C side (worker thread)
─────────────────              ─────────────────
RequestSafeStateChange(
  Restarting)
close(drainChan)
                               detects drain in waitForWorkerRequest()
                               returns false → PHP script exits
                               beforeScriptExecution():
                                 state is Restarting →
                                 Set(Yielding)
WaitFor(Yielding)
  → unblocked                    WaitFor(Ready, ShuttingDown)
drainChan = make(chan struct{})
Set(Ready)
                                 → unblocked
                               beforeScriptExecution() recurse:
                                 state is Ready → normal execution
```

## Go ile PHP arasındaki CGO sınırı

### Dışa aktarılan Go işlevleri

C kodu CGO dışa aktarımları üzerinden Go işlevlerini çağırır. Ana geri çağrılar şunlardır:

| İşlev                                       | Ne zaman çağrılır                                        |
| ------------------------------------------- | -------------------------------------------------------- |
| `go_frankenphp_before_script_execution`     | C döngüsü yürütülecek sonraki betiğe ihtiyaç duyduğunda  |
| `go_frankenphp_after_script_execution`      | PHP betiği yürütmeyi bitirdiğinde                        |
| `go_frankenphp_worker_handle_request_start` | Worker'ın `frankenphp_handle_request()`'i çağrıldığında  |
| `go_frankenphp_finish_worker_request`       | Worker istek işleyicisi döndüğünde                       |
| `go_ub_write`                               | PHP çıktı ürettiğinde (`echo` vb.)                       |
| `go_read_post`                              | PHP POST gövdesini okuduğunda (`php://input`)            |
| `go_read_cookies`                           | PHP çerezleri okuduğunda                                 |
| `go_write_headers`                          | PHP yanıt başlıklarını gönderdiğinde                     |
| `go_sapi_flush`                             | PHP çıktıyı boşalttığında                                |
| `go_log_attrs`                              | PHP yapılandırılmış bir mesaj kaydettiğinde              |

Tüm bu işlevler çağıran iş parçacığını tanımlayan bir `threadIndex` parametresi alır. Bu, iş parçacığı başlatılırken ayarlanan C'deki iş parçacığı yerel bir değişkendir (`__thread uintptr_t thread_index`).

### C iş parçacığı ana döngüsü

Her PHP iş parçacığı `frankenphp.c` içinde `php_thread()` çalıştırır:

```c
// frankenphp.c: php_thread() ana betik yürütme döngüsü
while ((scriptName = go_frankenphp_before_script_execution(thread_index))) {
    php_request_startup();
    php_execute_script(&file_handle);
    php_request_shutdown();
    go_frankenphp_after_script_execution(thread_index, exit_status);
}
```

Bailout'lar (ölümcül PHP hataları) `zend_catch` tarafından yakalanır; bu, iş parçacığını sağlıksız olarak işaretler ve temizliği zorlar.

### CGO sınırı boyunca bellek yönetimi

- **Go → C dizeleri**: `C.CString()` `malloc()` ile ayırır. Serbest bırakmak C tarafının sorumluluğundadır (ör. `frankenphp_free_request_context()` çerez verisini serbest bırakır).
- **Go dize sabitleme**: `phpThread` (`phpthread.go` içinde) Go'nun [`runtime.Pinner`](https://pkg.go.dev/runtime#Pinner)'ını gömer. `thread.Pin()` / `thread.Unpin()` C'den başvurulan Go belleğini kopyalamadan canlı tutar. İş parçacığı her betik yürütmesinden sonra serbest bırakılır.
- **PHP belleği**: Zend bellek yöneticisi (`emalloc`/`efree`) tarafından yönetilir. İstek kapanışında otomatik serbest bırakılır.

## FrankenPHP iş parçacığı otomatik ölçeklemesi

FrankenPHP, talebe göre PHP iş parçacığı sayısını otomatik ölçekleyebilir (`scaling.go`).

### Otomatik ölçekleme yapılandırması

- `num_threads`: Açılışta başlatılan başlangıç iş parçacığı sayısı
- `max_threads`: İzin verilen en fazla iş parçacığı sayısı (otomatik ölçeklenenler dahil)

### Yukarı ölçekleme

Ayrılmış bir goroutine tamponlanmamış bir `scaleChan`'den okur:

1. Bir istek işleyicisi müsait bir iş parçacığı bulamaz
2. İstek bağlamını `scaleChan`'e gönderir
3. Ölçekleme goroutine'i şunları kontrol eder:
   - İstek yeterince uzun süre takılı kaldı mı? (en az 5 ms)
   - CPU kullanımı eşiğin altında mı? (%80)
   - İş parçacığı sınırına ulaşıldı mı?
4. Tüm kontroller geçerse yeni bir iş parçacığı başlatılır ve atanır

### Aşağı ölçekleme

Ayrı bir goroutine periyodik olarak (her 5 sn) boştaki otomatik ölçeklenmiş iş parçacıklarını kontrol eder. `Ready` durumunda `maxIdleTime`'dan (varsayılan 5 sn) daha uzun süre boşta kalan iş parçacıkları `Inactive`'e dönüştürülür (döngü başına en fazla 10). Tamamen durdurulmazlar: bunun için bir kod yolu vardır ancak şu anda devre dışıdır; çünkü bazı PECL eklentileri bellek sızdırır ve iş parçacıklarının temiz kapanmasını engeller.

## İş parçacığı başına ortam sanal alanı

FrankenPHP ortam değişkenlerini iş parçacığı başına sanal alana alır:

1. Başlangıçta ana iş parçacığı `os.Environ()`'u `main_thread_env` (bir PHP `HashTable`) içine anlık görüntüler.
2. `$_SERVER`, `main_thread_env`'in bir kopyası artı isteğe özgü değişkenlerden oluşturulur (`frankenphp_register_server_vars` içinde). Her istek için, worker betiğinin her yinelemesi dahil, yeniden kurulur.
3. `$_ENV` aynı anlık görüntüden PHP'nin `php_import_environment_variables` kancası aracılığıyla doldurulur. Düzenli modda bu her betik yürütmesinde bir kez olur; worker modunda worker betiği başladığında bir kez olur ve worker istekleri arasında **yeniden kurulmaz**; bu yüzden `$_ENV` yazmaları istekler arasında sızar (bkz. [Worker modu](worker.md)).
4. `frankenphp_putenv()` / `frankenphp_getenv()` `main_thread_env`'den tembel başlatılan iş parçacığı yerel `sandboxed_env` üzerinde çalışır; global C ortamındaki yarış koşullarını önler.
5. `reset_sandboxed_environment()` her PHP betik yürütmesinden sonra `sandboxed_env`'i serbest bırakır. Düzenli modda bu istek başınadır; worker modunda yalnızca worker betiğinin kendisi çıktığında çalışır, bu yüzden `putenv()` yazmaları betik yeniden başlayana kadar aynı iş parçacığındaki sonraki worker isteklerinde görünür.

## İstek akışı (düzenli mod)

1. HTTP isteği Caddy'ye gelir
2. FrankenPHP'nin Caddy modülü PHP betik yolunu çözer
3. İstek ve betik bilgisiyle bir `frankenPHPContext` oluşturulur
4. Bağlam `requestChan` üzerinden müsait bir düzenli iş parçacığına gönderilir
5. İş parçacığının `beforeScriptExecution()`'ı betik dosya adını döndürür
6. C katmanı PHP betiğini yürütür
7. Yürütme sırasında Go geri çağrıları G/Ç'yi işler (`go_ub_write`, `go_read_post` vb.)
8. Yürütmeden sonra `afterScriptExecution()` tamamlanmayı bildirir
9. Yanıt istemciye gönderilir

## İstek akışı (worker modu)

1. HTTP isteği Caddy'ye gelir
2. FrankenPHP'nin Caddy modülü bu istek için worker'ı çözer
3. Bir `frankenPHPContext` oluşturulur
4. Bağlam worker'ın `requestChan`'ine veya belirli bir iş parçacığının `requestChan`'ine gönderilir
5. Worker iş parçacığının `waitForWorkerRequest()`'i onu alır
6. PHP'nin `frankenphp_handle_request()` geri çağrısı tetiklenir
7. Geri çağrı döndükten sonra `go_frankenphp_finish_worker_request()` temizler
8. Worker `waitForWorkerRequest()`'e döner
