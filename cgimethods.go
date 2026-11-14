package frankenphp

// const char *frankenphp_method_get = "GET";
// const char *frankenphp_method_head = "HEAD";
// const char *frankenphp_method_post = "POST";
// const char *frankenphp_method_put = "PUT";
// const char *frankenphp_method_delete = "DELETE";
// const char *frankenphp_method_connect = "CONNECT";
// const char *frankenphp_method_options = "OPTIONS";
// const char *frankenphp_method_trace = "TRACE";
// const char *frankenphp_method_patch = "PATCH";
import "C"

// cStringHTTPMethods maps common HTTP methods to static C strings, avoiding a
// pinCString call per request. C literals, unlike C.CString, aren't reported as
// leaks by LeakSanitizer, which can't see pointers held in Go memory.
var cStringHTTPMethods = map[string]*C.char{
	"GET":     C.frankenphp_method_get,
	"HEAD":    C.frankenphp_method_head,
	"POST":    C.frankenphp_method_post,
	"PUT":     C.frankenphp_method_put,
	"DELETE":  C.frankenphp_method_delete,
	"CONNECT": C.frankenphp_method_connect,
	"OPTIONS": C.frankenphp_method_options,
	"TRACE":   C.frankenphp_method_trace,
	"PATCH":   C.frankenphp_method_patch,
}
