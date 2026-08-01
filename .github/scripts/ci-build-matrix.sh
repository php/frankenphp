#!/usr/bin/env bash
set -euo pipefail

write_output() {
	if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
		echo "$1" >>"${GITHUB_OUTPUT}"
	else
		echo "$1"
	fi
}

kind="${1:?matrix kind is required}"
full_matrix="${2:?full matrix flag is required}"

case "${kind}" in
docker)
	if [[ "${full_matrix}" == "true" ]]; then
		variants="$(
			jq -c '.group.default.targets | map(sub("runner-|builder-"; "")) | unique' <<<"${METADATA}"
		)"
		platforms="$(jq -c 'first(.target[]) | .platforms' <<<"${METADATA}")"
	else
		variants="$(
			jq -c '.group.default.targets
					| map(sub("runner-|builder-"; ""))
					| unique
					| map(select(endswith("-bookworm")))' <<<"${METADATA}"
		)"
		platforms='["linux/amd64"]'
	fi

	write_output "variants=${variants}"
	write_output "platforms=${platforms}"
	;;
static)
	if [[ "${full_matrix}" == "true" ]]; then
		platforms="$(jq -c 'first(.target[]) | .platforms' <<<"${METADATA}")"
	else
		platforms='["linux/amd64"]'
	fi

	write_output "platforms=${platforms}"
	;;
*)
	echo "unknown matrix kind: ${kind}" >&2
	exit 1
	;;
esac
