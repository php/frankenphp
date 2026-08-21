#!/bin/bash

set -o errexit
set -o pipefail

# Build a static (musl) or mostly static (glibc) FrankenPHP binary using
# static-php-cli v3 (https://static-php.dev).
#
# What gets built is described in craft.yml: PHP version, extensions, libraries,
# SAPI and build flags. craft.yml works on its own; this script copies it to
# dist/craft.yml with a few values rewritten, fetches the `spc` binary if needed
# and runs `spc craft`.
#
# Supported variables:
# - PHP_VERSION: PHP version to build (default: the one pinned in craft.yml)
# - PHP_EXTENSIONS: comma-separated extensions (default: the list in craft.yml)
# - PHP_EXTENSION_LIBS: comma-separated extra libraries (default: the list in craft.yml)
# - XCADDY_ARGS: extra Caddy modules to pass to xcaddy (default: the list in craft.yml)
# - FRANKENPHP_VERSION: FrankenPHP version (default: current Git commit)
# - EMBED: path to the PHP app to embed (default: none)
# - CLEAN: when set, rebuild everything from scratch (default: none)
# - DEBUG_SYMBOLS: when set, keep debug symbols (default: none)
# - COMPRESS: when set, pack the resulting Linux binary with UPX; ignored when
#   DEBUG_SYMBOLS is set (default: none)
# - MIMALLOC: when set, use mimalloc as the allocator (default: none)
# - OUTPUT_DIR: where to write the resulting binary (default: dist/)
# - RELEASE: [maintainer only] create a GitHub release if set to 1 (default: none)
#
# The libc to link against (Linux only) is selected with:
# - LIBC: "musl" for a fully static binary (default), "gnu" to link dynamically
#   against the system glibc using the native GCC
#
# LIBC is a shorthand for static-php-cli's own SPC_TARGET and SPC_TOOLCHAIN.
# Setting those directly still works, but both have to be consistent, and they
# must be real environment variables: static-php-cli resolves its toolchain at
# startup, before craft.yml is read.

if ! type "git" >/dev/null 2>&1; then
	echo "The \"git\" command must be installed."
	exit 1
fi

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${CURRENT_DIR}/dist"

# Binary naming, kept stable for install.sh and the GitHub release assets
bin_arch="$(uname -m)"
bin_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
[ "${bin_os}" = "darwin" ] && bin_os="mac"

# static-php-cli release naming
spc_os="$([ "${bin_os}" = "mac" ] && echo "macos" || echo "${bin_os}")"
spc_arch="$([ "${bin_arch}" = "arm64" ] && echo "aarch64" || echo "${bin_arch}")"

unset SPC_LIBC
if [ "${bin_os}" = "linux" ]; then
	case "${LIBC:-musl}" in
	musl)
		: "${SPC_TARGET:=native-native-musl}"
		;;
	gnu)
		: "${SPC_TARGET:=}"
		: "${SPC_TOOLCHAIN:=StaticPHP\Toolchain\GccNativeToolchain}"
		;;
	*)
		echo "LIBC must be \"musl\" or \"gnu\", got \"${LIBC}\"."
		exit 1
		;;
	esac
	export SPC_TARGET
	export SPC_TOOLCHAIN="${SPC_TOOLCHAIN:-}"
fi

# Check out the requested version, if any
if [ -z "${FRANKENPHP_VERSION}" ]; then
	FRANKENPHP_VERSION="$(git -C "${CURRENT_DIR}" rev-parse --verify HEAD)"
