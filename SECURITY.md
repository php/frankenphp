# Security Policy

## Supported Versions

Only the latest version is supported.
Please ensure that you're always using the latest release.

Binaries and Docker images are rebuilt nightly using the latest versions of dependencies.

## Reporting a Vulnerability

If you believe you have discovered a security issue directly affecting FrankenPHP,
please do **NOT** report it publicly.

Please write a detailed vulnerability report and send it [through GitHub](https://github.com/php/frankenphp/security/advisories/new) or to [kevin+frankenphp-security@dunglas.dev](mailto:kevin+frankenphp-security@dunglas.dev?subject=Security%20issue%20affecting%20FrankenPHP).

Only vulnerabilities directly affecting FrankenPHP should be reported to this project.
Flaws affecting components used by FrankenPHP (PHP, Caddy, Go...) or using FrankenPHP (Laravel Octane, PHP Runtime...) should be reported to the relevant projects.

## Supply-chain hardening

FrankenPHP follows the open-source security practices from
[Astral's security guide](https://astral.sh/blog/open-source-security-at-astral):

- **Workflow auditing** --
  [Super Linter](https://github.com/super-linter/super-linter) runs
  [zizmor](https://docs.zizmor.sh/) on every pull request.
  The `unpinned-uses` rule in `zizmor.yaml` requires a tag pin on every action.
- **Least-privilege permissions** --
  Every workflow starts with `permissions: {}` and only grants access per job.
- **Environment-scoped secrets** --
  Secrets for publishing (Docker Hub, website deploy, translation API)
  live in dedicated GitHub Environments (`dockerhub`, `website`, `translate`).
- **Build provenance** --
  Release binaries carry
  [`attest-build-provenance`](https://github.com/actions/attest-build-provenance)
  attestations.
- **Dependency updates** --
  Dependabot tracks Go modules, GitHub Actions, and Docker base images.
- **Safe triggers** --
  Workflows never use `pull_request_target`.
- **No persisted credentials** --
  All `actions/checkout` steps set `persist-credentials: false`
  unless the job needs to push.
