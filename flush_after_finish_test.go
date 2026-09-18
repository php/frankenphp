package frankenphp_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlushAfterFinishedConnection(t *testing.T) {
	for _, worker := range []bool{false, true} {
		t.Run(fmt.Sprintf("worker=%t", worker), func(t *testing.T) {
			root := t.TempDir()
			resume := filepath.Join(root, "resume")
			script := filepath.Join(root, "index.php")
			body := `<?php
$handler = static function () {
    ignore_user_abort(false);
    echo "before finish";
    frankenphp_finish_request();
    $until = microtime(true) + 5;
    while (!file_exists(__DIR__."/resume")) {
        if (microtime(true) >= $until) {
            throw new RuntimeException("timed out waiting for the connection to close");
        }
        usleep(1000);
        clearstatcache();
    }
    flush();
    error_log("after flush: ".connection_status());
};
`
			if worker {
				body += "while (frankenphp_handle_request($handler)) {}\n"
			} else {
				body += "$handler();\n"
			}
			require.NoError(t, os.WriteFile(script, []byte(body), 0600))
			logger, logs := newTestLogger(t)
			opts := []frankenphp.Option{frankenphp.WithLogger(logger), frankenphp.WithNumThreads(2)}
			requestOpts := []frankenphp.RequestOption{frankenphp.WithRequestDocumentRoot(root, false)}
			if worker {
				opts = append(opts, frankenphp.WithWorkers("flush", script, 1))
				requestOpts = append(requestOpts, frankenphp.WithWorkerName("flush"))
			}
			require.NoError(t, frankenphp.Init(opts...))
			defer frankenphp.Shutdown()

			closed := make(chan struct{})
			var closeOnce sync.Once
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fr, err := frankenphp.NewRequestWithContext(r, requestOpts...)
				if !assert.NoError(t, err) {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				assert.NoError(t, frankenphp.ServeHTTP(w, fr))
			}))
			srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					closeOnce.Do(func() { close(closed) })
				}
			}
			srv.Start()
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/index.php", nil)
			require.NoError(t, err)
			req.Close = true
			client := srv.Client()
			client.Timeout = 5 * time.Second
			resp, err := client.Do(req)
			require.NoError(t, err)
			got, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, "before finish", string(got))
			select {
			case <-closed:
			case <-time.After(5 * time.Second):
				t.Fatal("connection did not close")
			}
			require.NoError(t, os.WriteFile(resume, nil, 0600))
			require.Eventually(t, func() bool {
				return strings.Contains(logs.String(), "after flush: 0")
			}, 5*time.Second, time.Millisecond, "flush must preserve normal post-response PHP execution")
		})
	}
}
