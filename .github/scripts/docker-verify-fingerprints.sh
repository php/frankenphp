#!/usr/bin/env bash
set -euo pipefail

PHP_VERSION="${PHP_VERSION:-}"
GO_VERSION="${GO_VERSION:-}"
USE_LATEST_PHP="${USE_LATEST_PHP:-0}"

if [[ -z "${GO_VERSION}" ]]; then
	GO_VERSION="$(awk -F'"' '/variable "GO_VERSION"/ {f=1} f && /default/ {print $2; exit}' docker-bake.hcl)"
	GO_VERSION="${GO_VERSION:-1.26}"
fi

if [[ -z "${PHP_VERSION}" ]]; then
	PHP_VERSION="$(awk -F'"' '/variable "PHP_VERSION"/ {f=1} f && /default/ {print $2; exit}' docker-bake.hcl)"
	PHP_VERSION="${PHP_VERSION:-8.2,8.3,8.4,8.5}"
fi

if [[ "${USE_LATEST_PHP}" == "1" ]]; then
	PHP_82_LATEST=$(skopeo inspect docker://docker.io/library/php:8.2 --override-os linux --override-arch amd64 | jq -r '.Env[] | select(test("^PHP_VERSION=")) | sub("^PHP_VERSION="; "")')
	PHP_83_LATEST=$(skopeo inspect docker://docker.io/library/php:8.3 --override-os linux --override-arch amd64 | jq -r '.Env[] | select(test("^PHP_VERSION=")) | sub("^PHP_VERSION="; "")')
	PHP_84_LATEST=$(skopeo inspect docker://docker.io/library/php:8.4 --override-os linux --override-arch amd64 | jq -r '.Env[] | select(test("^PHP_VERSION=")) | sub("^PHP_VERSION="; "")')
	PHP_85_LATEST=$(skopeo inspect docker://docker.io/library/php:8.5 --override-os linux --override-arch amd64 | jq -r '.Env[] | select(test("^PHP_VERSION=")) | sub("^PHP_VERSION="; "")')
	PHP_VERSION="${PHP_82_LATEST},${PHP_83_LATEST},${PHP_84_LATEST},${PHP_85_LATEST}"
fi

OS_LIST=()
while IFS= read -r os; do
	OS_LIST+=("${os}")
done < <(
	python3 - <<'PY'
import re

with open("docker-bake.hcl", "r", encoding="utf-8") as f:
	data = f.read()

# Find the first "os = [ ... ]" block and extract quoted values
m = re.search(r'os\s*=\s*\[(.*?)\]', data, re.S)
if not m:
	raise SystemExit(1)

vals = re.findall(r'"([^"]+)"', m.group(1))
for v in vals:
	print(v)
PY
)

IFS=',' read -r -a PHP_VERSIONS <<<"${PHP_VERSION}"

BASE_IMAGES=()
for os in "${OS_LIST[@]}"; do
	BASE_IMAGES+=("golang:${GO_VERSION}-${os}")
	for pv in "${PHP_VERSIONS[@]}"; do
		BASE_IMAGES+=("php:${pv}-zts-${os}")
	done
done

mapfile -t BASE_IMAGES < <(printf '%s\n' "${BASE_IMAGES[@]}" | sort -u)

BASE_IMAGE_DIGESTS=()
for image in "${BASE_IMAGES[@]}"; do
	if [[ "${image}" == */* ]]; then
		ref="docker://docker.io/${image}"
	else
		ref="docker://docker.io/library/${image}"
	fi
	digest="$(skopeo inspect "${ref}" --override-os linux --override-arch amd64 --format '{{.Digest}}')"
	BASE_IMAGE_DIGESTS+=("${image}@${digest}")
done

hash_cmd="sha256sum"
if ! command -v "${hash_cmd}" >/dev/null 2>&1; then
	hash_cmd="shasum -a 256"
fi

fingerprint="$(printf '%s\n' "${BASE_IMAGE_DIGESTS[@]}" | sort | ${hash_cmd} | awk '{print $1}')"

echo "PHP_VERSION=${PHP_VERSION}"
echo "GO_VERSION=${GO_VERSION}"
echo "OS_LIST=${OS_LIST[*]}"
echo "Base images:"
printf '  %s\n' "${BASE_IMAGES[@]}"
echo "Fingerprint: ${fingerprint}"
