#!/usr/bin/env bash

# Creates the tags for the library and the Caddy module.

set -o nounset
set -o errexit
trap 'echo "Aborting due to errexit on line $LINENO. Exit code: $?" >&2' ERR
set -o errtrace
set -o pipefail
set -o xtrace

if ! type "git" >/dev/null; then
	echo "The \"git\" command must be installed."
	exit 1
fi

if ! type "gh" >/dev/null; then
	echo "The \"gh\" command must be installed."
	exit 1
fi

if ! type "brew" >/dev/null; then
	echo "The \"brew\" command must be installed."
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

# Pre-flight checks
if [[ "$(git branch --show-current)" != "main" ]]; then
	echo "You must be on the main branch to release." >&2
	exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
	echo "Working tree is not clean. Commit or stash your changes first." >&2
	exit 1
fi

git fetch origin
local_head="$(git rev-parse HEAD)"
remote_head="$(git rev-parse origin/main)"
if [[ "$local_head" != "$remote_head" ]]; then
	if git merge-base --is-ancestor HEAD origin/main; then
		echo "Local main is behind origin/main. Pull first." >&2
	elif git merge-base --is-ancestor origin/main HEAD; then
		echo "Local main is ahead of origin/main. Push your commits or reset to origin/main before releasing." >&2
	else
		echo "Local main has diverged from origin/main. Reconcile with pull/rebase/reset before releasing." >&2
	fi
	exit 1
fi

cd caddy/
go get "github.com/dunglas/frankenphp@v$1"
cd -

git commit -S -a -m "chore: prepare release $1" || echo "skip"

git tag -s -m "Version $1" "v$1"
git tag -s -m "Version $1" "caddy/v$1"
git push --follow-tags

tags=$(git tag --list --sort=-version:refname 'v*')
previous_tag=$(awk 'NR==2 {print;exit}' <<<"${tags}")

gh release create --draft --generate-notes --latest --notes-start-tag "${previous_tag}" --verify-tag "v$1"
brew bump-formula-pr dunglas/frankenphp/frankenphp --version "$1"
