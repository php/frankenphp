---
title: "Modelo de segurança do FrankenPHP: limites de confiança entre Go e PHP"
description: "Onde reside o limite de confiança do FrankenPHP: quais entradas não são confiáveis, pelo que o runtime Go/C é responsável e o que cabe à aplicação PHP ou a projetos upstream."
---

# Modelo de segurança

Este documento descreve o modelo de confiança do FrankenPHP: quais entradas são confiáveis, quais não são e onde reside o limite entre elas. Ele tem como objetivo ajudar auditorias de segurança e scanners automatizados a raciocinar sobre o **próprio FrankenPHP**, e não sobre as aplicações PHP que ele serve.

Para os mecanismos internos aqui referenciados (threads, o limite CGO, sandboxing de ambiente), consulte [Internals](internals.md). Para persistência de estado em processos de longa duração, consulte [Modo Worker](worker.md).

## Limites de confiança

O FrankenPHP é executado em uma stack com quatro atores distintos:

| Ator                   | Confiança                  | Notas                                                                                                                                                |
| ----------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Cliente remoto**       | Não confiável              | A requisição HTTP (método, URI, cabeçalhos, cookies, corpo, uploads) é a principal fonte de entrada adulterada.                                             |
| **Operador**            | Confiável                | Fornece a configuração de implantação: o `Caddyfile`, variáveis de ambiente, `php.ini`, extensões PHP e módulos Caddy instalados, e o próprio código da aplicação. |
| **Código da aplicação PHP** | Confiável *por proveniência* | Implantado pelo operador, então o FrankenPHP nunca executa código fornecido por atacantes, mas este código *consome* dados de requisição não confiáveis.                    |
| **FrankenPHP (Go + C)** | Base de computação confiável | Incorpora PHP, transporta dados para dentro e para fora, e isola requisições e threads. Seus próprios defeitos são o que este documento aborda.                            |

## Proveniência do código vs. contaminação de dados

Esta é a distinção mais importante, e aquela que resolve a maior parte da confusão "confiamos no PHP ou não?":

- **A proveniência do código é confiável.** O FrankenPHP só executa arquivos PHP implantados pelo operador: o script resolvido sob o document root, ou o script worker configurado. Ele nunca avalia código carregado na própria requisição (corpo, query string, cabeçalhos): o limite SAPI carrega *dados*, nunca *código* não confiável.
- **Os dados da requisição estão contaminados.** Tudo o que o código PHP lê da requisição (`$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER`, `php://input`) não é confiável, exatamente como em qualquer SAPI PHP. Sanitizá-los é trabalho da aplicação.

Então, "confiamos no que vem do PHP" é verdadeiro para o *código*, enquanto "o SAPI carrega entrada não confiável" é verdadeiro para os *dados*: os dois não estão em conflito. O trabalho do FrankenPHP é transportar esses dados contaminados fielmente e evitar que os dados de uma requisição vazem para outra.

## Pelo que o FrankenPHP é responsável

A base de computação confiável tem três trabalhos. Defeitos de segurança no FrankenPHP residem em um deles:

1. **Transporte fiel**: mapear a requisição para superglobais PHP e `php://input`, e transportar a saída e os cabeçalhos do PHP de volta para o cliente, sem introduzir injeção (injeção de cabeçalho/CRLF, contrabando de requisições, execução do arquivo errado).
2. **Isolamento**: evitar que o estado com escopo de requisição cruze entre requisições, entre threads PHP e entre iterações do worker.
3. **Segurança de memória**: gerenciar o limite CGO (Go ↔ C/PHP) sem corromper a memória.

## Em escopo: superfície de ataque do FrankenPHP

Estas são as superfícies que o FrankenPHP possui. Uma vulnerabilidade aqui é uma vulnerabilidade do FrankenPHP:

