# Real-time режим

FrankenPHP поставляется с встроенным хабом [Mercure](https://mercure.rocks)!  
Mercure позволяет отправлять события в режиме реального времени на все подключённые устройства: они мгновенно получат JavaScript-событие.

Не требуются JS-библиотеки или SDK!

![Mercure](../mercure-hub.png)

Чтобы включить хаб Mercure, обновите `Caddyfile` в соответствии с инструкциями [на сайте Mercure](https://mercure.rocks/docs/hub/config).

Для отправки обновлений Mercure из вашего кода мы рекомендуем использовать [Symfony Mercure Component](https://symfony.com/components/Mercure) (для его использования не требуется полный стек Symfony).
