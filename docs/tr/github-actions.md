# GitHub Actions Kullanma

Bu depo Docker imajını [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) üzerinde derler ve dağıtır.
Bu durum onaylanan her çekme (pull) isteğinde veya çatallandıktan (fork) sonra gerçekleşir.

## GitHub Eylemlerini Ayarlama

Depo ayarlarında, gizli değerler altında aşağıdaki gizli değerleri ekleyin:

- `REGISTRY_LOGIN_SERVER`: Kullanılacak Docker Registry bilgisi (örneğin `docker.io`).
- `REGISTRY_USERNAME`: Giriş yapmak için kullanılacak kullanıcı adı (örn. `dunglas`).
- `REGISTRY_PASSWORD`: Oturum açmak için kullanılacak parola (örn. bir erişim anahtarı).
- `IMAGE_NAME`: İmajın adı (örn. `dunglas/frankenphp`).

## İmajı Oluşturma ve Dağıtma

1. Bir Çekme (pull) İsteği oluşturun veya çatala (forka) dağıtın.
2. GitHub Actions imajı oluşturacak ve tüm testleri çalıştıracaktır.
3. Derleme başarılı olursa, görüntü `pr-x` (burada `x` PR numarasıdır) etiketi kullanılarak ilgili saklanan yere (registry'e) gönderilir.

## İmajı Dağıtma

1. Çekme (pull) isteği birleştirildikten sonra, GitHub Actions testleri tekrar çalıştıracak ve yeni bir imaj oluşturacaktır.
2. Derleme başarılı olursa, `main` etiketi Docker Registry'de güncellenecektir.

## Bültenler

1. Depoda yeni bir etiket oluşturun.
2. GitHub Actions imajı oluşturacak ve tüm testleri çalıştıracaktır.
3. Derleme başarılı olursa, etiket adı etiket olarak kullanılarak imaj saklanan yere (registry'e) gönderilir (örneğin `v1.2.3` ve `v1.2` oluşturulur).
4. `latest` etiketi de güncellenecektir.
