# Klasik Modu Kullanma

Herhangi bir ek yapılandırma olmadan, FrankenPHP klasik modda çalışır. Bu modda, FrankenPHP geleneksel bir PHP sunucusu gibi işlev görür ve PHP dosyalarını doğrudan sunar. Bu da onu PHP-FPM veya mod_php'li Apache için sorunsuz bir doğrudan ikame haline getirir.

Caddy'ye benzer şekilde, FrankenPHP sınırsız sayıda bağlantıyı kabul eder ve bunları sunmak için [sabit sayıda iş parçacığı](config.md#caddyfile-config) kullanır. Kabul edilen ve kuyruğa alınan bağlantıların sayısı yalnızca mevcut sistem kaynaklarıyla sınırlıdır.
PHP iş parçacığı havuzu, başlangıçta başlatılan sabit sayıda iş parçacığıyla çalışır, bu da PHP-FPM'nin statik moduna benzer. Ayrıca, PHP-FPM'nin dinamik moduna benzer şekilde, iş parçacıklarının [çalışma zamanında otomatik olarak ölçeklenmesini](performance.md#max_threads) sağlamak da mümkündür.

Kuyruğa alınan bağlantılar, bir PHP iş parçacığı hizmet vermeye hazır olana kadar süresiz olarak bekleyecektir. Bunu önlemek için, bir isteğin reddedilmeden önce boş bir PHP iş parçacığı için bekleme süresini sınırlamak amacıyla FrankenPHP'nin genel yapılandırmasındaki max_wait_time [yapılandırmasını](config.md#caddyfile-config) kullanabilirsiniz.
Ek olarak, Caddy'de makul bir [yazma zaman aşımı](https://caddyserver.com/docs/caddyfile/options#timeouts) ayarlayabilirsiniz.

Her Caddy örneği yalnızca bir FrankenPHP iş parçacığı havuzu başlatır ve bu havuz tüm `php_server` blokları arasında paylaşılır.
