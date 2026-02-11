package caddy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/dunglas/frankenphp"
)

type FrankenPHPAdmin struct {
}

// if the id starts with "admin.api" the module will register AdminRoutes via module.Routes()
func (FrankenPHPAdmin) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "admin.api.frankenphp",
		New: func() caddy.Module { return new(FrankenPHPAdmin) },
	}
}

// EXPERIMENTAL: These routes are not yet stable and may change in the future.
func (admin FrankenPHPAdmin) Routes() []caddy.AdminRoute {
	return []caddy.AdminRoute{
		{
			Pattern: "/frankenphp/workers/restart",
			Handler: caddy.AdminHandlerFunc(admin.restartWorkers),
		},
		{
			Pattern: "/frankenphp/threads",
			Handler: caddy.AdminHandlerFunc(admin.threads),
		},
	}
}

func (admin *FrankenPHPAdmin) restartWorkers(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return admin.error(http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}

	frankenphp.RestartWorkers()
	caddy.Log().Info("workers restarted from admin api")
	admin.success(w, "workers restarted successfully\n")

	return nil
}

func (admin *FrankenPHPAdmin) threads(w http.ResponseWriter, _ *http.Request) error {
	debugState := frankenphp.DebugState()
	prettyJson, err := json.MarshalIndent(debugState, "", "    ")
	if err != nil {
		return admin.error(http.StatusInternalServerError, err)
	}

	return admin.success(w, string(prettyJson))
}

func (admin *FrankenPHPAdmin) success(w http.ResponseWriter, message string) error {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(message))
	return err
}

func (admin *FrankenPHPAdmin) error(statusCode int, err error) error {
	return caddy.APIError{HTTPStatus: statusCode, Err: err}
}
