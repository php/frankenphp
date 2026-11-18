# FrankenPHP'yi bir Go kütüphanesi olarak kullanma

FrankenPHP yalnızca bir Caddy modülü değildir: PHP betiklerini `net/http` ile çalıştırmak için herhangi bir Go programına kütüphane olarak gömülebilir.

Derleme gereksinimleri [FrankenPHP'yi kaynaktan derlemek](compile.md) ile aynıdır: embed SAPI ve ZTS etkin bir PHP ve eşleşen CGO bayrakları.

## Başlarken

Bir `Server` oluşturun, FrankenPHP'yi başlatırken kaydedin ve bir `http.Handler` olarak kullanın:

Minimal bir örnek için [https://pkg.go.dev](https://pkg.go.dev/github.com/dunglas/frankenphp#example-ServeHTTP) adresine bakın.

`NewServer()`; worker'ları, metrikleri ve günlükleri bu sunucuya atamak için kullanılan insan tarafından okunabilir bir ad (boşsa kayıtta `server_<idx>` olur), belge kökü, yol ayırma sonekleri (varsayılan `[".php"]`), her isteğe sunulan ortam değişkenleri ve bir `*slog.Logger` (varsayılan olarak global logger) alır.

`Init()` PHP çalışma zamanını başlatır ve istek sunulmadan önce tam olarak bir kez çağrılmalıdır; `Shutdown()` onu durdurur. `Init()` öncesi veya `Shutdown()` sonrası `Server.ServeHTTP()` çağrısı `ErrNotRunning` döndürür. Aynı `*Server`, örneğin yapılandırmayı yeniden yüklemek için, bir `Shutdown()` sonrasında yeniden `Init()`'e geçilebilir.

## Birden fazla sunucu

Birden fazla sunucu aynı anda kaydedilebilir; her birinin kendi belge kökü, ortamı ve logger'ı vardır. Bu, Caddyfile'daki birden fazla `php_server` bloğunun yaptığını yansıtır:

```go
// Ayrı belge köklerine sahip iki sunucu kaydetme
api, _ := frankenphp.NewServer("api/public/")
admin, _ := frankenphp.NewServer("admin/public/")

err := frankenphp.Init(
	frankenphp.WithServer(api),
	frankenphp.WithServer(admin),
)
```

`api.ServeHTTP()` üzerinden sunulan istekler yalnızca o sunucunun yapılandırmasını (ve aşağıda görüleceği gibi worker'larını) görür.

## Worker'lar

[Worker betikleri](worker.md) `WithWorkers()` ile bildirilir. Bir worker `WithWorkerServerScope()` ile bir sunucuya kapsamlandırılabilir: yalnızca bu sunucu örneğinin işlediği istekler worker'a ulaşır. İstekler betik yoluna veya `WithWorkerMatcher()` ile kaydedilen özel bir eşleştiriciye göre eşleştirilir:

```go
// Worker'ları bir sunucuya kapsamlandırma
server, _ := frankenphp.NewServer("public/")

err := frankenphp.Init(
	frankenphp.WithServer(server),
	frankenphp.WithWorkers("app", "public/index.php", 4,
		frankenphp.WithWorkerServerScope(server),
	),
	frankenphp.WithWorkers("api", "public/api.php", 2,
		frankenphp.WithWorkerServerScope(server),
		frankenphp.WithWorkerMatcher(func(r *http.Request) bool {
			return strings.HasPrefix(r.URL.Path, "/api/")
		}),
	),
)
```

Sunucu kapsamı olmadan bildirilen worker'lar globaldir: herhangi bir sunucuda dosya yoluna göre eşleşirler. Global bir worker'ın eşleştireceği belirli bir istek kümesi olmadığından `WithWorkerMatcher()` ile global bir worker'ı birleştirmek bir yapılandırma hatasıdır ve `Init()` bunu reddeder.

## İstek başına seçenekler

`Server.ServeHTTP()`, tek bir istek için sunucu yapılandırmasını geçersiz kılmak üzere `RequestOption`'lar kabul eder; örn. `WithRequestDocumentRoot()`, `WithRequestSplitPath()`, `WithRequestEnv()` veya `WithRequestLogger()`.

## Server öncesi API ile uyumluluk

Paket düzeyindeki `frankenphp.ServeHTTP()` işlevi herhangi bir sunucu kaydetmeden çalışmaya devam eder: `frankenphp.NewRequestWithContext()` ile hazırlanan istekler, global yapılandırmayı taşıyan dahili bir yedek sunucuda yürütülür. Yeni kod açık `Server` örneklerini tercih etmelidir.
