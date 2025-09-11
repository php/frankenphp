# リアルタイム

FrankenPHPには組み込みの[Mercure](https://mercure.rocks)ハブが付属しています！
Mercureを使用すると、接続されているすべてのデバイスにリアルタイムイベントをプッシュでき、各デバイスは即座にJavaScriptイベントを受信します。

JSライブラリやSDKは必要ありません！

![Mercure](mercure-hub.png)

Mercureハブを有効にするには、[Mercureのサイト](https://mercure.rocks/docs/hub/config)で説明されているように`Caddyfile`を更新してください。

Mercureハブのパスは`/.well-known/mercure`です。
FrankenPHPをDocker内で実行している場合、完全な送信URLは`http://php/.well-known/mercure`のようになります。ここでの`php`はFrankenPHPを実行するコンテナの名前です。

コードからMercureの更新をプッシュするには、[Symfony Mercure Component](https://symfony.com/components/Mercure)をお勧めします。なお、Symfonyのフルスタックフレームワークは必要ありません。
