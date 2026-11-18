# Gözlemlenebilirlik

FrankenPHP yerleşik gözlemlenebilirlik özellikleri sunar: [Prometheus uyumlu metrikler](metrics.md) ve [yapılandırılmış günlük kaydı](logging.md).
Bu özellikler, aşağıda önerilen araçlarla birlikte, PHP uygulamanızın geliştirme ve üretimdeki davranışına tam görünürlük sağlar.

## Ember TUI ve Prometheus dışa aktarıcı

[Ember](https://github.com/alexandre-daubois/ember), FrankenPHP'yi izlemenin en kullanıcı dostu yoludur.

Caddy'nin admin API'sine bağlanır ve FrankenPHP ile derinlemesine entegre olur; sıfır yapılandırma ve harici altyapı olmadan gerçek zamanlı görünürlük sağlar.

Hem geliştirmede hem üretimde kullanılmak üzere tasarlanmıştır: yerel kullanım için bir TUI panosu ve üretim izlemesi için bir Prometheus dışa aktarma daemon modu vardır.

> [!TIP]
> Özelliklerin tam listesi ve kurulum ayrıntıları için [Ember belgelerine](https://github.com/alexandre-daubois/ember) bakın.

## Metrikler

[Caddy metrikleri](https://caddyserver.com/docs/metrics) etkinleştirildiğinde FrankenPHP; iş parçacıkları, worker'lar, istek işleme ve kuyruk derinliği için Prometheus uyumlu metrikler yayınlar.

Kullanılabilir metriklerin tam listesi için [Metrikler](metrics.md) sayfasına bakın.

## Günlük kaydı

FrankenPHP, Caddy'nin günlük sistemine entegre olur ve şiddet düzeyi ile bağlam verisi içeren yapılandırılmış günlükler için `frankenphp_log()` sağlar; bu da Datadog, Grafana Loki veya Elastic gibi platformlara aktarımı kolaylaştırır.

Kullanım ayrıntıları için [Günlük kaydı](logging.md) sayfasına bakın.

## Özel Prometheus/Grafana kurulumu

Özel bir izleme yığını tercih ediyorsanız FrankenPHP metriklerini doğrudan toplayabilirsiniz.
İki seçenek vardır:

1. **Caddy'yi doğrudan tarayın**: Caddy, metrikleri admin uç noktasında yayınlar (varsayılan: `localhost:2019/metrics`)
2. **Ember üzerinden tarayın**: Ember `--expose` ile çalışırken FrankenPHP metriklerini, Caddy verisinden türetilen hesaplanmış metriklerle (RPS, gecikme yüzdelikleri, hata oranları) birlikte özel bir uç noktada yayınlar.