elif [ -d "${CURRENT_DIR}/.git/" ]; then
	CURRENT_REF="$(git -C "${CURRENT_DIR}" rev-parse --abbrev-ref HEAD)"
	export CURRENT_REF

	if echo "${FRANKENPHP_VERSION}" | grep -F -q "."; then
		# Tag: trim the "v" prefix, if any
		FRANKENPHP_VERSION=${FRANKENPHP_VERSION#v}
		git -C "${CURRENT_DIR}" checkout "v${FRANKENPHP_VERSION}"
	else
		git -C "${CURRENT_DIR}" checkout "${FRANKENPHP_VERSION}"
	fi
fi
export FRANKENPHP_VERSION

if [ -n "${CLEAN}" ]; then
	# Drop the build root, the sources, the download cache and the spc binary
	rm -rf "${DIST_DIR}"
fi

mkdir -p "${DIST_DIR}"

# macOS dependencies
if type "brew" >/dev/null 2>&1; then
	if [ -n "${RELEASE}" ] && ! type "gh" >/dev/null 2>&1; then
		brew install --formula --quiet gh
	fi
fi

# Fetch the spc binary if we don't have it yet
spc="${DIST_DIR}/spc"
if [ ! -x "${spc}" ]; then
	curl -fsSL -o "${spc}" "https://dl.static-php.dev/v3/spc-bin/nightly/spc-${spc_os}-${spc_arch}"
	chmod +x "${spc}"
fi

# Turn a potentially relative EMBED path into an absolute one
if [ -n "${EMBED}" ] && [[ "${EMBED}" != /* ]]; then
	EMBED="$(cd "${EMBED}" && pwd)"
fi

# When embedding an app that declares its dependencies, build only the
# extensions it actually needs.
if [ -z "${PHP_EXTENSIONS}" ] && [ -n "${EMBED}" ] &&
	[ -f "${EMBED}/composer.json" ] && [ -f "${EMBED}/composer.lock" ] &&
	[ -f "${EMBED}/vendor/composer/installed.json" ]; then
	PHP_EXTENSIONS="$("${spc}" dump-extensions "${EMBED}" --format=text --no-dev)"
fi

libs="${PHP_EXTENSION_LIBS}"
if [ -n "${libs}" ]; then
	libs="${libs}${MIMALLOC:+,mimalloc}"
	# caddy-cbrotli needs the brotli library, always build it
	case ",${libs}," in
	*,brotli,*) ;;
	*) libs="${libs},brotli" ;;
	esac
fi

bool() { [ -n "${1}" ] && echo true || echo false; }

upx_pack=false
if [ -n "${COMPRESS}" ] && [ -z "${DEBUG_SYMBOLS}" ] && [ "${bin_os}" = "linux" ]; then
	upx_pack=true
fi

# Rewrite the values craft.yml cannot hold a working default for, and apply the
# optional overrides. Each case replaces a whole value, so running this over an
# already rendered craft.yml gives the same result.
craft="${DIST_DIR}/craft.yml"
: >"${craft}"
while IFS= read -r line || [ -n "${line}" ]; do
	case "${line}" in
	"php-version:"*)
		[ -n "${PHP_VERSION}" ] && line="php-version: \"${PHP_VERSION}\""
		;;
	"extensions:"*)
		[ -n "${PHP_EXTENSIONS}" ] && line="extensions: \"${PHP_EXTENSIONS}\""
		;;
	"packages:"*)
		packages="${PHP_EXTENSION_LIBS}"
		if [ -z "${packages}" ]; then
			packages="${line#packages:}"
			packages="${packages# }"
			packages="${packages#\"}"
			packages="${packages%\"}"
		fi
		if [ -n "${MIMALLOC}" ]; then
			case ",${packages}," in
			*,mimalloc,*) ;;
			*) packages="${packages},mimalloc" ;;
			esac
		fi
		# caddy-cbrotli needs the brotli library, always build it
		case ",${packages}," in
		*,brotli,*) ;;
		*) packages="${packages},brotli" ;;
		esac
		line="packages: \"${packages}\""
		;;
	"clean-build:"*)
		line="clean-build: $(bool "${CLEAN}")"
		;;
	*"no-strip:"*)
		line="  no-strip: $(bool "${DEBUG_SYMBOLS}")"
		;;
	*"with-upx-pack:"*)
		line="  with-upx-pack: ${upx_pack}"
		;;
	*"with-frankenphp-app:"*)
		line="  with-frankenphp-app: \"${EMBED}\""
		;;
	*"custom-local:"*)
		line="  custom-local: [\"frankenphp:${CURRENT_DIR}\"]"
		;;
	*'- "frankenphp:'*)
		# stale list entry, custom-local is rewritten as a one-liner above
		continue
		;;
	*"SPC_CMD_VAR_FRANKENPHP_XCADDY_MODULES:"*)
		[ -n "${XCADDY_ARGS}" ] && line="  SPC_CMD_VAR_FRANKENPHP_XCADDY_MODULES: \"${XCADDY_ARGS}\""
		;;
	esac

	printf '%s\n' "${line}" >>"${craft}"
done <"${CURRENT_DIR}/craft.yml"

cd "${DIST_DIR}"
"${spc}" craft "${craft}"

bin="${OUTPUT_DIR:-${DIST_DIR}}/frankenphp-${bin_os}-${bin_arch}"
mkdir -p "$(dirname "${bin}")"
cp "${DIST_DIR}/buildroot/bin/frankenphp" "${bin}"
"${bin}" version
"${bin}" build-info

if [ -n "${RELEASE}" ]; then
	gh release upload "v${FRANKENPHP_VERSION}" "${bin}" --repo dunglas/frankenphp --clobber
fi

if [ -n "${CURRENT_REF}" ]; then
	git -C "${CURRENT_DIR}" checkout "${CURRENT_REF}"
fi
