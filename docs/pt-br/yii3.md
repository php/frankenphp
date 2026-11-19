---
title: Executando Yii3 com FrankenPHP (Docker, modo worker)
description: Como executar uma aplicação Yii3 com FrankenPHP usando a imagem Docker, uma instalação local ou o modo worker com o pacote yii-runner-frankenphp.
---

# Yii3

## Executando Yii3 com a imagem Docker do FrankenPHP

Servir uma aplicação web [Yii](https://www.yiiframework.com/) com FrankenPHP é tão fácil quanto montar o projeto no diretório `/app` da imagem Docker oficial.

Execute este comando a partir do diretório principal da sua aplicação Yii:

```console
docker run -p 80:80 -p 443:443 -p 443:443/udp -v $PWD:/app dunglas/frankenphp
```

E aproveite!

## Instalando Yii3 com FrankenPHP localmente

Como alternativa, você pode executar seus projetos Yii com FrankenPHP a partir da sua máquina local:

1. [Baixe o binário correspondente ao seu sistema](../#binario-autonomo)
2. Adicione a seguinte configuração a um arquivo chamado `Caddyfile` no diretório raiz do seu projeto Yii:

   ```caddyfile
   {
   	frankenphp
   }

   # The domain name of your server
   localhost {
   	# Enable compression (optional)
   	encode zstd br gzip
   	# Execute PHP files from the public/ directory and serve assets
   	php_server {
   		root public/
   		try_files {path} index.php
   	}
   }
   ```

3. Inicie o FrankenPHP a partir do diretório raiz do seu projeto Yii: `frankenphp run`

## Modo worker do Yii3

Para executar sua aplicação Yii em [modo worker](worker.md), instale o [runner Yii para FrankenPHP](https://github.com/yiisoft/yii-runner-frankenphp):

```console
composer require yiisoft/yii-runner-frankenphp
```

Em seguida, crie um arquivo chamado `worker.php` no diretório raiz da sua aplicação:

```php
<?php

declare(strict_types=1);

use Yiisoft\Yii\Runner\FrankenPHP\FrankenPHPApplicationRunner;

require_once __DIR__ . '/vendor/autoload.php';

(new FrankenPHPApplicationRunner(rootPath: __DIR__))->run();
```

Atualize seu `Caddyfile` para iniciar a aplicação em modo worker:

```caddyfile
{
	frankenphp
}

localhost {
	encode zstd br gzip
	php_server {
		root public/
		worker ./worker.php {
			# Envia todas as requisições para o worker
			match *
			# Recarrega os workers quando arquivos PHP são alterados (somente desenvolvimento)
			watch ./**/*.php
		}
	}
}
```

Se sua aplicação é baseada no [template oficial de app do Yii](https://github.com/yiisoft/app), consulte o [readme do pacote](https://github.com/yiisoft/yii-runner-frankenphp) para um exemplo completo de `worker.php` com configuração de depuração, ambiente e manipulador de erros.

Para limitar o número de requisições que um worker processa antes de ser reiniciado (útil para mitigar vazamentos de memória), defina a variável de ambiente `MAX_REQUESTS`. Por padrão, os workers processam requisições indefinidamente.

Ao usar o modo worker, certifique-se de que os serviços com estado sejam reiniciados após cada requisição. Consulte a [documentação do modo worker](worker.md) e a [documentação do StateResetter do Yii DI](https://github.com/yiisoft/di#resetting-services-state) para detalhes.
