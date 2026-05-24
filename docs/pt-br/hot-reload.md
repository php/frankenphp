# Recarregamento Instantâneo

FrankenPHP inclui um recurso de **recarregamento instantâneo** embutido, projetado para melhorar drasticamente a experiência do desenvolvedor.

![Hot Reload](hot-reload.png)

Este recurso oferece um fluxo de trabalho semelhante ao **Hot Module Replacement (HMR)** em ferramentas modernas de JavaScript, como Vite ou webpack.
Em vez de atualizar o navegador manualmente após cada alteração de arquivo (código PHP, templates, arquivos JavaScript e CSS...), o FrankenPHP atualiza o conteúdo da página em tempo real.

O Recarregamento Instantâneo funciona nativamente com WordPress, Laravel, Symfony e qualquer outra aplicação ou framework PHP.

Quando ativado, o FrankenPHP monitora seu diretório de trabalho atual em busca de alterações no sistema de arquivos. Quando um arquivo é modificado, ele envia uma atualização [Mercure](mercure.md) para o navegador.

Dependendo da sua configuração, o navegador irá:

- **Transformar o DOM** (preservando a posição de rolagem e o estado dos inputs) se o [Idiomorph](https://github.com/bigskysoftware/idiomorph) estiver carregado.
- **Recarregar a página** (recarregamento ao vivo padrão) se o Idiomorph não estiver presente.

## Configuração

Para ativar o recarregamento instantâneo, habilite o Mercure e, em seguida, adicione a subdiretiva `hot_reload` à diretiva `php_server` no seu `Caddyfile`.

> [!WARNING]
>
> Este recurso é destinado **apenas para ambientes de desenvolvimento**.
> Não ative `hot_reload` em produção, pois este recurso não é seguro (expõe detalhes internos sensíveis) e desacelera a aplicação.

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

Por padrão, o FrankenPHP monitorará todos os arquivos no diretório de trabalho atual que correspondem a este padrão glob: `./**/*.{css,env,gif,htm,html,jpg,jpeg,js,mjs,php,png,svg,twig,webp,xml,yaml,yml}`

É possível definir os arquivos a serem monitorados usando a sintaxe glob explicitamente:

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

Use a forma longa de `hot_reload` para especificar o tópico Mercure a ser usado, bem como quais diretórios ou arquivos monitorar:

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
O FrankenPHP expõe a URL do Hub Mercure a ser usada para se inscrever em alterações de arquivo através da variável de ambiente `$_SERVER['FRANKENPHP_HOT_RELOAD']`.

Uma biblioteca JavaScript de conveniência, [frankenphp-hot-reload](https://www.npmjs.com/package/frankenphp-hot-reload), também está disponível para lidar com a lógica do lado do cliente.
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

A biblioteca se inscreverá automaticamente no hub Mercure, buscará a URL atual em segundo plano quando uma alteração de arquivo for detectada e transformará o DOM.
Está disponível como um pacote [npm](https://www.npmjs.com/package/frankenphp-hot-reload) e no [GitHub](https://github.com/dunglas/frankenphp-hot-reload).

Alternativamente, você pode implementar sua própria lógica do lado do cliente inscrevendo-se diretamente no hub Mercure usando a classe nativa `EventSource` do JavaScript.

### Preservando Nós DOM Existentes

Em casos raros, como ao usar ferramentas de desenvolvimento [como a barra de depuração da web do Symfony](https://github.com/symfony/symfony/pull/62970), você pode querer preservar nós DOM específicos.
Para fazer isso, adicione o atributo `data-frankenphp-hot-reload-preserve` ao elemento HTML relevante:

```html
<div data-frankenphp-hot-reload-preserve><!-- My debug bar --></div>
```

## Modo Worker

Se você estiver executando sua aplicação em [Modo Worker](https://frankenphp.dev/docs/worker/), seu script de aplicação permanece na memória.
Isso significa que as alterações no seu código PHP não serão refletidas imediatamente, mesmo que o navegador recarregue.

Para a melhor experiência do desenvolvedor, você deve combinar `hot_reload` com [a subdiretiva `watch` na diretiva `worker`](config.md#observando-alteracoes-de-arquivos).

- `hot_reload`: atualiza o **navegador** quando os arquivos são alterados
- `worker.watch`: reinicia o worker quando os arquivos são alterados

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

## Como Funciona

1. **Monitoramento**: O FrankenPHP monitora o sistema de arquivos em busca de modificações usando a biblioteca [`e-dant/watcher`](https://github.com/e-dant/watcher) por baixo dos panos (contribuímos com o binding Go).
2. **Reiniciar (Modo Worker)**: se `watch` estiver habilitado na configuração do worker, o worker PHP é reiniciado para carregar o novo código.
3. **Envio**: Um payload JSON contendo a lista de arquivos alterados é enviado para o [hub Mercure](https://mercure.rocks) embutido.
4. **Recebimento**: O navegador, ouvindo através da biblioteca JavaScript, recebe o evento Mercure.
5. **Atualização**:

- Se o **Idiomorph** for detectado, ele busca o conteúdo atualizado e transforma o HTML atual para corresponder ao novo estado, aplicando as alterações instantaneamente sem perder o estado.
- Caso contrário, `window.location.reload()` é chamado para recarregar a página.
