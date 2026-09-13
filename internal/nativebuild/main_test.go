package main

import (
	"reflect"
	"testing"
)

func TestCompilerFlags(t *testing.T) {
	for _, test := range []struct {
		cc    []string
		flags string
	}{
		{[]string{"cc"}, "-fPIC -m64"},
		{[]string{"cc", "-D_GNU_SOURCE"}, ". -fPIC -m64"},
		{[]string{"zig", "cc", "-target", "x86_64-linux-musl"}, "x86_64-linux-musl -I . -fPIC -m64"},
		{[]string{"cc", "-D_GNU_SOURCE"}, "-fPIC -m64"},
	} {
		got, err := compilerFlags(test.cc, test.flags)
		if err != nil || !reflect.DeepEqual(got, []string{"-fPIC", "-m64"}) {
			t.Errorf("compilerFlags(%q, %q) = %q, %v", test.cc, test.flags, got, err)
		}
	}
}

func TestLinkerFlags(t *testing.T) {
	for _, test := range []struct {
		name, flags string
		want        []string
	}{
		{"dynamic", `-w -s -X 'main.Version=FrankenPHP dev'`, []string{"-Wl,--strip-debug", "-s"}},
		{"static", `-linkmode=external -extldflags '-static-pie -Wl,-z,stack-size=0x80000 -Wl,--export-dynamic-symbol=php_printf' -s`, []string{"-static-pie", "-Wl,-z,stack-size=0x80000", "-Wl,--export-dynamic-symbol=php_printf", "-s"}},
		{"quoted path", `-extldflags="-L'/path with spaces' -Wl,-rpath,/lib/php"`, []string{"-L/path with spaces", "-Wl,-rpath,/lib/php"}},
		{"all packages", `all=-extldflags=-pie`, []string{"-pie"}},
		{"debug", `-s=false -w=false`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := linkerFlags(test.flags)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("linkerFlags(%q) = %q, %v; want %q", test.flags, got, err, test.want)
			}
		})
	}
	for _, invalid := range []string{`-extldflags`, `-extldflags 'unterminated`, `-extldflags="'unterminated"`} {
		if _, err := linkerFlags(invalid); err == nil {
			t.Errorf("linkerFlags(%q) should reject invalid flags", invalid)
		}
	}
}
