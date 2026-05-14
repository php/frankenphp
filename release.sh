#!/usr/bin/env bash

# Dispatches the Release workflow, which does the real work
# (PGO refresh, Caddy module bump, commit, tag, push, draft release,
# Homebrew formula bump). See .github/workflows/release.yaml.

set -o nounset
set -o errexit
set -o pipefail
trap 'echo "Aborting on line $LINENO. Exit: $?" >&2' ERR

if ! command -v gh >/dev/null; then
	echo 'The "gh" command must be installed.' >&2
	exit 1
fi

if [[ $# -ne 1 ]]; then
	echo "Usage: ./release.sh version" >&2
	exit 1
fi

# Adapted from https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
if [[ ! $1 =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*))*))?(\+([0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*))?$ ]]; then
	echo "Invalid version number: $1" >&2
	exit 1
fi

# Cheap operator-side guards so the workflow dispatch matches local intent.
if [[ "$(git branch --show-current 2>/dev/null)" != "main" ]]; then
	echo "You must be on the main branch to dispatch a release." >&2
	exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
	echo "Working tree is not clean. Commit or stash your changes first." >&2
	exit 1
fi

if gh api "repos/{owner}/{repo}/git/refs/tags/v$1" --silent 2>/dev/null; then
	echo "Tag v$1 already exists on origin." >&2
	exit 1
fi

gh workflow run release.yaml --ref main -f version="$1"
echo "Release workflow dispatched for v$1."
echo "Follow progress with: gh run watch \$(gh run list --workflow=release.yaml --limit=1 --json databaseId -q '.[0].databaseId')"
