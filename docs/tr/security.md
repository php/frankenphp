# Güvenlik modeli

Bu belge FrankenPHP'nin güven modelini açıklar: hangi girdilerin güvenilir olduğu, hangilerinin olmadığı ve aralarındaki sınırın nerede durduğu.
Amaç, güvenlik denetimlerinin ve otomatik tarayıcıların **FrankenPHP'nin kendisini** (sunduğu PHP uygulamalarını değil) değerlendirmesine yardımcı olmaktır.

Burada atıfta bulunulan iç mekanikler (iş parçacıkları, CGO sınırı, ortam sanal alanı) için [İç yapı](internals.md) belgesine bakın.
Uzun ömürlü süreçlerde durum kalıcılığı için [Worker modu](worker.md) belgesine bakın.

## Güven sınırları

FrankenPHP dört ayrı aktörden oluşan bir yığında çalışır:

| Aktör                    | Güven                      | Notlar                                                                                                                                                         |
| ------------------------ | -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Uzak istemci**         | Güvenilmez                 | HTTP isteği (metod, URI, başlıklar, çerezler, gövde, yüklemeler) kirli girdinin birincil kaynağıdır.                                                           |
| **Operatör**             | Güvenilir                  | Dağıtım yapılandırmasını sağlar: `Caddyfile`, ortam değişkenleri, `php.ini`, yüklü PHP eklentileri ve Caddy modülleri, ve uygulamanın kendi kodu.               |
| **PHP uygulama kodu**    | *Kaynağı itibarıyla* güvenilir | Operatör tarafından dağıtılır, bu yüzden FrankenPHP saldırganın sağladığı kodu asla çalıştırmaz; ancak bu kod güvenilmez istek verisini *tüketir*.            |
| **FrankenPHP (Go + C)**  | Güvenilir hesaplama tabanı | PHP'yi gömer, veriyi içeri ve dışarı taşır, istekleri ve iş parçacıklarını yalıtır. Bu belgenin kapsamı kendi kusurlarıdır.                                     |

## Kod kaynağı ve veri kirliliği

Bu, en önemli ayrımdır ve "PHP'ye güveniyor muyuz yoksa güvenmiyor muyuz?" kafa karışıklığının çoğunu çözer:

- **Kod kaynağı güvenilirdir.** FrankenPHP yalnızca operatörün dağıttığı PHP dosyalarını çalıştırır: belge kökü altında çözümlenen betik veya yapılandırılmış worker betiği. İsteğin kendisinde taşınan kodu (gövde, sorgu dizesi, başlıklar) asla değerlendirmez: SAPI sınırı *veri* taşır, güvenilmez *kod* değil.
- **İstek verisi kirlidir.** PHP kodunun istekten okuduğu her şey (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `php://input`) herhangi bir PHP SAPI'sinde olduğu gibi güvenilmezdir. Bunu arındırmak uygulamanın işidir.

Dolayısıyla "PHP'den gelene güveniyoruz" *kod* için doğru iken "SAPI güvenilmez girdi taşır" *veri* için doğrudur: ikisi çelişmez.
FrankenPHP'nin işi bu kirli veriyi sadakatle taşımak ve bir isteğin verisinin diğerine sızmasını önlemektir.

## FrankenPHP'nin sorumlulukları

Güvenilir hesaplama tabanının üç işi vardır. FrankenPHP'deki güvenlik kusurları bunlardan birinde yaşar:

1. **Sadık taşıma**: isteği PHP süper küresellerine ve `php://input`'a eşle, PHP'nin çıktısını ve başlıklarını istemciye geri taşı; enjeksiyon (başlık/CRLF enjeksiyonu, istek kaçırma, yanlış dosyanın çalıştırılması) eklemeden.
2. **Yalıtım**: istek kapsamlı durumun istekler, PHP iş parçacıkları ve worker yinelemeleri arasında geçmesini önle.
3. **Bellek güvenliği**: CGO sınırını (Go ↔ C/PHP) belleği bozmadan yönet.

## Kapsamda: FrankenPHP'nin kendi saldırı yüzeyi

Bunlar FrankenPHP'nin sahip olduğu yüzeylerdir. Buradaki bir zafiyet FrankenPHP zafiyetidir:

