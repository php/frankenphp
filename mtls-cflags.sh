#!/bin/sh
# Prints -mtls-size=12 if the toolchain compiles and links local-exec TLS with it (excludes non-AArch64 and ld.lld).
d="$(mktemp -d)" || exit 0
printf '__thread __attribute__((tls_model("local-exec"))) int v;\nint main(void){v=1;return v;}\n' >"$d/p.c"
"${CC:-cc}" -O2 -mtls-size=12 "$d/p.c" -o "$d/p" >/dev/null 2>&1 && printf -- -mtls-size=12
rm -rf "$d"
