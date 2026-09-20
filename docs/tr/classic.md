# Classic modu kullanma

Ek bir yapılandırma olmadan FrankenPHP klasik modda çalışır. Bu modda FrankenPHP, PHP dosyalarını doğrudan sunan geleneksel bir PHP sunucusu gibi davranır. Bu da onu PHP-FPM veya Apache + mod_php için kesintisiz bir yerine geçen çözüm haline getirir.

Caddy'ye benzer şekilde FrankenPHP sınırsız sayıda bağlantı kabul eder ve bunları [sabit sayıda iş parçacığı](config.md#caddyfile-config) ile karşılar. Kabul edilen ve kuyruğa alınan bağlantı sayısı yalnızca mevcut sistem kaynaklarıyla sınırlıdır.
PHP iş parçacığı havuzu, başlangıçta sabit sayıda iş parçacığıyla çalışır; bu, PHP-FPM'nin static moduna benzer. İş parçacıklarının [çalışma zamanında otomatik ölçeklenmesine](performance.md#max_threads) izin vermek de mümkündür; bu da PHP-FPM'nin dynamic moduna benzer.

Kuyruktaki bağlantılar, bir PHP iş parçacığı müsait olana kadar süresiz bekler. Bunu önlemek için FrankenPHP'nin genel yapılandırmasındaki max_wait_time [ayarını](config.md#caddyfile-config) kullanarak, bir isteğin boş bir PHP iş parçacığı için reddedilmeden önce ne kadar bekleyebileceğini sınırlayabilirsiniz.
Ayrıca Caddy'de makul bir [yazma zaman aşımı](https://caddyserver.com/docs/caddyfile/options#timeouts) ayarlayabilirsiniz.

Her Caddy örneği yalnızca bir FrankenPHP iş parçacığı havuzu başlatır; bu havuz tüm `php_server` blokları arasında paylaşılır.