- **Mapeamento de requisição para superglobal** (`cgi.go`, `frankenphp_register_server_vars`): construção de `$_SERVER`, `REMOTE_ADDR`, `SCRIPT_NAME`, `PATH_INFO` e as outras variáveis CGI a partir da requisição.
- **Resolução do caminho do script PHP**: o caminho da requisição é dividido em `split_path` (`.php` por padrão) em `SCRIPT_NAME` / `PATH_INFO`, então unido ao document root com `sanitizedPathJoin` (`filepath.Join(root, filepath.Clean("/"+reqPath))`), o que impede que `SCRIPT_FILENAME` escape do document root (path traversal). A diretiva `php_server` adicionalmente define um `try_files` padrão que roteia requisições para arquivos existentes ou para o controlador frontal, mitigando a armadilha clássica do PHP-FPM de executar o arquivo errado.
- **Isolamento de estado no modo worker**: o FrankenPHP redefine `$_GET`, `$_POST`, `$_COOKIE`, `$_FILES`, `$_SERVER` e `$_REQUEST` entre as requisições, limpa explicitamente `$_SESSION` (que de outra forma vazaria entre as requisições) e os erros retornados por `error_get_last()` e `libxml_get_errors()`, e os cabeçalhos retornados por `http_get_last_response_headers()`, o estado de `strtok()` e as variáveis adicionadas por `output_add_rewrite_var()`, mas **`$_ENV` não é redefinido**, e as escritas de `putenv()`, variáveis `static`, propriedades estáticas de classe, globais e configurações do motor de tempo de execução (`ini_set()`, o contexto de stream padrão, o fuso horário padrão, o diretório de trabalho, ...) persistem entre requisições na mesma thread. Dados específicos da requisição ou do usuário deixados nesse estado podem vazar para uma requisição posterior (consulte [Modo Worker](worker.md#state-persistence)).
- **Sandboxing de ambiente por thread**: `frankenphp_putenv()` / `frankenphp_getenv()` operam em um `sandboxed_env` local à thread. As escritas de `putenv()` permanecem dentro desse sandbox e nunca alcançam o ambiente do sistema operacional global ao processo, então um segredo definido por uma requisição não pode vazar para processos filhos (`proc_open`, `exec`, ...), para o lado Go/Caddy, ou para outras requisições. Um processo filho que precise ver tal variável deve recebê-la explicitamente, através do argumento de ambiente de `proc_open()` ou `exec()` (consulte [Internals](internals.md#ambiente-sandbox-por-thread)).
- **Limite de memória CGO**: fixação de strings Go e ciclos de vida de `C.CString()` / `free()` através do limite Go ↔ C.
- **API de administração do Caddy**: os endpoints `/frankenphp/workers/restart` e `/frankenphp/threads`, expostos através da API de administração do Caddy (que escuta em `localhost:2019` por padrão). Expor esse endpoint além do localhost é uma decisão do operador.
- **Tratamento de proxy confiável**: os cabeçalhos `X-Forwarded-*` de entrada sempre chegam ao PHP como valores `$_SERVER['HTTP_X_FORWARDED_*']` contaminados; eles são confiáveis para derivar o IP e o esquema real do cliente apenas quando [`trusted_proxies`](production.md#executando-atrás-de-um-proxy-reverso) é configurado.
- **Corpos de requisição lentos**: um cliente que anuncia um corpo e depois o envia lentamente ou o interrompe, mantém a thread de manipulação pela duração. Com um pool de threads limitado, um número suficiente de tais conexões o esgota (slow-POST DoS). O FrankenPHP aplica um tempo limite de inatividade de 60s na leitura do corpo por padrão ([`request_body_timeout`](config.md#configuração-do-caddyfile)), redefinindo o prazo antes de cada leitura para que um upload constante de qualquer tamanho seja bem-sucedido enquanto um travado é interrompido e a thread liberada.

## Fora do escopo

- **Vulnerabilidades no código PHP da aplicação** (SQL injection, XSS, desserialização insegura, etc.). O FrankenPHP entrega dados de requisição não confiáveis à aplicação sem alterações; a defesa contra eles é responsabilidade da aplicação, assim como com qualquer SAPI.
- **Falhas em componentes upstream** usados pelo FrankenPHP (PHP, Caddy, Go) ou em projetos construídos sobre ele (Laravel Octane, Symfony Runtime). Reporte-os ao projeto relevante.

## Reportando uma vulnerabilidade

Consulte [`SECURITY.md`](../SECURITY.md) para saber como relatar um problema de segurança que afete o FrankenPHP.
