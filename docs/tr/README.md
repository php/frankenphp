# FrankenPHP: PHP için Modern Uygulama Sunucusu

<h1 align="center"><a href="https://frankenphp.dev"><img src="../../frankenphp.png" alt="FrankenPHP" width="600"></a></h1>

FrankenPHP, [Caddy](https://caddyserver.com/) web sunucusunun üzerine inşa edilmiş PHP için modern bir uygulama sunucusudur.

FrankenPHP, çarpıcı özellikleri sayesinde PHP uygulamalarınıza süper güçler kazandırır: [Early Hints\*](https://frankenphp.dev/docs/early-hints/), [worker modu](https://frankenphp.dev/docs/worker/), [real-time yetenekleri](https://frankenphp.dev/docs/mercure/), otomatik HTTPS, HTTP/2 ve HTTP/3 desteği...

FrankenPHP herhangi bir PHP uygulaması ile çalışır ve worker modu ile resmi entegrasyonları sayesinde Laravel ve Symfony projelerinizi her zamankinden daha performanslı hale getirir.

FrankenPHP, PHP'yi `net/http` kullanarak herhangi bir uygulamaya yerleştirmek için bağımsız bir Go kütüphanesi olarak da kullanılabilir.

[_Frankenphp.dev_](https://frankenphp.dev) adresinden ve bu slayt üzerinden daha fazlasını öğrenin:

<a href="https://dunglas.dev/2022/10/frankenphp-the-modern-php-app-server-written-in-go/"><img src="https://dunglas.dev/wp-content/uploads/2022/10/frankenphp.png" alt="Slides" width="600"></a>

## Başlarken

Windows üzerinde FrankenPHP çalıştırmak için [WSL](https://learn.microsoft.com/windows/wsl/) kullanın.

### Kurulum Betiği

Platformunuza uygun sürümü otomatik olarak kurmak için bu satırı terminalinize kopyalayabilirsiniz:

```console
curl https://frankenphp.dev/install.sh | sh
```

### Binary Çıktısı

Docker kullanmayı tercih etmiyorsanız, Linux ve macOS için geliştirme amaçlı bağımsız (statik) FrankenPHP binary dosyaları sağlıyoruz;
[PHP 8.4](https://www.php.net/releases/8.4/en.php) ve en popüler PHP eklentilerinin çoğu dahildir.

[FrankenPHP'yi indirin](https://github.com/php/frankenphp/releases)

**Eklenti kurulumu:** Yaygın eklentiler paketle birlikte gelir. Daha fazla eklenti yüklemek mümkün değildir.

### rpm Paketleri

Bakımcılarımız `dnf` kullanan tüm sistemler için rpm paketleri sunuyor. Kurulum için:

```console
sudo dnf install https://rpm.henderkes.com/static-php-1-0.noarch.rpm
sudo dnf module enable php-zts:static-8.4 # 8.2-8.5 mevcut
sudo dnf install frankenphp
```

**Eklenti kurulumu:** `sudo dnf install php-zts-<extension>`

Varsayılan olarak mevcut olmayan eklentiler için [PIE](https://github.com/php/pie) kullanın:

```console
sudo dnf install pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

### deb Paketleri

Bakımcılarımız `apt` kullanan tüm sistemler için deb paketleri sunuyor. Kurulum için:

```console
sudo curl -fsSL https://key.henderkes.com/static-php.gpg -o /usr/share/keyrings/static-php.gpg && \
echo "deb [signed-by=/usr/share/keyrings/static-php.gpg] https://deb.henderkes.com/ stable main" | sudo tee /etc/apt/sources.list.d/static-php.list && \
sudo apt update
sudo apt install frankenphp
```

**Eklenti kurulumu:** `sudo apt install php-zts-<extension>`

Varsayılan olarak mevcut olmayan eklentiler için [PIE](https://github.com/php/pie) kullanın:

```console
sudo apt install pie-zts
sudo pie-zts install asgrim/example-pie-extension
```

### Docker

```console
docker run -v $PWD:/app/public \
    -p 80:80 -p 443:443 -p 443:443/udp \
    dunglas/frankenphp
```

`https://localhost` adresine gidin ve keyfini çıkarın!

> [!TIP]
>
> `https://127.0.0.1` kullanmaya çalışmayın. `https://localhost` kullanın ve kendinden imzalı sertifikayı kabul edin.
> Kullanılacak alan adını değiştirmek için [`SERVER_NAME` ortam değişkenini](https://frankenphp.dev/tr/docs/config#ortam-değişkenleri) kullanın.

### Homebrew

FrankenPHP, macOS ve Linux için [Homebrew](https://brew.sh) paketi olarak da mevcuttur.

```console
brew install dunglas/frankenphp/frankenphp
```

**Eklenti kurulumu:** [PIE](https://github.com/php/pie) kullanın.

### Kullanım

Geçerli dizinin içeriğini sunmak için çalıştırın:

```console
frankenphp php-server
```

Komut satırı betiklerini şu şekilde çalıştırabilirsiniz:

```console
frankenphp php-cli /path/to/your/script.php
```

deb ve rpm paketleri için systemd servisini de başlatabilirsiniz:

```console
sudo systemctl start frankenphp
```

## Docs

- [Worker modu](worker.md)
- [Early Hints desteği (103 HTTP durum kodu)](early-hints.md)
- [Real-time](mercure.md)
- [Konfigürasyon](config.md)
- [Docker imajları](docker.md)
- [Production'a dağıtım](production.md)
- [**Bağımsız** kendiliğinden çalıştırılabilir PHP uygulamaları oluşturma](embed.md)
- [Statik binary'leri oluşturma](static.md)
- [Kaynak dosyalarından derleme](config.md)
- [Laravel entegrasyonu](laravel.md)
- [Bilinen sorunlar](known-issues.md)
- [Demo uygulama (Symfony) ve kıyaslamalar](https://github.com/dunglas/frankenphp-demo)
- [Go kütüphane dokümantasonu](https://pkg.go.dev/github.com/dunglas/frankenphp)
- [Katkıda bulunma ve hata ayıklama](CONTRIBUTING.md)

## Örnekler ve İskeletler

- [Symfony](https://github.com/dunglas/symfony-docker)
- [API Platform](https://api-platform.com/docs/distribution/)
- [Laravel](https://frankenphp.dev/docs/laravel/)
- [Sulu](https://sulu.io/blog/running-sulu-with-frankenphp)
- [WordPress](https://github.com/StephenMiracle/frankenwp)
- [Drupal](https://github.com/dunglas/frankenphp-drupal)
- [Joomla](https://github.com/alexandreelise/frankenphp-joomla)
- [TYPO3](https://github.com/ochorocho/franken-typo3)
- [Magento2](https://github.com/ekino/frankenphp-magento2)
