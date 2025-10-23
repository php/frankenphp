#!/bin/sh

set -e

if [ -z "${BIN_DIR}" ]; then
	BIN_DIR=$(pwd)
fi

THE_ARCH_BIN=""
DEST=${BIN_DIR}/frankenphp

OS=$(uname -s)
ARCH=$(uname -m)
GNU=""

if type "tput" >/dev/null 2>&1; then
	bold=$(tput bold || true)
	italic=$(tput sitm || true)
	normal=$(tput sgr0 || true)
fi

case ${OS} in
Linux*)
	case ${ARCH} in
	aarch64)
		THE_ARCH_BIN="frankenphp-linux-aarch64"
		;;
	x86_64)
		THE_ARCH_BIN="frankenphp-linux-x86_64"
		;;
	*)
		THE_ARCH_BIN=""
		;;
	esac

	if getconf GNU_LIBC_VERSION >/dev/null 2>&1; then
		THE_ARCH_BIN="${THE_ARCH_BIN}-gnu"
		GNU=" (glibc)"
	fi
	;;
Darwin*)
	case ${ARCH} in
	arm64)
		THE_ARCH_BIN="frankenphp-mac-arm64"
		;;
	*)
		THE_ARCH_BIN="frankenphp-mac-x86_64"
		;;
	esac
	;;
Windows | MINGW64_NT*)
	echo "❗ Use WSL to run FrankenPHP on Windows: https://learn.microsoft.com/windows/wsl/"
	exit 1
	;;
*)
	THE_ARCH_BIN=""
	;;
esac

if [ -z "${THE_ARCH_BIN}" ]; then
	echo "❗ FrankenPHP is not supported on ${OS} and ${ARCH}"
	echo "❗ You can attempt to compile from sources by following the documentation at: https://frankenphp.dev/docs/compile/"
	exit 1
fi

SUDO=""

echo "📦 Downloading ${bold}FrankenPHP${normal} for ${OS}${GNU} (${ARCH}):"

# check if $DEST is writable and suppress an error message
touch "${DEST}" 2>/dev/null

# we need sudo powers to write to DEST
if [ $? -eq 1 ]; then
	echo "❗ You do not have permission to write to ${italic}${DEST}${normal}, enter your password to grant sudo powers"
	SUDO="sudo"
fi

if type "curl" >/dev/null 2>&1; then
	curl -L --progress-bar "https://github.com/php/frankenphp/releases/latest/download/${THE_ARCH_BIN}" -o "${DEST}"
elif type "wget" >/dev/null 2>&1; then
	${SUDO} wget "https://github.com/php/frankenphp/releases/latest/download/${THE_ARCH_BIN}" -O "${DEST}"
else
	echo "❗ Please install ${italic}curl${normal} or ${italic}wget${normal} to download FrankenPHP"
	exit 1
fi

${SUDO} chmod +x "${DEST}"

echo
echo "🥳 FrankenPHP downloaded successfully to ${italic}${DEST}${normal}"
echo "🔧 Move the binary to ${italic}/usr/local/bin/${normal} or another directory in your ${italic}PATH${normal} to use it globally:"
echo "   ${bold}sudo mv ${DEST} /usr/local/bin/${normal}"
echo
echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
