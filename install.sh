#!/bin/sh

set -e

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
	SUDO="sudo"
fi

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
	if [ "${ARCH}" = "aarch64" ] || [ "${ARCH}" = "x86_64" ]; then
		if command -v dnf >/dev/null 2>&1; then
			echo "📦 Detected dnf. Installing FrankenPHP from RPM repository..."
			if [ -n "${SUDO}" ]; then
				echo "❗ Enter your password to grant sudo powers for package installation"
				${SUDO} -v || true
			fi
			${SUDO} dnf -y install https://rpm.henderkes.com/static-php-1-0.noarch.rpm
			${SUDO} dnf -y module enable php-zts:static-8.4 || true
			${SUDO} dnf -y install frankenphp
			echo
			echo "🥳 FrankenPHP installed successfully"
			echo
			echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
			exit 0
		fi

		if command -v apt >/dev/null 2>&1 || command -v apt-get >/dev/null 2>&1; then
			echo "📦 Detected apt. Installing FrankenPHP from DEB repository..."
			if [ -n "${SUDO}" ]; then
				echo "❗ Enter your password to grant sudo powers for package installation"
				${SUDO} -v || true
			fi
			${SUDO} sh -c 'curl -fsSL https://key.henderkes.com/static-php.gpg -o /usr/share/keyrings/static-php.gpg'
			${SUDO} sh -c 'echo "deb [signed-by=/usr/share/keyrings/static-php.gpg] https://deb.henderkes.com/ stable main" > /etc/apt/sources.list.d/static-php.list'
			if command -v apt >/dev/null 2>&1; then
				${SUDO} apt update
				${SUDO} apt -y install frankenphp
			else
				${SUDO} apt-get update
				${SUDO} apt-get -y install frankenphp
			fi
			echo
			echo "🥳 FrankenPHP installed successfully."
			echo
			echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
			exit 0
		fi
	fi

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
	echo "❗ Precompiled binaries are not available for ${ARCH}-${OS}"
	echo "❗ You can compile from sources by following the documentation at: https://frankenphp.dev/docs/compile/"
	exit 1
fi

echo "📦 Downloading ${bold}FrankenPHP${normal} for ${OS}${GNU} (${ARCH}):"

touch "${DEST}" 2>/dev/null

if [ $? -eq 1 ]; then
	echo "❗ You do not have permission to write to ${italic}${DEST}${normal}, enter your password to grant sudo powers"
	SUDO="sudo"
fi

curl -L --progress-bar "https://github.com/php/frankenphp/releases/latest/download/${THE_ARCH_BIN}" -o "${DEST}"

${SUDO} chmod +x "${DEST}"

echo
echo "🥳 FrankenPHP downloaded successfully to ${italic}${DEST}${normal}"
echo "🔧 Move the binary to ${italic}/usr/local/bin/${normal} or another directory in your ${italic}PATH${normal} to use it globally:"
echo "	 ${bold}sudo mv ${DEST} /usr/local/bin/${normal}"
echo
echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
