#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

assert_output() {
	local kind="$1"
	local full_matrix="$2"
	local expected="$3"
	local actual

	actual="$(METADATA="${TEST_METADATA}" "${SCRIPT_DIR}/ci-build-matrix.sh" "${kind}" "${full_matrix}")"
	if [[ "${actual}" != "${expected}" ]]; then
		printf 'expected:\n%s\nactual:\n%s\n' "${expected}" "${actual}" >&2
		return 1
	fi
}

TEST_METADATA='{
	"group": {
		"default": {
			"targets": [
				"builder-php-8-2-bookworm",
				"builder-php-8-2-trixie",
				"builder-php-8-3-bookworm",
				"builder-php-8-3-trixie",
				"runner-php-8-2-bookworm",
				"runner-php-8-2-trixie",
				"runner-php-8-3-bookworm",
				"runner-php-8-3-trixie"
			]
		}
	},
	"target": {
		"builder-php-8-2-bookworm": {
			"platforms": ["linux/amd64", "linux/arm64"]
		},
		"static-builder-musl": {
			"platforms": ["linux/amd64", "linux/arm64"]
		}
	}
}'

assert_output docker false $'variants=["php-8-2-bookworm","php-8-3-bookworm"]\nplatforms=["linux/amd64"]'
assert_output docker true $'variants=["php-8-2-bookworm","php-8-2-trixie","php-8-3-bookworm","php-8-3-trixie"]\nplatforms=["linux/amd64","linux/arm64"]'
assert_output static false 'platforms=["linux/amd64"]'
assert_output static true 'platforms=["linux/amd64","linux/arm64"]'
