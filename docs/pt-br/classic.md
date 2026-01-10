# Usando o modo clássico

Sem nenhuma configuração adicional, o FrankenPHP opera no modo clássico.
Neste modo, o FrankenPHP funciona como um servidor PHP tradicional, servindo
diretamente arquivos PHP.
Isso o torna um substituto perfeito para PHP-FPM ou Apache com mod_php.

Semelhante ao Caddy, o FrankenPHP aceita um número ilimitado de conexões e usa
um [número fixo de threads](config.md#configuracao-do-caddyfile) para servi-las.
O número de conexões aceitas e enfileiradas é limitado apenas pelos recursos
disponíveis no sistema.
O pool de threads do PHP opera com um número fixo de threads inicializadas na
inicialização, comparável ao modo estático do PHP-FPM.
Também é possível permitir que as threads
[escalem automaticamente em tempo de execução](performance.md#max_threads),
semelhante ao modo dinâmico do PHP-FPM.

As conexões enfileiradas aguardarão indefinidamente até que uma thread PHP
esteja disponível para servi-las.
Para evitar isso, você pode usar a
[configuração](config.md#configuracao-do-caddyfile) `max_wait_time` na
configuração global do FrankenPHP para limitar o tempo que uma requisição pode
esperar por uma thread PHP livre antes de ser rejeitada.
Além disso, você pode definir um
[tempo limite de escrita razoável no Caddy](https://caddyserver.com/docs/caddyfile/options#timeouts).

Cada instância do Caddy ativará apenas um pool de threads do FrankenPHP, que
será compartilhado entre todos os blocos `php_server`.
