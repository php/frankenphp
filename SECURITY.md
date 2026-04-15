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

FrankenPHP follows the open-source security practices documented in
[Astral's "Open source security at Astral" post](https://astral.sh/blog/open-source-security-at-astral):

- **Workflow auditing.** Every push and pull request that touches CI
  is audited by [zizmor](https://docs.zizmor.sh/) as a hard gate. The
  `unpinned-uses` rule in `zizmor.yaml` requires, at a minimum, a tag
  pin on every action.
- **Least-privilege permissions.** Every workflow starts with
  `permissions: {}` and only broadens access on a per-job basis, so a
  newly added job inherits no permissions by default.
- **Environment-scoped secrets.** Secrets that publish artifacts
  (Docker Hub credentials, the website deploy token, the translation
  API key) live in dedicated GitHub Environments (`dockerhub`,
  `website`, `translate`) instead of repository-wide secrets,
  limiting the blast radius of a compromised job.
- **Build provenance.** Release binaries are attested with
  [`actions/attest-build-provenance`](https://github.com/actions/attest-build-provenance)
  so downstream consumers can verify they were produced by this
  repository's CI.
- **Continuous dependency updates.** Dependabot tracks Go modules,
  GitHub Actions and Docker base images; new versions land through
  reviewable pull requests rather than implicit `latest` upgrades.
- **No `pull_request_target`.** Workflows never use the
  `pull_request_target` trigger, which would expose write tokens to
  fork pull requests.
- **Checkout without persisted credentials.** All `actions/checkout`
  steps set `persist-credentials: false` unless they specifically
  need to push back to the repository.
