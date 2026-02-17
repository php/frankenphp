package frankenphp_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

func parallelBenchmark(
	b *testing.B,
	options []frankenphp.Option,
	requestOptions []frankenphp.RequestOption,
	newRequest func() *http.Request,
) {
	require.NoError(b, frankenphp.Init(options...))
	b.Cleanup(frankenphp.Shutdown)

	handler := func(w http.ResponseWriter, r *http.Request) {
		req, err := frankenphp.NewRequestWithContext(r, requestOptions...)
		require.NoError(b, err)

		require.NoError(b, frankenphp.ServeHTTP(w, req))
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := newRequest()
			w := httptest.NewRecorder()
			handler(w, req)
		}
	})
}

func testDataDir() string {
	cwd, _ := os.Getwd()

	return cwd + "/testdata/"
}

func BenchmarkHelloWorld(b *testing.B) {
	parallelBenchmark(
		b,
		[]frankenphp.Option{
			frankenphp.WithNumThreads(3),
		},
		[]frankenphp.RequestOption{
			frankenphp.WithRequestDocumentRoot(testDataDir(), false),
		},
		func() *http.Request {
			return httptest.NewRequest("GET", "http://example.com/hello.php", nil)
		},
	)
}

func BenchmarkHelloWorldWorker(b *testing.B) {
	parallelBenchmark(
		b,
		[]frankenphp.Option{
			frankenphp.WithNumThreads(4),
			frankenphp.WithWorkers("worker", "testdata/worker-with-counter.php", 3),
		},
		[]frankenphp.RequestOption{
			frankenphp.WithRequestDocumentRoot(testDataDir(), false),
			frankenphp.WithWorkerName("worker"),
		},
		func() *http.Request {
			return httptest.NewRequest("GET", "http://example.com/worker", nil)
		},
	)
}

func BenchmarkEcho(b *testing.B) {
	const body = `{
		"squadName": "Super hero squad",
		"homeTown": "Metro City",
		"formed": 2016,
		"secretBase": "Super tower",
		"active": true,
		"members": [
		  {
			"name": "Molecule Man",
			"age": 29,
			"secretIdentity": "Dan Jukes",
			"powers": ["Radiation resistance", "Turning tiny", "Radiation blast"]
		  },
		  {
			"name": "Madame Uppercut",
			"age": 39,
			"secretIdentity": "Jane Wilson",
			"powers": [
			  "Million tonne punch",
			  "Damage resistance",
			  "Superhuman reflexes"
			]
		  },
		  {
			"name": "Eternal Flame",
			"age": 1000000,
			"secretIdentity": "Unknown",
			"powers": [
			  "Immortality",
			  "Heat Immunity",
			  "Inferno",
			  "Teleportation",
			  "Interdimensional travel"
			]
		  }
		]
	  }`
	parallelBenchmark(
		b,
		[]frankenphp.Option{
			frankenphp.WithNumThreads(3),
		},
		[]frankenphp.RequestOption{
			frankenphp.WithRequestDocumentRoot(testDataDir(), false),
		},
		func() *http.Request {
			r := strings.NewReader(body)
			return httptest.NewRequest("POST", "http://example.com/echo.php", r)
		},
	)
}

func BenchmarkEchoOften(b *testing.B) {
	parallelBenchmark(
		b,
		[]frankenphp.Option{
			frankenphp.WithNumThreads(3),
		},
		[]frankenphp.RequestOption{
			frankenphp.WithRequestDocumentRoot(testDataDir(), false),
		},
		func() *http.Request {
			return httptest.NewRequest("POST", "http://example.com/echo-often.php?count=1000", nil)
		},
	)
}

func BenchmarkServerSuperGlobal_filters(b *testing.B) {
	benchmarkServerSuperGlobal(b, true, false)
}

func BenchmarkServerSuperGlobal_nofilters(b *testing.B) {
	benchmarkServerSuperGlobal(b, false, false)
}

func BenchmarkServerSuperGlobal_workerfilters(b *testing.B) {
	benchmarkServerSuperGlobal(b, true, true)
}

func BenchmarkServerSuperGlobal_workernofilters(b *testing.B) {
	benchmarkServerSuperGlobal(b, false, true)
}

