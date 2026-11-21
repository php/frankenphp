---
title: FrankenPHP release and support policy
description: How FrankenPHP is versioned, which versions are supported, when new versions are tagged, and how Docker images and static binaries pick up PHP security fixes.
---

# Release and support policy

## Versioning

FrankenPHP follows [semantic versioning](https://semver.org).
Breaking changes needed to fix a security issue can still be released in a patch or minor version.

The backward compatibility promise covers:

- the Caddyfile directives and options, and the `frankenphp` commands
- the PHP functions and constants provided by FrankenPHP
- the Go library and extension APIs, except those marked as experimental

The Go APIs follow it more loosely: a release can include a small breaking change when keeping the old behavior would make the code much more complex.

The promise doesn't cover the build tooling: `build-static.sh`, the static builder images and their options, the Dockerfiles and their build arguments, and the platforms and minimum system requirements (such as the glibc version) of the prebuilt binaries.
These can change in any release.

Breaking changes are listed in the release notes.

## Supported versions

Only the latest release is supported. There are no maintenance branches and fixes aren't backported.
Please ensure that you're always using the latest release.

Maintaining several versions at the same time, as PHP or Symfony do, needs more resources than the project has today.
With more funding, an old-stable or LTS version could be considered.
If you or your company rely on FrankenPHP, please consider sponsoring its maintainers:
[@dunglas](https://github.com/sponsors/dunglas), [@henderkes](https://github.com/sponsors/henderkes),
[@AlliBalliBaba](https://github.com/AlliBalliBaba), [@alexandre-daubois](https://github.com/sponsors/alexandre-daubois)
and [@withinboredom](https://github.com/sponsors/withinboredom).

## PHP and system dependencies

The Docker images can be used in production.
The static binaries are provided for convenience and are meant for development: they bundle many PHP extensions, which increases their size and attack surface.
In production, use the Docker images, the deb, rpm or apk packages, or [build a static binary](static.md) containing only the extensions you need.

A new version of PHP or of another system dependency doesn't lead to a new FrankenPHP release.
Instead, the artifacts of the latest release are rebuilt:

- [Docker images](docker.md#frankenphp-docker-image-updates) daily, when new official PHP images are available
- [standalone binaries](production.md#standalone-binary-updates) every day, replacing the previous ones under the same name

Run `frankenphp version` to see which PHP version a binary contains.

A patch version is still tagged when an upstream change breaks the build pipeline, as the daily builds always use the latest release tag and only a new tag can ship the fix.

A patch version can also be tagged when PHP fixes an exceptionally severe vulnerability, usually rated critical (CVSS score of 9.0 or higher) in [PHP's security advisories](https://github.com/php/php-src/security/advisories).
Anyone running at least this version with the Docker images is then guaranteed to have a patched PHP.
This isn't systematic: most PHP security releases only reach users through the daily rebuilds.

To report a vulnerability, read the [security policy](https://github.com/php/frankenphp/blob/main/SECURITY.md).
