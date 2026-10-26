---
title: "FrankenPHP 安全模型：Go 和 PHP 之间的信任边界"
description: "FrankenPHP 的信任边界所在：哪些输入是不可信的，Go/C 运行时负责什么，以及什么属于 PHP 应用程序或上游项目。"
---

# 安全模型

本文档描述了 FrankenPHP 的信任模型：哪些输入是受信任的，哪些不是，以及它们之间的边界在哪里。它旨在帮助安全审计和自动化扫描器理解 **FrankenPHP 本身**，而不是它所服务的 PHP 应用程序。

有关此处引用的内部机制（线程、CGO 边界、环境沙盒），请参阅 [内部原理](internals.md)。有关长时间运行进程中的状态持久性，请参阅 [工作模式](worker.md)。

## 信任边界

FrankenPHP 在一个包含四个不同参与者的栈中运行：

| 参与者                   | 信任           | 备注                                                                                                                                                |
| ----------------------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **远程客户端**          | 不可信         | HTTP 请求（方法、URI、请求头、Cookie、请求体、文件上传）是主要的不受信任输入来源。                                                                    |
| **操作员**              | 受信任         | 提供部署配置：Caddyfile、环境变量、php.ini、已安装的 PHP 扩展和 Caddy 模块，以及应用程序代码本身。                                                            |
| **PHP 应用程序代码**    | *源头受信任*   | 由操作员部署，因此 FrankenPHP 从不执行攻击者提供的代码，但此代码会*消费*不受信任的请求数据。                                                              |
| **FrankenPHP (Go + C)** | 可信计算基     | 嵌入 PHP，进行数据传输，并隔离请求和线程。本文档的范围是其自身的缺陷。                                                                                      |

## 代码来源 vs. 数据污染

这是最重要的区别，它解决了大多数关于“我们是否信任 PHP？”的困惑：

- **代码来源受信任。** FrankenPHP 只执行操作员部署的 PHP 文件：在文档根目录下解析的脚本，或配置的工作进程脚本。它从不评估请求本身（请求体、查询字符串、请求头）中携带的代码：SAPI 边界传输的是*数据*，从不是不可信的*代码*。
- **请求数据是不可信的。** PHP 代码从请求中读取的所有内容（`$_GET`、`$_POST`、`$_COOKIE`、`$_FILES`、`$_SERVER`、`php://input`）都是不可信的，就像任何 PHP SAPI 一样。净化这些数据是应用程序的职责。

因此，“我们信任来自 PHP 的东西”对于*代码*来说是真实的，而“SAPI 携带不可信的输入”对于*数据*来说是真实的：两者并不冲突。FrankenPHP 的职责是忠实地传输这些不可信的数据，并防止一个请求的数据泄露到另一个请求中。

## FrankenPHP 的职责

可信计算基有三项职责。FrankenPHP 中的安全缺陷存在于其中之一：

1.  **忠实传输**：将请求映射到 PHP 超全局变量和 php://input 中，并将 PHP 的输出和请求头传回客户端，而不引入注入（请求头/CRLF 注入、请求走私、执行错误文件）。
2.  **隔离**：防止请求范围的状态在不同请求之间、PHP 线程之间以及工作进程迭代之间交叉。
3.  **内存安全**：管理 CGO 边界（Go ↔ C/PHP），不损坏内存。

## 范围：FrankenPHP 自身的攻击面

以下是 FrankenPHP 拥有的攻击面。这里的漏洞是 FrankenPHP 的漏洞：

-   **请求到超全局变量映射**（`cgi.go`、`frankenphp_register_server_vars`）：从请求构建 `$_SERVER`、`REMOTE_ADDR`、`SCRIPT_NAME`、`PATH_INFO` 和其他 CGI 变量。
-   **PHP 脚本路径解析**：请求路径在 `split_path`（默认为 `.php`）处被分割为 `SCRIPT_NAME` / `PATH_INFO`，然后通过 `sanitizedPathJoin`（`filepath.Join(root, filepath.Clean("/"+reqPath))`）与文档根目录连接，这可以防止 `SCRIPT_FILENAME` 逃逸文档根目录（路径遍历）。`php_server` 指令还会额外设置一个默认的 `try_files` 重写规则，将请求路由到现有文件或前端控制器，从而缓解了 PHP-FPM 经典陷阱——执行错误文件的问题。
-   **工作模式状态隔离**：FrankenPHP 在请求之间重置 `$_GET`、`$_POST`、`$_COOKIE`、`$_FILES`、`$_SERVER` 和 `$_REQUEST`，显式清除 `$_SESSION`（否则会在请求之间泄露），以及 `error_get_last()` 和 `libxml_get_errors()` 返回的错误，`http_get_last_response_headers()` 返回的请求头，`strtok()` 状态，以及 `output_add_rewrite_var()` 添加的变量，但 **`$_ENV` 不会被重置**，并且 `putenv()` 写入、`static` 变量、类静态属性、全局变量和运行时引擎设置（`ini_set()`、默认流上下文、默认时区、工作目录等）会在同一线程的请求之间持久存在。留在该状态中的请求或用户特定数据可能会泄露到后续请求中（参见 [工作模式](worker.md#state-persistence)）。
-   **每线程环境沙盒**：`frankenphp_putenv()` / `frankenphp_getenv()` 操作的是线程本地的 `sandboxed_env`。`putenv()` 写入保留在该沙盒内，绝不会到达进程全局的操作系统环境，因此一个请求设置的秘密不会泄露给子进程（`proc_open`、`exec` 等）、Go/Caddy 端或其他请求。必须看到此类变量的子进程必须通过 `proc_open()` 或 `exec()` 的环境参数明确地提供给它（参见 [内部原理](internals.md#per-thread-environment-sandboxing)）。
-   **CGO 内存边界**：Go 字符串固定以及 `C.CString()` / `free()` 在 Go ↔ C 边界上的生命周期。
-   **Caddy 管理 API**：`/frankenphp/workers/restart` 和 `/frankenphp/threads` 端点，通过 Caddy 的管理 API（默认监听 `localhost:2019`）暴露。将该端点暴露到 localhost 之外是操作员的决定。
-   **受信任代理处理**：传入的 `X-Forwarded-*` 请求头总是作为不可信的 `$_SERVER['HTTP_X_FORWARDED_*']` 值到达 PHP；只有当配置了 [`trusted_proxies`](production.md#running-behind-a-reverse-proxy) 时，它们才被信任用于推导真实的客户端 IP 和协议。
-   **慢速请求体**：客户端宣布请求体后，如果缓慢发送或停滞，会长时间占用处理线程。在一个有界线程池中，足够多的此类连接会耗尽线程池（慢速 POST DoS）。FrankenPHP 默认对请求体读取应用 60 秒的空闲超时（[`request_body_timeout`](config.md#caddyfile-config)），并在每次读取前重置截止时间，这样任何大小的稳定上传都能成功，而停滞的上传会被切断并释放线程。

## 范围之外

-   **应用程序 PHP 代码中的漏洞**（SQL 注入、XSS、不安全的反序列化等）。FrankenPHP 将不可信的请求数据原封不动地传递给应用程序；防御这些漏洞是应用程序的职责，就像任何 SAPI 一样。
-   **FrankenPHP 使用的上游组件（PHP、Caddy、Go）或在其之上构建的项目（Laravel Octane、Symfony Runtime）中的缺陷。** 请将这些报告给相关项目。

## 报告漏洞

有关如何报告影响 FrankenPHP 的安全问题，请参阅 [`SECURITY.md`](../SECURITY.md)。
