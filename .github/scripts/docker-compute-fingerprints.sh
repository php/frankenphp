#!/usr/bin/env bash
set -euo pipefail

write_output() {
	if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
		echo "$1" >>"${GITHUB_OUTPUT}"
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

image_ref() {
	if [[ "$1" == */* ]]; then
		echo "docker://docker.io/$1"
	else
		echo "docker://docker.io/library/$1"
	fi
}

get_image_digest() {
	skopeo inspect "$(image_ref "$1")" \
		--override-os linux \
		--override-arch amd64 \
		--format '{{.Digest}}'
}

get_image_platforms() {
	local platforms
	platforms="$(skopeo inspect --raw "$(image_ref "$1")" | jq -c '
		[.manifests // [] | .[] | .platform | select(.os == "linux")
		| "linux/\(.architecture)" + (if .architecture == "arm" and (.variant // "") != "" then "/\(.variant)" else "" end)]
		| unique
	')"

	if [[ "${platforms}" == "[]" ]]; then
		echo "No linux platform in the manifest of $1" >&2
		return 1
	fi

	echo "${platforms}"
}

get_existing_fingerprint() {
	local tag="$1"
	skopeo inspect "docker://docker.io/dunglas/frankenphp:${tag}" \
		--override-os linux \
		--override-arch amd64 2>/dev/null |
		jq -r '.Labels["dev.frankenphp.base.fingerprint"] // empty' || true
}

# Everything runs from main(), parsed before execution starts: this script checks out
# another Git reference (and thus replaces itself on disk) while running, and bash
# reads script files lazily.
main() {
	local PHP_82_LATEST PHP_83_LATEST PHP_84_LATEST PHP_85_LATEST PHP_VERSION
	PHP_82_LATEST="$(get_php_version 8.2)"
	PHP_83_LATEST="$(get_php_version 8.3)"
	PHP_84_LATEST="$(get_php_version 8.4)"
	PHP_85_LATEST="$(get_php_version 8.5)"

	PHP_VERSION="${PHP_82_LATEST},${PHP_83_LATEST},${PHP_84_LATEST},${PHP_85_LATEST}"
	write_output "php_version=${PHP_VERSION}"

	local FRANKENPHP_LATEST_TAG=""
	if [[ "${GITHUB_EVENT_NAME:-}" == "schedule" ]]; then
		FRANKENPHP_LATEST_TAG="$(gh release view --repo php/frankenphp --json tagName --jq '.tagName')"
		git checkout "${FRANKENPHP_LATEST_TAG}"
		write_output "ref=${FRANKENPHP_LATEST_TAG}"
	fi

	# Pin the revision and timestamps to the commit being built so that rebuilding
	# unchanged sources produces reproducible images
	write_output "sha=$(git rev-parse HEAD)"
	write_output "source_date_epoch=$(git log -1 --format=%ct)"
	write_output "created=$(TZ=UTC git log -1 --date=format-local:%Y-%m-%dT%H:%M:%SZ --format=%cd)"

	local METADATA
	METADATA="$(PHP_VERSION="${PHP_VERSION}" docker buildx bake --print | jq -c)"

	# Collect the base images (docker-image:// contexts) of each variant. The variant key
	# is derived from the php-base ref (e.g. "php:8.4.23-zts-trixie" -> "8.4.23-trixie") and
	# matches the "${php-version}-${os}" keys expected by docker-bake.hcl for BASE_FINGERPRINTS.
	declare -A VARIANT_IMAGES=() VARIANT_NAMES=() VARIANT_PLATFORMS=() DIGEST_CACHE=() VARIANT_FINGERPRINTS=()
	local target php_base images platforms image variant_key variant_name
	while IFS=$'\t' read -r target php_base images platforms; do
		[[ -z "${php_base}" ]] && continue
		variant_key="${php_base#php:}"
		variant_key="${variant_key/-zts/}"
		variant_name="${target#builder-}"
		variant_name="${variant_name#runner-}"
		VARIANT_IMAGES["${variant_key}"]="${images}"
		VARIANT_NAMES["${variant_key}"]="${variant_name}"
		VARIANT_PLATFORMS["${variant_key}"]="${platforms}"
	done < <(jq -r '
		.target | to_entries[]
		| (.value.contexts // {} | [to_entries[].value | select(startswith("docker-image://")) | sub("^docker-image://"; "")]) as $images
		| select(($images | length) > 0)
		| [.key, ($images | map(select(startswith("php:"))) | first // ""), ($images | sort | join(" ")), (.value.platforms // [] | tojson)]
		| @tsv
	' <<<"${METADATA}")

	# Fingerprint each variant from the digests of its own base images, and keep the
	# legacy fingerprint of all base images for images built before the per-variant split
	local fingerprints_json="{}" all_digests=()
	for variant_key in "${!VARIANT_IMAGES[@]}"; do
		local variant_digests=()
		for image in ${VARIANT_IMAGES["${variant_key}"]}; do
			if [[ -z "${DIGEST_CACHE["${image}"]:-}" ]]; then
				DIGEST_CACHE["${image}"]="$(get_image_digest "${image}")"
			fi
			variant_digests+=("${image}@${DIGEST_CACHE["${image}"]}")
		done
		VARIANT_FINGERPRINTS["${variant_key}"]="$(printf '%s\n' "${variant_digests[@]}" | sort | sha256sum | awk '{print $1}')"
		fingerprints_json="$(jq -c --arg k "${variant_key}" --arg v "${VARIANT_FINGERPRINTS["${variant_key}"]}" '. + {($k): $v}' <<<"${fingerprints_json}")"
	done
	for image in "${!DIGEST_CACHE[@]}"; do
		all_digests+=("${image}@${DIGEST_CACHE["${image}"]}")
	done
	local BASE_FINGERPRINT
	BASE_FINGERPRINT="$(printf '%s\n' "${all_digests[@]}" | sort | sha256sum | awk '{print $1}')"
	write_output "base_fingerprint=${BASE_FINGERPRINT}"
	write_output "base_fingerprints=${fingerprints_json}"

	declare -A PLATFORM_CACHE=()
	local variant_platforms="{}" buildable
	for variant_key in "${!VARIANT_IMAGES[@]}"; do
		buildable="${VARIANT_PLATFORMS["${variant_key}"]}"
		for image in ${VARIANT_IMAGES["${variant_key}"]}; do
			if [[ -z "${PLATFORM_CACHE["${image}"]:-}" ]]; then
				PLATFORM_CACHE["${image}"]="$(get_image_platforms "${image}")"
			fi
			buildable="$(jq -c --argjson carried "${PLATFORM_CACHE["${image}"]}" 'map(select(IN($carried[])))' <<<"${buildable}")"
		done

		if [[ "${buildable}" == "[]" ]]; then
			echo "None of ${VARIANT_PLATFORMS["${variant_key}"]} is carried by every base image of ${VARIANT_NAMES["${variant_key}"]}" >&2
			return 1
		fi

		variant_platforms="$(jq -c --arg k "${VARIANT_NAMES["${variant_key}"]}" --argjson v "${buildable}" '. + {($k): $v}' <<<"${variant_platforms}")"
	done
	write_output "variant_platforms=${variant_platforms}"

	if [[ "${GITHUB_EVENT_NAME:-}" != "schedule" ]]; then
		write_output "skip=false"
		return 0
	fi

	# Only rebuild the variants whose base images changed since the last push. The
	# fingerprint label is read from the versioned builder tag; images built before the
	# per-variant split carry the legacy all-variants fingerprint, so accept it too.
	local version_no_prefix="${FRANKENPHP_LATEST_TAG#v}"
	local rebuild_variants=() php_full php_minor os builder_tag existing
	for variant_key in "${!VARIANT_FINGERPRINTS[@]}"; do
		php_full="${variant_key%-*}"
		php_minor="${php_full%.*}"
		os="${variant_key##*-}"
		builder_tag="${version_no_prefix}-builder-php${php_minor}-${os}"
		existing="$(get_existing_fingerprint "${builder_tag}")"
		if [[ -n "${existing}" ]] &&
			{ [[ "${existing}" == "${VARIANT_FINGERPRINTS["${variant_key}"]}" ]] || [[ "${existing}" == "${BASE_FINGERPRINT}" ]]; }; then
			continue
		fi
		rebuild_variants+=("${VARIANT_NAMES["${variant_key}"]}")
	done

	if [[ ${#rebuild_variants[@]} -eq 0 ]]; then
		write_output "rebuild_variants=[]"
		write_output "skip=true"
		return 0
	fi

	write_output "rebuild_variants=$(printf '%s\n' "${rebuild_variants[@]}" | sort -u | jq -R . | jq -cs .)"
	write_output "skip=false"
}

main "$@"
