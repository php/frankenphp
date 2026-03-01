#!/bin/sh

set -e

SUDO=""

if [ -z "${BIN_DIR}" ]; then
	BIN_DIR=$(pwd)
fi

THE_ARCH_BIN=""
DEST=${BIN_DIR}/frankenphp

OS=$(uname -s)
ARCH=$(uname -m)
GNU=""

if ! command -v curl >/dev/null 2>&1; then
	echo "Please install curl to download FrankenPHP"
	exit 1
fi

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
			if [ "$(id -u)" -ne 0 ]; then
				SUDO="sudo"
				echo "❗ Enter your password to grant sudo powers for package installation"
				${SUDO} -v || true
			fi
			${SUDO} dnf -y install https://rpm.henderkes.com/static-php-1-1.noarch.rpm
			${SUDO} dnf -y module enable php-zts:static-8.5 || true
			${SUDO} dnf -y install frankenphp
			echo
			echo "🥳 FrankenPHP installed to ${italic}/usr/bin/frankenphp${normal} successfully."
			echo "❗ The systemd service uses the Caddyfile in ${italic}/etc/frankenphp/Caddyfile${normal}"
			echo "❗ Your php.ini is found in ${italic}/etc/php-zts/php.ini${normal}"
			echo
			echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
			exit 0
		fi

		if command -v apt-get >/dev/null 2>&1; then
			echo "📦 Detected apt-get. Installing FrankenPHP from DEB repository..."
			if [ "$(id -u)" -ne 0 ]; then
				SUDO="sudo"
				echo "❗ Enter your password to grant sudo powers for package installation"
				${SUDO} -v || true
			fi
			${SUDO} sh -c 'curl -fsSL https://pkg.henderkes.com/api/packages/85/debian/repository.key -o /etc/apt/keyrings/static-php85.asc'
			${SUDO} sh -c 'echo "deb [signed-by=/etc/apt/keyrings/static-php85.asc] https://pkg.henderkes.com/api/packages/85/debian php-zts main" | sudo tee -a /etc/apt/sources.list.d/static-php85.list'
			${SUDO} apt-get update
			${SUDO} apt-get -y install frankenphp
			echo
			echo "🥳 FrankenPHP installed to ${italic}/usr/bin/frankenphp${normal} successfully."
			echo "❗ The systemd service uses the Caddyfile in ${italic}/etc/frankenphp/Caddyfile${normal}"
			echo "❗ Your php.ini is found in ${italic}/etc/php-zts/php.ini${normal}"
			echo
			echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
			exit 0
		fi

		if command -v apk >/dev/null 2>&1; then
			echo "📦 Detected apk. Installing FrankenPHP from APK repository..."
			if [ "$(id -u)" -ne 0 ]; then
				SUDO="sudo"
				echo "❗ Enter your password to grant sudo powers for package installation"
				${SUDO} -v || true
			fi

			KEY_URL="https://pkg.henderkes.com/api/packages/85/alpine/key"
			${SUDO} sh -c "cd /etc/apk/keys && curl -JOsS \"$KEY_URL\" 2>/dev/null || true"

			REPO_URL="https://pkg.henderkes.com/api/packages/85/alpine/main/php-zts"
			if grep -q "$REPO_URL" /etc/apk/repositories 2>/dev/null; then
				echo "Repository already exists in /etc/apk/repositories"
			else
				${SUDO} sh -c "echo \"$REPO_URL\" >> /etc/apk/repositories"
				${SUDO} apk update
				echo "Repository added to /etc/apk/repositories"
			fi

			${SUDO} apk add frankenphp
			echo
			echo "🥳 FrankenPHP installed to ${italic}/usr/bin/frankenphp${normal} successfully."
			echo "❗ The OpenRC service uses the Caddyfile in ${italic}/etc/frankenphp/Caddyfile${normal}"
			echo "❗ Your php.ini is found in ${italic}/etc/php-zts/php.ini${normal}"
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
Windows | CYGWIN_NT* | MSYS_NT* | MINGW*)
	if ! command -v unzip >/dev/null 2>&1 && ! command -v powershell.exe >/dev/null 2>&1; then
		echo "❗ Please install unzip to extract FrankenPHP"
		exit 1
	fi

	WIN_ASSET=$(curl -s https://api.github.com/repos/php/frankenphp/releases/latest | \
		grep -o '"name": *"frankenphp-[^"]*-Win32-vs17-x64\.zip"' | head -1 | \
		sed 's/"name": *"//;s/"//')

	if [ -z "${WIN_ASSET}" ]; then
		echo "❗ Could not find a Windows release asset"
		echo "❗ Check https://github.com/php/frankenphp/releases for available downloads"
		exit 1
	fi

	echo "📦 Downloading ${bold}FrankenPHP${normal} for Windows (x64):"

	TMPZIP="/tmp/frankenphp-windows-$$.zip"
	curl -L --progress-bar "https://github.com/php/frankenphp/releases/latest/download/${WIN_ASSET}" -o "${TMPZIP}"

	echo "📂 Extracting to ${italic}${BIN_DIR}${normal}..."
	if command -v unzip >/dev/null 2>&1; then
		unzip -o -q "${TMPZIP}" -d "${BIN_DIR}"
	else
		powershell.exe -Command "Expand-Archive -Force -Path '$(cygpath -w "${TMPZIP}")' -DestinationPath '$(cygpath -w "${BIN_DIR}")'"
	fi
	rm -f "${TMPZIP}"

	echo
	echo "🥳 FrankenPHP downloaded successfully to ${italic}${BIN_DIR}${normal}"
	echo "🔧 Add ${italic}${BIN_DIR}${normal} to your PATH to use ${italic}frankenphp.exe${normal} globally."
	echo
	echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
	exit 0
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

# check if $DEST is writable and suppress an error message
touch "${DEST}" 2>/dev/null

# we need sudo powers to write to DEST
if [ $? -eq 1 ]; then
	echo "❗ You do not have permission to write to ${italic}${DEST}${normal}, enter your password to grant sudo powers"
	SUDO="sudo"
fi

curl -L --progress-bar "https://github.com/php/frankenphp/releases/latest/download/${THE_ARCH_BIN}" -o "${DEST}"

${SUDO} chmod +x "${DEST}"
# Allow binding to ports 80/443 without running as root (if setcap is available)
if command -v setcap >/dev/null 2>&1; then
	${SUDO} setcap 'cap_net_bind_service=+ep' "${DEST}" || true
else
	echo "❗ install setcap (e.g. libcap2-bin) to allow FrankenPHP to bind to ports 80/443 without root:"
	echo "	 ${bold}sudo setcap 'cap_net_bind_service=+ep' \"${DEST}\"${normal}"
fi

echo
echo "🥳 FrankenPHP downloaded successfully to ${italic}${DEST}${normal}"
echo "❗ It uses ${italic}/etc/frankenphp/php.ini${normal} if found."
case ":$PATH:" in
*":$DEST:"*) ;;
*)
	echo "🔧 Move the binary to ${italic}/usr/local/bin/${normal} or another directory in your ${italic}PATH${normal} to use it globally:"
	echo "	${bold}sudo mv ${DEST} /usr/local/bin/${normal}"
	;;
esac

echo
echo "⭐ If you like FrankenPHP, please give it a star on GitHub: ${italic}https://github.com/php/frankenphp${normal}"
