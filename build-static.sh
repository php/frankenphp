#!/bin/bash

set -o errexit
set -x

if ! type "git" >/dev/null 2>&1; then
	echo "The \"git\" command must be installed."
	exit 1
fi

CURRENT_DIR=$(pwd)

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
[ "$os" = "darwin" ] && os="mac"

# Supported variables:
# - PHP_VERSION: PHP version to build (default: "8.4")
# - PHP_EXTENSIONS: PHP extensions to build (default: ${defaultExtensions} set below)
# - PHP_EXTENSION_LIBS: PHP extension libraries to build (default: ${defaultExtensionLibs} set below)
# - FRANKENPHP_VERSION: FrankenPHP version (default: current Git commit)
# - EMBED: Path to the PHP app to embed (default: none)
# - DEBUG_SYMBOLS: Enable debug symbols if set to 1 (default: none)
# - MIMALLOC: Use mimalloc as the allocator if set to 1 (default: none)
# - XCADDY_ARGS: Additional arguments to pass to xcaddy
# - RELEASE: [maintainer only] Create a GitHub release if set to 1 (default: none)

# - SPC_REL_TYPE: Release type to download (accept "source" and "binary", default: "source")
# - SPC_OPT_BUILD_ARGS: Additional arguments to pass to spc build
# - SPC_OPT_DOWNLOAD_ARGS: Additional arguments to pass to spc download
# - SPC_LIBC: Set to glibc to build with GNU toolchain (default: musl)

# init spc command, if we use spc binary, just use it instead of fetching source
if [ -z "${SPC_REL_TYPE}" ]; then
	SPC_REL_TYPE="source"
fi
# init spc libc
if [ -z "${SPC_LIBC}" ]; then
	if [ "${os}" = "linux" ]; then
		SPC_LIBC="musl"
	fi
fi
# init spc build additional args
if [ -z "${SPC_OPT_BUILD_ARGS}" ]; then
	SPC_OPT_BUILD_ARGS=""
fi
if [ "${SPC_LIBC}" = "musl" ] && [[ "${SPC_OPT_BUILD_ARGS}" != *"--disable-opcache-jit"* ]]; then
	SPC_OPT_BUILD_ARGS="${SPC_OPT_BUILD_ARGS} --disable-opcache-jit"
fi
# init spc download additional args
if [ -z "${SPC_OPT_DOWNLOAD_ARGS}" ]; then
	SPC_OPT_DOWNLOAD_ARGS="--ignore-cache-sources=php-src --retry 5"
	if [ "${SPC_LIBC}" = "musl" ]; then
		SPC_OPT_DOWNLOAD_ARGS="${SPC_OPT_DOWNLOAD_ARGS} --prefer-pre-built"
	fi
fi
# if we need debug symbols, disable strip
if [ -n "${DEBUG_SYMBOLS}" ]; then
	SPC_OPT_BUILD_ARGS="${SPC_OPT_BUILD_ARGS} --no-strip"
fi
# php version to build
if [ -z "${PHP_VERSION}" ]; then
	get_latest_php_version() {
		input="$1"
		json=$(curl -s "https://www.php.net/releases/index.php?json&version=$input")
		latest=$(echo "$json" | jq -r '.version')

		if [[ "$latest" == "$input"* ]]; then
			echo "$latest"
		else
			echo "$input"
		fi
	}

	PHP_VERSION="$(get_latest_php_version "8.5")"
	export PHP_VERSION
fi
# default extension set
defaultExtensions="amqp,apcu,ast,bcmath,brotli,bz2,calendar,ctype,curl,dba,dom,exif,fileinfo,filter,ftp,gd,gmp,gettext,iconv,igbinary,imagick,intl,ldap,lz4,mbregex,mbstring,memcached,mysqli,mysqlnd,opcache,openssl,password-argon2,parallel,pcntl,pdo,pdo_mysql,pdo_pgsql,pdo_sqlite,pgsql,phar,posix,protobuf,readline,redis,session,shmop,simplexml,soap,sockets,sodium,sqlite3,ssh2,sysvmsg,sysvsem,sysvshm,tidy,tokenizer,xlswriter,xml,xmlreader,xmlwriter,xsl,xz,zip,zlib,yaml,zstd"
defaultExtensionLibs="libavif,nghttp2,nghttp3,ngtcp2,watcher"

if [ -z "${FRANKENPHP_VERSION}" ]; then
	FRANKENPHP_VERSION="$(git rev-parse --verify HEAD)"
	export FRANKENPHP_VERSION
