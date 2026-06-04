# Security Policy

## Supported Versions

Only the latest version is supported.
Please ensure that you're always using the latest release.

Binaries and Docker images are rebuilt nightly using the latest versions of dependencies.

## Security Model

FrankenPHP embeds the PHP interpreter into a Go and Caddy server, so its trust boundaries span Go, C, and PHP.
Before auditing the project or reporting an issue, read the [security model documentation](docs/security.md),
which describes what is trusted, what is not, and which attack surfaces belong to FrankenPHP itself.

## Reporting a Vulnerability

If you believe you have discovered a security issue directly affecting FrankenPHP,
please do **NOT** report it publicly.

Please write a detailed vulnerability report and send it [through GitHub](https://github.com/php/frankenphp/security/advisories/new) or to [kevin+frankenphp-security@dunglas.dev](mailto:kevin+frankenphp-security@dunglas.dev?subject=Security%20issue%20affecting%20FrankenPHP).

Only vulnerabilities directly affecting FrankenPHP should be reported to this project.
Flaws affecting components used by FrankenPHP (PHP, Caddy, Go...) or using FrankenPHP (Laravel Octane, PHP Runtime...) should be reported to the relevant projects.
