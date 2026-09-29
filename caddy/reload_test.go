package caddy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

var reloadTestEntered, reloadTestRelease chan struct{}

type reloadTestPause struct {
	Pause bool `json:"pause"`
}

func (*reloadTestPause) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.frankenphp_test_reload_pause",
		New: func() caddy.Module { return new(reloadTestPause) },
	}
}

func (p *reloadTestPause) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if p.Pause {
		close(reloadTestEntered)
		<-reloadTestRelease
	}
	return next.ServeHTTP(w, r)
}

func init() {
	caddy.RegisterModule(new(reloadTestPause))
}

func TestReloadPreservesRequestAndResponse(t *testing.T) {
	for _, duringRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("during_request=%t", duringRequest), func(t *testing.T) {
			t.Cleanup(func() {
				require.NoError(t, caddy.Stop())
				frankenphp.Shutdown()
				activeApp.Store(nil)
			})
			reloadTestEntered, reloadTestRelease = make(chan struct{}), make(chan struct{})
			defer func() {
				select {
				case <-reloadTestRelease:
				default:
					close(reloadTestRelease)
				}
			}()
			load := func(policy string, pause bool) *caddyhttp.Server {
				t.Helper()
				config := fmt.Sprintf(`{
					"admin":{"disabled":true,"config":{"persist":false}},
					"logging":{"logs":{"default":{"level":"ERROR"}}},
					"apps":{
						"frankenphp":{"num_threads":2},
						"http":{"servers":{"test":{
							"listen":["127.0.0.1:0"],
							"automatic_https":{"disable":true},
							"routes":[
							{"match":[{"path":["/static"]}],"handle":[{"handler":"static_response","body":%q}]},
							{"match":[{"host":["app.example"]}],"handle":[
								{"handler":"headers","request":{"set":{"Host":["internal.example"]},"add":{"X-Tag":["tag"]}},"response":{"add":{"X-Trace":[%q]}}},
								{"handler":"headers","response":{"set":{"X-Policy":[%q]},"deferred":true}},
								{"handler":"rewrite","uri":"/reload.php"},
								{"handler":"frankenphp_test_reload_pause","pause":%t},
								{"handler":"php","root":"../testdata"}
							]}]
						}}}
					}
				}`, policy, policy, policy, pause)
				require.NoError(t, caddy.Load([]byte(config), true))
				app := activeApp.Load()
				require.Equal(t, "app.example", app.modules[0].server.Name())
				return app.httpApp.Servers["test"]
			}
			request := func(server *caddyhttp.Server, response *httptest.ResponseRecorder) {
				r := httptest.NewRequest(http.MethodPost, "http://app.example/api?query=value", strings.NewReader("payload"))
				server.ServeHTTP(response, r)
			}

			old := load("old", duringRequest)
			forwarded := httptest.NewRecorder()
			done := make(chan struct{})
			if duringRequest {
				go func() {
					defer close(done)
					request(old, forwarded)
				}()
				select {
				case <-reloadTestEntered:
				case <-time.After(5 * time.Second):
					t.Fatal("request did not reach the old PHP handler")
				}
			}

			current := load("new", false)
			static := httptest.NewRecorder()
			old.ServeHTTP(static, httptest.NewRequest(http.MethodGet, "http://app.example/static", nil))
			require.Equal(t, "old", static.Body.String())
			if duringRequest {
				close(reloadTestRelease)
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("request handoff did not finish")
				}
			} else {
				request(old, forwarded)
			}
			direct := httptest.NewRecorder()
			request(current, direct)
			require.Equal(t, http.StatusOK, forwarded.Code)
			require.Equal(t, direct.Body.String(), forwarded.Body.String())
			// keeps the old tree's header state
			require.Equal(t, []string{"old"}, forwarded.Header().Values("X-Trace"))
			require.Equal(t, "old", forwarded.Header().Get("X-Policy"))
			var result map[string]string
			require.NoError(t, json.Unmarshal(forwarded.Body.Bytes(), &result))
			require.Equal(t, map[string]string{
				"host": "internal.example",
				"tag":  "tag",
				"uri":  "/api?query=value",
				"body": "payload",
			}, result)
		})
	}
}
