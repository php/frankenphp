# Hot Reload

FrankenPHP inclui um recurso de **hot reload** integrado projetado para melhorar significativamente a experiência do desenvolvedor.

![Mercure](hot-reload.png)

Este recurso oferece um fluxo de trabalho semelhante ao **Hot Module Replacement (HMR)** encontrado em ferramentas JavaScript modernas (como Vite ou webpack).
Em vez de atualizar manualmente o navegador após cada alteração de arquivo (código PHP, templates, arquivos JavaScript e CSS...),
FrankenPHP atualiza o conteúdo em tempo real.

O Hot Reload funciona nativamente com WordPress, Laravel, Symfony e qualquer outra aplicação ou framework PHP.

Quando ativado, o FrankenPHP monitora seu diretório de trabalho atual em busca de alterações no sistema de arquivos.
Quando um arquivo é modificado, ele envia uma atualização [Mercure](mercure.md) para o navegador.

Dependendo da sua configuração, o navegador irá:

- **Transformar o DOM** (preservando a posição de rolagem e o estado dos inputs) se o [Idiomorph](https://github.com/bigskysoftware/idiomorph) for carregado.
- **Recarregar a página** (recarregamento ao vivo padrão) se o Idiomorph não estiver presente.

## Configuração

Para habilitar o hot reload, ative o Mercure e adicione a subdiretiva `hot_reload` à diretiva `php_server` no seu `Caddyfile`.

> [!AVISO]
> Este recurso é destinado **apenas a ambientes de desenvolvimento**.
> Não habilite `hot_reload` em produção, pois o monitoramento do sistema de arquivos acarreta sobrecarga de desempenho e expõe endpoints internos.

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
}
```

Por padrão, o FrankenPHP irá monitorar todos os arquivos no diretório de trabalho atual que correspondem a este padrão glob: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

É possível definir explicitamente os arquivos a serem monitorados usando a sintaxe glob:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload src/**/*{.php,.js} config/**/*.yaml
}
```

Use a forma longa para especificar o tópico Mercure a ser usado, bem como quais diretórios ou arquivos monitorar, fornecendo caminhos para a opção `hot_reload`:

```caddyfile
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload {
        topic hot-reload-topic
        watch src/**/*.php
        watch assets/**/*.{ts,json}
        watch templates/
        watch public/css/
    }
}
```

## Integração no Lado do Cliente

Enquanto o servidor detecta as alterações, o navegador precisa se inscrever nesses eventos para atualizar a página.
FrankenPHP expõe a URL do Mercure Hub a ser usada para se inscrever em alterações de arquivo através da variável de ambiente `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

Uma biblioteca JavaScript conveniente, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), também está disponível para lidar com a lógica do lado do cliente.
Para usá-la, adicione o seguinte ao seu layout principal:

```php
<!DOCTYPE html>
<title>FrankenPHP Hot Reload</title>
<?php if (isset($_SERVER['FRANKENPHP_HOT_RELOAD'])): ?>
<meta name="frankenphp-hot-reload:url" content="<?=$_SERVER['FRANKENPHP_HOT_RELOAD']?>">
<script src="https://cdn.jsdelivr.net/npm/idiomorph"></script>
<script src="https://cdn.jsdelivr.net/npm/frankenphp-hot-reload/+esm" type="module"></script>
<?php endif ?>
```

A biblioteca irá se inscrever automaticamente no hub Mercure, buscar a URL atual em segundo plano quando uma alteração de arquivo for detectada e transformar o DOM.
Ela está disponível como um pacote [npm](https://www.npmjs.com/package/frankenphp-hot-reload) e no [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

Alternativamente, você pode implementar sua própria lógica do lado do cliente inscrevendo-se diretamente no hub Mercure usando a classe nativa JavaScript `EventSource`.

### Modo Worker

Se você estiver executando sua aplicação no [Modo Worker](https://frankenphp.dev/docs/worker/), seu script de aplicação permanece na memória.
Isso significa que as alterações no seu código PHP não serão refletidas imediatamente, mesmo que o navegador recarregue.

Para a melhor experiência do desenvolvedor, você deve combinar `hot_reload` com [a subdiretiva `watch` na diretiva `worker`](config.md#watching-for-file-changes).

- `hot_reload`: atualiza o **navegador** quando arquivos mudam
- `worker.watch`: reinicia o worker quando arquivos mudam

```caddy
localhost

mercure {
    anonymous
}

root public/
php_server {
    hot_reload
    worker {
        file /path/to/my_worker.php
        watch
    }
}
```

### Como funciona

1. **Monitoramento**: FrankenPHP monitora o sistema de arquivos em busca de modificações usando a [biblioteca `e-dant/watcher`](https://github.com/e-dant/watcher) por baixo dos panos (contribuímos com o binding Go).
2. **Reinício (Modo Worker)**: se `watch` estiver ativado na configuração do worker, o worker PHP é reiniciado para carregar o novo código.
3. **Envio**: um payload JSON contendo a lista de arquivos alterados é enviado para o [hub Mercure](https://mercure.rocks) integrado.
4. **Recebimento**: O navegador, escutando via a biblioteca JavaScript, recebe o evento Mercure.
5. **Atualização**:

- Se **Idiomorph** for detectado, ele busca o conteúdo atualizado e transforma o HTML atual para corresponder ao novo estado, aplicando as alterações instantaneamente sem perder o estado.
- Caso contrário, `window.location.reload()` é chamado para recarregar a página.