- **İstekten süper küresele eşleme** (`cgi.go`, `frankenphp_register_server_vars`): `$_SERVER`, `REMOTE_ADDR`, `SCRIPT_NAME`, `PATH_INFO` ve diğer CGI değişkenlerini istekten oluşturma.
- **PHP betik yolu çözümlemesi**: istek yolu `split_path` (varsayılan `.php`) ile `SCRIPT_NAME` / `PATH_INFO` olarak ayrılır, ardından `sanitizedPathJoin` (`filepath.Join(root, filepath.Clean("/"+reqPath))`) ile belge köküne birleştirilir; bu `SCRIPT_FILENAME`'in belge kökünden kaçmasını (path traversal) önler. `php_server` yönergesi ayrıca istekleri mevcut dosyalara veya ön denetleyiciye yönlendiren varsayılan bir `try_files` yeniden yazması ayarlar; böylece PHP-FPM'nin klasik tuzaklarından biri olan yanlış dosyanın çalıştırılması azaltılır.
- **Worker modu durum yalıtımı**: FrankenPHP istekler arasında `$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER` ve `$_REQUEST`'i sıfırlar ve `$_SESSION`'ı açıkça temizler (aksi halde istekler arasında sızardı), ancak **`$_ENV` sıfırlanmaz**; `putenv()` yazmaları, `static` değişkenler, sınıf statik özellikleri ve globaller aynı iş parçacığındaki istekler boyunca kalır. Bu durumda bırakılan istek veya kullanıcıya özgü veri sonraki bir isteğe sızabilir (bkz. [Worker modu](worker.md#state-persistence)).
- **İş parçacığı başına ortam sanal alanı**: `frankenphp_putenv()` / `frankenphp_getenv()` iş parçacığı yerel `sandboxed_env` üzerinde çalışır, böylece eşzamanlı iş parçacıkları global C ortamında yarışmaz (bkz. [İç yapı](internals.md#per-thread-environment-sandboxing)).
- **CGO bellek sınırı**: Go dize sabitleme ve Go ↔ C sınırında `C.CString()` / `free()` ömürleri.
- **Caddy admin API**: `/frankenphp/workers/restart` ve `/frankenphp/threads` uç noktaları, Caddy'nin admin API'si üzerinden sunulur (varsayılan olarak `localhost:2019` dinler). Bu uç noktayı localhost dışına açmak operatör kararıdır.
- **Güvenilir vekil işleme**: gelen `X-Forwarded-*` başlıkları her zaman PHP'ye kirli `$_SERVER['HTTP_X_FORWARDED_*']` değerleri olarak ulaşır; gerçek istemci IP'sini ve şemayı türetmek için yalnızca [`trusted_proxies`](production.md#running-behind-a-reverse-proxy) yapılandırıldığında güvenilir kabul edilirler.
- **Yavaş istek gövdeleri**: bir gövde duyurup ardından damla damla gönderen veya takılan bir istemci, işleyen iş parçacığını süre boyunca tutar. Sınırlı bir iş parçacığı havuzuyla yeterince böyle bağlantı havuzu tüketir (yavaş-POST DoS). FrankenPHP gövde okumalarına varsayılan olarak 60 sn boşta kalma zaman aşımı uygular ([`request_body_timeout`](config.md#caddyfile-config)); her okumadan önce son tarihi sıfırlar, böylece her boyutta istikrarlı bir yükleme başarılı olurken takılan bir yükleme kesilir ve iş parçacığı serbest bırakılır.

## Kapsam dışı

- **Uygulamanın PHP kodundaki zafiyetler** (SQL enjeksiyonu, XSS, güvensiz serileştirme vb.). FrankenPHP güvenilmez istek verisini uygulamaya değiştirmeden iletir; buna karşı savunma, herhangi bir SAPI'de olduğu gibi uygulamanın sorumluluğudur.
- **FrankenPHP'nin kullandığı üst bileşenlerdeki kusurlar** (PHP, Caddy, Go) veya üzerine inşa edilen projelerdeki kusurlar (Laravel Octane, Symfony Runtime). Bunları ilgili projeye bildirin.

## Bir zafiyet bildirme

FrankenPHP'yi etkileyen bir güvenlik sorununu nasıl bildireceğiniz için [`SECURITY.md`](../../SECURITY.md) dosyasına bakın.
