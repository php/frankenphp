# Tempo real

O FrankenPHP vem com um hub [Mercure](https://mercure.rocks) integrado!
O Mercure permite que você envie eventos em tempo real para todos os
dispositivos conectados: eles receberão um evento JavaScript instantaneamente.

Não é necessária nenhuma biblioteca JS ou SDK!

![Mercure](mercure-hub.png)

Para habilitar o hub Mercure, atualize o `Caddyfile` conforme descrito
[no site do Mercure](https://mercure.rocks/docs/hub/config).

O caminho do hub Mercure é `/.well-known/mercure`.
Ao executar o FrankenPHP dentro do Docker, a URL de envio completa seria
`http://php/.well-known/mercure` (com `php` sendo o nome do contêiner que
executa o FrankenPHP).

Para enviar atualizações do Mercure do seu código, recomendamos o
[Componente Symfony Mercure](https://symfony.com/components/Mercure) (você não
precisa do framework full-stack do Symfony para usá-lo).
