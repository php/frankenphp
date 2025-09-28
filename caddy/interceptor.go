package caddy

import (
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
)

type responseWriterInterceptor struct {
	http.ResponseWriter
	replacer    *caddy.Replacer
	wroteHeader bool
}

func (i *responseWriterInterceptor) WriteHeader(statusCode int) {
	if !i.wroteHeader {
		if i.replacer != nil {
			i.replacer.Set("http.frankenphp.status_code", statusCode)
			i.replacer.Set("http.frankenphp.status_text", http.StatusText(statusCode))

			for key, values := range i.Header() {
				i.replacer.Set("http.frankenphp.header."+key, strings.Join(values, ","))
			}
		}

		i.wroteHeader = true
	}

	i.ResponseWriter.WriteHeader(statusCode)
}

func (i *responseWriterInterceptor) Write(b []byte) (int, error) {
	if !i.wroteHeader {
		i.WriteHeader(http.StatusOK)
	}

	return i.ResponseWriter.Write(b)
}
