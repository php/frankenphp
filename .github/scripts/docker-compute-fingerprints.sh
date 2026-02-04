#!/usr/bin/env bash
set -euo pipefail

write_output() {
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "$1" >> "${GITHUB_OUTPUT}"
  else
    echo "$1"
  fi
}

get_php_version() {
  local version="$1"
  skopeo inspect "docker://docker.io/library/php:${version}" \
    --override-os linux \
    --override-arch amd64 |
    jq -r '.Env[] | select(test("^PHP_VERSION=")) | sub("^PHP_VERSION="; "")'
}

PHP_82_LATEST="$(get_php_version 8.2)"
PHP_83_LATEST="$(get_php_version 8.3)"
PHP_84_LATEST="$(get_php_version 8.4)"
PHP_85_LATEST="$(get_php_version 8.5)"

PHP_VERSION="${PHP_82_LATEST},${PHP_83_LATEST},${PHP_84_LATEST},${PHP_85_LATEST}"
write_output "php_version=${PHP_VERSION}"
write_output "php82_version=${PHP_82_LATEST//./-}"
write_output "php83_version=${PHP_83_LATEST//./-}"
write_output "php84_version=${PHP_84_LATEST//./-}"
write_output "php85_version=${PHP_85_LATEST//./-}"

if [[ "${GITHUB_EVENT_NAME:-}" == "schedule" ]]; then
  FRANKENPHP_LATEST_TAG="$(gh release view --repo php/frankenphp --json tagName --jq '.tagName')"
  git checkout "${FRANKENPHP_LATEST_TAG}"
fi

METADATA="$(PHP_VERSION="${PHP_VERSION}" docker buildx bake --print | jq -c)"

BASE_IMAGES=()
while IFS= read -r image; do
  BASE_IMAGES+=("${image}")
done < <(jq -r '
  .target[]?.contexts? | to_entries[]?
  | select(.value | startswith("docker-image://"))
  | .value
  | sub("^docker-image://"; "")
' <<< "${METADATA}" | sort -u)

BASE_IMAGE_DIGESTS=()
for image in "${BASE_IMAGES[@]}"; do
  if [[ "${image}" == */* ]]; then
    ref="docker://docker.io/${image}"
  else
    ref="docker://docker.io/library/${image}"
  fi
  digest="$(skopeo inspect "${ref}" \
    --override-os linux \
    --override-arch amd64 \
    --format '{{.Digest}}')"
  BASE_IMAGE_DIGESTS+=("${image}@${digest}")
done

BASE_FINGERPRINT="$(printf '%s\n' "${BASE_IMAGE_DIGESTS[@]}" | sort | sha256sum | awk '{print $1}')"
write_output "base_fingerprint=${BASE_FINGERPRINT}"

if [[ "${GITHUB_EVENT_NAME:-}" != "schedule" ]]; then
  write_output "skip=false"
  exit 0
fi

FRANKENPHP_LATEST_TAG_NO_PREFIX="${FRANKENPHP_LATEST_TAG#v}"
EXISTING_FINGERPRINT=$(
  skopeo inspect "docker://docker.io/dunglas/frankenphp:${FRANKENPHP_LATEST_TAG_NO_PREFIX}" \
    --override-os linux \
    --override-arch amd64 |
    jq -r '.Labels["dev.frankenphp.base.fingerprint"] // empty'
)

if [[ -n "${EXISTING_FINGERPRINT}" ]] && [[ "${EXISTING_FINGERPRINT}" == "${BASE_FINGERPRINT}" ]]; then
  write_output "skip=true"
  exit 0
fi

write_output "ref=${FRANKENPHP_LATEST_TAG}"
write_output "skip=false"