elif [ -d ".git/" ]; then
	CURRENT_REF="$(git rev-parse --abbrev-ref HEAD)"
	export CURRENT_REF

	if echo "${FRANKENPHP_VERSION}" | grep -F -q "."; then
		# Tag

		# Trim "v" prefix if any
		FRANKENPHP_VERSION=${FRANKENPHP_VERSION#v}
		export FRANKENPHP_VERSION

		git checkout "v${FRANKENPHP_VERSION}"
	else
		git checkout "${FRANKENPHP_VERSION}"
	fi
fi

if [ -n "${CLEAN}" ]; then
	rm -Rf dist/
	go clean -cache
fi

mkdir -p dist/
cd dist/

if type "brew" >/dev/null 2>&1; then
	if ! type "composer" >/dev/null; then
		packages="composer"
	fi
	if ! type "go" >/dev/null 2>&1; then
		packages="${packages} go"
	fi
	if [ -n "${RELEASE}" ] && ! type "gh" >/dev/null 2>&1; then
		packages="${packages} gh"
	fi

	if [ -n "${packages}" ]; then
		# shellcheck disable=SC2086
		brew install --formula --quiet ${packages}
	fi
fi

if [ "${SPC_REL_TYPE}" = "binary" ]; then
	mkdir -p static-php-cli/
	cd static-php-cli/
	if [[ "${arch}" =~ "arm" ]]; then
		dl_arch="aarch64"
	else
		dl_arch="${arch}"
	fi
	curl -o spc -fsSL "https://dl.static-php.dev/static-php-cli/spc-bin/nightly/spc-linux-${dl_arch}"
	chmod +x spc
	spcCommand="./spc"
elif [ -d "static-php-cli/src" ]; then
	cd static-php-cli/
	git pull
	composer install --no-dev -a --no-interaction
	spcCommand="./bin/spc"
else
	git clone --depth 1 https://github.com/crazywhalecc/static-php-cli --branch main
	cd static-php-cli/
	composer install --no-dev -a --no-interaction
	spcCommand="./bin/spc"
fi

# turn potentially relative EMBED path into absolute path
if [ -n "${EMBED}" ]; then
	if [[ "${EMBED}" != /* ]]; then
		EMBED="${CURRENT_DIR}/${EMBED}"
	fi
fi

# Extensions to build
if [ -z "${PHP_EXTENSIONS}" ]; then
	# enable EMBED mode, first check if project has dumped extensions
	if [ -n "${EMBED}" ] && [ -f "${EMBED}/composer.json" ] && [ -f "${EMBED}/composer.lock" ] && [ -f "${EMBED}/vendor/composer/installed.json" ]; then
		cd "${EMBED}"
		# read the extensions using spc dump-extensions
		PHP_EXTENSIONS=$(${spcCommand} dump-extensions "${EMBED}" --format=text --no-dev --no-ext-output="${defaultExtensions}")
	else
		PHP_EXTENSIONS="${defaultExtensions}"
	fi
fi

# Additional libraries to build
if [ -z "${PHP_EXTENSION_LIBS}" ]; then
	PHP_EXTENSION_LIBS="${defaultExtensionLibs}"
fi

# The Brotli library must always be built as it is required by http://github.com/dunglas/caddy-cbrotli
if ! echo "${PHP_EXTENSION_LIBS}" | grep -q "\bbrotli\b"; then
	PHP_EXTENSION_LIBS="${PHP_EXTENSION_LIBS},brotli"
fi

# The mimalloc library must be built if MIMALLOC is true
if [ -n "${MIMALLOC}" ]; then
	if ! echo "${PHP_EXTENSION_LIBS}" | grep -q "\bmimalloc\b"; then
		PHP_EXTENSION_LIBS="${PHP_EXTENSION_LIBS},mimalloc"
	fi
fi

# Embed PHP app, if any
if [ -n "${EMBED}" ] && [ -d "${EMBED}" ]; then
	# shellcheck disable=SC2089
	SPC_OPT_BUILD_ARGS="${SPC_OPT_BUILD_ARGS} --with-frankenphp-app='${EMBED}'"
fi

SPC_OPT_INSTALL_ARGS="go-xcaddy"
if [ -z "${DEBUG_SYMBOLS}" ] && [ -z "${NO_COMPRESS}" ] && [ "${os}" = "linux" ]; then
	SPC_OPT_BUILD_ARGS="${SPC_OPT_BUILD_ARGS} --with-upx-pack"
	SPC_OPT_INSTALL_ARGS="${SPC_OPT_INSTALL_ARGS} upx"
fi

export SPC_DEFAULT_C_FLAGS="-fPIC -O2"
if [ -n "${DEBUG_SYMBOLS}" ]; then
	SPC_CMD_VAR_PHP_MAKE_EXTRA_CFLAGS="${SPC_CMD_VAR_PHP_MAKE_EXTRA_CFLAGS} -fPIE -g"
else
	SPC_CMD_VAR_PHP_MAKE_EXTRA_CFLAGS="${SPC_CMD_VAR_PHP_MAKE_EXTRA_CFLAGS} -fPIE -fstack-protector-strong -O2 -w -s"
fi
export SPC_CMD_VAR_PHP_MAKE_EXTRA_CFLAGS
if [ -z "$SPC_CMD_VAR_FRANKENPHP_XCADDY_MODULES" ]; then
	export SPC_CMD_VAR_FRANKENPHP_XCADDY_MODULES="--with github.com/dunglas/mercure/caddy --with github.com/dunglas/vulcain/caddy --with github.com/dunglas/caddy-cbrotli"
fi

# Build FrankenPHP
${spcCommand} doctor --auto-fix
for pkg in ${SPC_OPT_INSTALL_ARGS}; do
	${spcCommand} install-pkg "${pkg}"
done
# shellcheck disable=SC2086
${spcCommand} download --with-php="${PHP_VERSION}" --for-extensions="${PHP_EXTENSIONS}" --for-libs="${PHP_EXTENSION_LIBS}" ${SPC_OPT_DOWNLOAD_ARGS}
export FRANKENPHP_SOURCE_PATH="${CURRENT_DIR}"
# shellcheck disable=SC2086,SC2090
${spcCommand} build --enable-zts --build-embed --build-frankenphp ${SPC_OPT_BUILD_ARGS} "${PHP_EXTENSIONS}" --with-libs="${PHP_EXTENSION_LIBS}"

if [ -n "$CI" ]; then
	rm -rf ./downloads
	rm -rf ./source
fi

cd ../..

bin="dist/frankenphp-${os}-${arch}"
cp "dist/static-php-cli/buildroot/bin/frankenphp" "${bin}"
"${bin}" version
"${bin}" build-info

if [ -n "${RELEASE}" ]; then
	gh release upload "v${FRANKENPHP_VERSION}" "${bin}" --repo dunglas/frankenphp --clobber
fi

if [ -n "${CURRENT_REF}" ]; then
	git checkout "${CURRENT_REF}"
fi