func benchmarkServerSuperGlobal(b *testing.B, withFilters bool, withWorker bool) {
	// Mimics headers of a request sent by Firefox to GitHub
	header := http.Header{}
	header.Add(strings.Clone("Accept"), strings.Clone("text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"))
	header.Add(strings.Clone("Accept-Encoding"), strings.Clone("gzip, deflate, br"))
	header.Add(strings.Clone("Accept-Language"), strings.Clone("fr,fr-FR;q=0.8,en-US;q=0.5,en;q=0.3"))
	header.Add(strings.Clone("Cache-Control"), strings.Clone("no-cache"))
	header.Add(strings.Clone("Connection"), strings.Clone("keep-alive"))
	header.Add(strings.Clone("Cookie"), strings.Clone("user_session=myrandomuuid; __Host-user_session_same_site=myotherrandomuuid; dotcom_user=dunglas; logged_in=yes; _foo=barbarbarbarbarbar; _device_id=anotherrandomuuid; color_mode=foobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobarfoobar; preferred_color_mode=light; tz=Europe%2FParis; has_recent_activity=1"))
	header.Add(strings.Clone("DNT"), strings.Clone("1"))
	header.Add(strings.Clone("Host"), strings.Clone("example.com"))
	header.Add(strings.Clone("Pragma"), strings.Clone("no-cache"))
	header.Add(strings.Clone("Sec-Fetch-Dest"), strings.Clone("document"))
	header.Add(strings.Clone("Sec-Fetch-Mode"), strings.Clone("navigate"))
	header.Add(strings.Clone("Sec-Fetch-Site"), strings.Clone("cross-site"))
	header.Add(strings.Clone("Sec-GPC"), strings.Clone("1"))
	header.Add(strings.Clone("Upgrade-Insecure-Requests"), strings.Clone("1"))
	header.Add(strings.Clone("User-Agent"), strings.Clone("Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:122.0) Gecko/20100101 Firefox/122.0"))

	// Env vars available in a typical Docker container
	env := map[string]string{
		"HOSTNAME":        "a88e81aa22e4",
		"PHP_INI_DIR":     "/usr/local/etc/php",
		"HOME":            "/root",
		"GODEBUG":         "cgocheck=0",
		"PHP_LDFLAGS":     "-Wl,-O1 -pie",
		"PHP_CFLAGS":      "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64",
		"PHP_VERSION":     "8.3.2",
		"GPG_KEYS":        "1198C0117593497A5EC5C199286AF1F9897469DC C28D937575603EB4ABB725861C0779DC5C0A9DE4 AFD8691FDAEDF03BDF6E460563F15A9B715376CA",
		"PHP_CPPFLAGS":    "-fstack-protector-strong -fpic -fpie -O2 -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64",
		"PHP_ASC_URL":     "https://www.php.net/distributions/php-8.3.2.tar.xz.asc",
		"PHP_URL":         "https://www.php.net/distributions/php-8.3.2.tar.xz",
		"PATH":            "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"XDG_CONFIG_HOME": "/config",
		"XDG_DATA_HOME":   "/data",
		"PHPIZE_DEPS":     "autoconf dpkg-dev file g++ gcc libc-dev make pkg-config re2c",
		"PWD":             "/app",
		"PHP_SHA256":      "4ffa3e44afc9c590e28dc0d2d31fc61f0139f8b335f11880a121b9f9b9f0634e",
	}

	preparedEnv := frankenphp.PrepareEnv(env)

	opts := []frankenphp.Option{
		frankenphp.WithNumThreads(6),
		frankenphp.WithWorkers("worker", "testdata/worker-with-counter.php", 3),
	}
	if withFilters {
		opts = append(opts, frankenphp.WithPhpIni(map[string]string{"filter.default": "unsafe.raw"}))
	}

	rOpts := []frankenphp.RequestOption{
		frankenphp.WithRequestDocumentRoot(testDataDir(), false),
		frankenphp.WithRequestEnv(preparedEnv),
	}
	if withWorker {
		rOpts = append(rOpts, frankenphp.WithWorkerName("worker"))
	}

	parallelBenchmark(
		b,
		opts,
		rOpts,
		func() *http.Request {
			r := httptest.NewRequest("GET", "http://example.com/server-variable.php", nil)
			r.Header = header
			return r
		},
	)
}

func BenchmarkUncommonHeaders(b *testing.B) {
	header := http.Header{}
	for i := 0; i < 100; i++ {
		header.Add(strings.Clone("X-Custom-"+strconv.Itoa(i)), strings.Clone("Foo"))
	}

	parallelBenchmark(
		b,
		[]frankenphp.Option{
			frankenphp.WithNumThreads(3),
		},
		[]frankenphp.RequestOption{
			frankenphp.WithRequestDocumentRoot(testDataDir(), false),
		},
		func() *http.Request {
			r := httptest.NewRequest("GET", "http://example.com/server-variable.php", nil)
			r.Header = header
			return r
		},
	)
}
