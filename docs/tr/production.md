# Production Ortamına Dağıtım

Bu eğitimde, Docker Compose kullanarak bir PHP uygulamasını tek bir sunucuya nasıl dağıtacağımızı öğreneceğiz.

Symfony kullanıyorsanız, Symfony Docker projesinin (FrankenPHP kullanan) "[Production ortamına dağıtım](https://github.com/dunglas/symfony-docker/blob/main/docs/production.md)" belgesini okumayı tercih edebilirsiniz.

API Platform kullanıyorsanız (o da FrankenPHP kullanır), [çerçevenin dağıtım belgesine](https://api-platform.com/docs/deployment/) başvurabilirsiniz.

## Uygulamanızı Hazırlama

İlk olarak, PHP projenizin kök dizininde bir `Dockerfile` oluşturun:

```dockerfile
FROM dunglas/frankenphp

# "your-domain-name.example.com" yerine kendi alan adınızı yazdığınızdan emin olun
ENV SERVER_NAME=your-domain-name.example.com
# HTTPS'yi devre dışı bırakmak istiyorsanız, bunun yerine bu değeri kullanın:
#ENV SERVER_NAME=:80

# Projeniz "public" dizinini web kökü olarak kullanmıyorsa, bunu burada ayarlayabilirsiniz:
# ENV SERVER_ROOT=web/

# PHP production ayarlarını etkinleştirin
RUN mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini"

# Projenizin PHP dosyalarını genel dizine kopyalayın
COPY . /app/public
# Symfony veya Laravel kullanıyorsanız, bunun yerine tüm projeyi kopyalamanız gerekir:
#COPY . /app
```

Daha fazla ayrıntı ve seçenek için "[Özel Docker İmajı Oluşturma](docker.md)" bölümüne bakın,
ve yapılandırmayı nasıl özelleştireceğinizi, PHP eklentilerini ve Caddy modüllerini nasıl kuracağınızı öğrenmek için.

Projeniz Composer kullanıyorsa,
Docker imajına dahil ettiğinizden ve bağımlılıklarınızı yüklediğinizden emin olun.

Ardından, bir `compose.yaml` dosyası ekleyin:

```yaml
services:
  php:
    image: dunglas/frankenphp
    restart: always
    ports:
      - "80:80" # HTTP
      - "443:443" # HTTPS
      - "443:443/udp" # HTTP/3
    volumes:
      - caddy_data:/data
      - caddy_config:/config

# Caddy sertifikaları ve yapılandırması için gereken birimler
volumes:
  caddy_data:
  caddy_config:
```

> [!NOTE]
>
> Önceki örnekler production kullanımı için tasarlanmıştır.
> Geliştirme aşamasında, bir birim (volume), farklı bir PHP yapılandırması ve `SERVER_NAME` ortam değişkeni için farklı bir değer kullanmak isteyebilirsiniz.
>
> Çok aşamalı imajlar, Composer, ekstra PHP eklentileri vb. kullanan daha gelişmiş bir örnek için [Symfony Docker](https://github.com/dunglas/symfony-docker) projesine (FrankenPHP kullanır) bir göz atın.

Son olarak, eğer Git kullanıyorsanız, bu dosyaları commit edin ve push edin.

## Sunucu Hazırlama

Uygulamanızı production ortamına dağıtmak için bir sunucuya ihtiyacınız vardır.
Bu eğitimde, DigitalOcean tarafından sağlanan bir sanal makine kullanacağız, ancak herhangi bir Linux sunucusu çalışabilir.
Docker yüklü bir Linux sunucunuz varsa, doğrudan [bir sonraki bölüme](#configuring-a-domain-name) geçebilirsiniz.

Aksi takdirde, 200 $ ücretsiz kredi almak için [bu ortaklık bağlantısını](https://m.do.co/c/5d8aabe3ab80) kullanın, bir hesap oluşturun ve ardından "Create a Droplet" seçeneğine tıklayın.
Ardından, "Bir imaj seçin" bölümünün altındaki "Marketplace" sekmesine tıklayın ve "Docker" adlı uygulamayı bulun.
Bu, Docker ve Docker Compose'un en son sürümlerinin zaten yüklü olduğu bir Ubuntu sunucusu sağlayacaktır!

Test amaçlı kullanım için en ucuz planlar yeterli olacaktır.
Gerçek production kullanımı için, muhtemelen ihtiyaçlarınıza uyacak şekilde "genel amaçlı" bölümünden bir plan seçmek isteyeceksiniz.

![DigitalOcean'da Docker ile FrankenPHP Dağıtımı](digitalocean-droplet.png)

Diğer ayarlar için varsayılanları koruyabilir veya ihtiyaçlarınıza göre değiştirebilirsiniz.
SSH anahtarınızı eklemeyi veya bir parola oluşturmayı unutmayın, ardından "Finalize and create" düğmesine basın.

Ardından, Droplet'iniz hazırlanırken birkaç saniye bekleyin.
Droplet'iniz hazır olduğunda, bağlanmak için SSH kullanın:

```console
ssh root@<droplet-ip>
```

## Alan Adı Yapılandırma

Çoğu durumda sitenizle bir alan adını ilişkilendirmek isteyeceksiniz.
Henüz bir alan adınız yoksa, bir kayıt şirketi aracılığıyla bir alan adı satın almanız gerekir.

Daha sonra alan adınız için sunucunuzun IP adresini işaret eden `A` türünde bir DNS kaydı oluşturun:

```dns
your-domain-name.example.com.  IN  A     207.154.233.113
```

DigitalOcean Alan Adları hizmetiyle ilgili örnek ("Networking" > "Domains"):

![DigitalOcean'da DNS Yapılandırma](digitalocean-dns.png)

> [!NOTE]
>
> FrankenPHP tarafından varsayılan olarak otomatik olarak TLS sertifikası oluşturmak için kullanılan hizmet olan Let's Encrypt, sadece IP adreslerinin kullanılmasını desteklemez. Let's Encrypt'i kullanmak için alan adı kullanmak zorunludur.

## Dağıtım

Projenizi `git clone`, `scp` veya ihtiyacınıza uygun başka bir araç kullanarak sunucuya kopyalayın.
GitHub kullanıyorsanız [bir dağıtım anahtarı](https://docs.github.com/en/free-pro-team@latest/developers/overview/managing-deploy-keys#deploy-keys) kullanmak isteyebilirsiniz.
Dağıtım anahtarları ayrıca [GitLab tarafından desteklenir](https://docs.gitlab.com/ee/user/project/deploy_keys/).

Git ile örnek:

```console
git clone git@github.com:<username>/<project-name>.git
```

Projenizi içeren dizine gidin (`<project-name>`) ve uygulamayı production modunda başlatın:

```console
docker compose up --wait
```

Sunucunuz hazır ve çalışıyor, ve sizin için otomatik olarak bir HTTPS sertifikası oluşturuldu.
`https://your-domain-name.example.com` adresine gidin ve keyfini çıkarın!

> [!CAUTION]
>
> Docker bir önbellek katmanına sahip olabilir, her dağıtım için doğru derlemeye sahip olduğunuzdan emin olun veya önbellek sorununu önlemek için projenizi `--no-cache` seçeneği ile yeniden oluşturun.

## Birden Fazla Düğümde Dağıtım

Uygulamanızı bir makine kümesine dağıtmak istiyorsanız, [Docker Swarm](https://docs.docker.com/engine/swarm/stack-deploy/) kullanabilirsiniz,
sağlanan Compose dosyaları ile uyumludur.
Kubernetes üzerinde dağıtım yapmak için, FrankenPHP kullanan [API Platform ile sunulan Helm grafiğine](https://api-platform.com/docs/deployment/kubernetes/) göz atın.
