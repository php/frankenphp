#!/bin/sh
set -eu

readonly TEST_PHP_CONFIG="custom-php-config"
readonly TEST_PATH_PHP_CONFIG_ERROR="go.sh ignored PHP_CONFIG"
readonly TEST_PHP_INCLUDE_FLAG="-I/custom/php/include"
readonly TEST_PHP_LDFLAG="-Wl,-rpath,/custom/php/lib"
readonly TEST_PHP_LIBS="-lxml2"
readonly TEST_EXISTING_CFLAGS="-Dexisting"
readonly TEST_EXISTING_LDFLAGS="-lexisting"
readonly EXPECTED_CGO_LDFLAGS="${TEST_EXISTING_LDFLAGS} ${TEST_PHP_LDFLAG} ${TEST_PHP_LIBS}"

ROOT_DIR="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
TMP_DIR="$(mktemp -d)"
OUTPUT_FILE="${TMP_DIR}/go-env"

cleanup() {
	rm -rf "${TMP_DIR}"
}
trap cleanup EXIT INT TERM

cat >"${TMP_DIR}/php-config" <<PHP_CONFIG_PATH
#!/bin/sh
echo "${TEST_PATH_PHP_CONFIG_ERROR}" >&2
exit 42
PHP_CONFIG_PATH

cat >"${TMP_DIR}/${TEST_PHP_CONFIG}" <<PHP_CONFIG
#!/bin/sh
case "\$1" in
	--includes)
		printf '%s\n' "${TEST_PHP_INCLUDE_FLAG}"
		;;
	--ldflags)
		printf '%s\n' "${TEST_PHP_LDFLAG}"
		;;
	--libs)
		printf '%s\n' "${TEST_PHP_LIBS}"
		;;
	*)
		echo "unexpected php-config argument: \$1" >&2
		exit 2
		;;
esac
PHP_CONFIG

cat >"${TMP_DIR}/go" <<'GO'
#!/bin/sh
{
	printf 'CGO_CFLAGS=%s\n' "${CGO_CFLAGS}"
	printf 'CGO_LDFLAGS=%s\n' "${CGO_LDFLAGS}"
	printf 'GOFLAGS=%s\n' "${GOFLAGS}"
	printf 'ARGS=%s\n' "$*"
} >"${OUTPUT_FILE}"
GO

chmod +x "${TMP_DIR}/php-config" "${TMP_DIR}/${TEST_PHP_CONFIG}" "${TMP_DIR}/go"

PATH="${TMP_DIR}:${PATH}" \
	CGO_CFLAGS="${TEST_EXISTING_CFLAGS}" \
	CGO_LDFLAGS="${TEST_EXISTING_LDFLAGS}" \
	PHP_CONFIG="${TMP_DIR}/${TEST_PHP_CONFIG}" \
	OUTPUT_FILE="${OUTPUT_FILE}" \
	"${ROOT_DIR}/go.sh" build -v

grep -F "CGO_CFLAGS=${TEST_EXISTING_CFLAGS} ${TEST_PHP_INCLUDE_FLAG}" "${OUTPUT_FILE}"
grep -Fx "CGO_LDFLAGS=${EXPECTED_CGO_LDFLAGS}" "${OUTPUT_FILE}"
grep -Fx "ARGS=build -v" "${OUTPUT_FILE}"
