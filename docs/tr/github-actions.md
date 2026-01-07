# GitHub Actions Kullanma

Bu depo, her onaylanmış çekme isteğinde veya kurulum yapıldıktan sonra kendi çatalınızda Docker imajını [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) üzerine inşa eder ve dağıtır.

## GitHub Actions'ı Ayarlama

Depo ayarlarında, gizliler altında aşağıdaki gizli değerleri ekleyin:

- `REGISTRY_LOGIN_SERVER`: Kullanılacak Docker kayıt defteri (örn. `docker.io`).
- `REGISTRY_USERNAME`: Kayıt defterine giriş yapmak için kullanılacak kullanıcı adı (örn. `dunglas`).
- `REGISTRY_PASSWORD`: Kayıt defterine giriş yapmak için kullanılacak parola (örn. bir erişim anahtarı).
- `IMAGE_NAME`: İmajın adı (örn. `dunglas/frankenphp`).

## İmajı İnşa Etme ve Gönderme

1. Bir çekme isteği oluşturun veya kendi çatalınıza (fork) gönderin.
2. GitHub Actions imajı inşa edecek ve tüm testleri çalıştıracaktır.
3. İnşa başarılı olursa, imaj `pr-x` (burada `x` PR numarasıdır) etiketi kullanılarak kayıt defterine gönderilecektir.

## İmajı Dağıtma

1. Çekme isteği birleştirildikten sonra, GitHub Actions testleri tekrar çalıştıracak ve yeni bir imaj inşa edecektir.
2. İnşa başarılı olursa, `main` etiketi Docker kayıt defterinde güncellenecektir.

## Sürümler

1. Depoda yeni bir etiket oluşturun.
2. GitHub Actions imajı inşa edecek ve tüm testleri çalıştıracaktır.
3. İnşa başarılı olursa, etiket adı etiket olarak kullanılarak imaj kayıt defterine gönderilir (örn. `v1.2.3` ve `v1.2` etiketleri oluşturulur).
4. `latest` etiketi de güncellenecektir.
