package frankenphp

// #cgo nocallback frankenphp_register_server_vars
// #cgo nocallback frankenphp_register_variable_safe
// #cgo nocallback frankenphp_register_known_variable
// #cgo nocallback frankenphp_init_persistent_string
// #cgo noescape frankenphp_register_server_vars
// #cgo noescape frankenphp_register_variable_safe
// #cgo noescape frankenphp_register_known_variable
// #cgo noescape frankenphp_init_persistent_string
// #include "frankenphp.h"
// #include <php_variables.h>
import "C"
import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/phpheaders"
	"golang.org/x/text/language"
	"golang.org/x/text/search"
)

// cStringHTTPMethods caches C string versions of common HTTP methods
// to avoid allocations in pinCString on every request.
var cStringHTTPMethods = map[string]*C.char{
	"GET":     C.CString("GET"),
	"HEAD":    C.CString("HEAD"),
	"POST":    C.CString("POST"),
	"PUT":     C.CString("PUT"),
	"DELETE":  C.CString("DELETE"),
	"CONNECT": C.CString("CONNECT"),
	"OPTIONS": C.CString("OPTIONS"),
	"TRACE":   C.CString("TRACE"),
	"PATCH":   C.CString("PATCH"),
}

// computeKnownVariables returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func addKnownVariablesToServer(fc *frankenPHPContext, trackVarsArray *C.zval) {
	request := fc.request
	// Separate remote IP and port; more lenient than net.SplitHostPort
	var ip, port string
	if idx := strings.LastIndex(request.RemoteAddr, ":"); idx > -1 {
		ip = request.RemoteAddr[:idx]
		port = request.RemoteAddr[idx+1:]
	} else {
		ip = request.RemoteAddr
	}

	// Remove [] from IPv6 addresses
	if len(ip) > 0 && ip[0] == '[' {
		ip = ip[1 : len(ip)-1]
	}

	var rs, https, sslProtocol *C.zend_string
	var sslCipher string

	if request.TLS == nil {
		rs = C.frankenphp_strings.httpLowercase
		https = C.frankenphp_strings.empty
		sslProtocol = C.frankenphp_strings.empty
		sslCipher = ""
	} else {
		rs = C.frankenphp_strings.httpsLowercase
		https = C.frankenphp_strings.on

		// and pass the protocol details in a manner compatible with Apache's mod_ssl
		// (which is why these have an SSL_ prefix and not TLS_).
		sslProtocol = tlsProtocol(request.TLS.Version)

		if request.TLS.CipherSuite != 0 {
			sslCipher = tls.CipherSuiteName(request.TLS.CipherSuite)
		}
	}

	reqHost, reqPort, _ := net.SplitHostPort(request.Host)

	if reqHost == "" {
		// whatever, just assume there was no port
		reqHost = request.Host
	}

	if reqPort == "" {
		// compliance with the CGI specification requires that
		// the SERVER_PORT variable MUST be set to the TCP/IP port number on which this request is received from the client
		// even if the port is the default port for the scheme and could otherwise be omitted from a URI.
		// https://tools.ietf.org/html/rfc3875#section-4.1.15
		switch rs {
		case C.frankenphp_strings.httpsLowercase:
			reqPort = "443"
		case C.frankenphp_strings.httpLowercase:
			reqPort = "80"
		}
	}

	serverPort := reqPort
	contentLength := request.Header.Get("Content-Length")

	var requestURI string
	if fc.originalRequest != nil {
		requestURI = fc.originalRequest.URL.RequestURI()
	} else {
		requestURI = fc.requestURI
	}

	requestPath := ensureLeadingSlash(request.URL.Path)

	C.frankenphp_register_server_vars(trackVarsArray, C.frankenphp_server_vars{
		// approximate total length to avoid array re-hashing:
		// 28 CGI vars + headers + environment
		total_num_vars: C.size_t(28 + len(request.Header) + len(fc.env) + lengthOfEnv),

		// CGI vars with variable values
		remote_addr:         toUnsafeChar(ip),
		remote_addr_len:     C.size_t(len(ip)),
		remote_host:         toUnsafeChar(ip),
		remote_host_len:     C.size_t(len(ip)),
		remote_port:         toUnsafeChar(port),
		remote_port_len:     C.size_t(len(port)),
		document_root:       toUnsafeChar(fc.documentRoot),
		document_root_len:   C.size_t(len(fc.documentRoot)),
		path_info:           toUnsafeChar(fc.pathInfo),
		path_info_len:       C.size_t(len(fc.pathInfo)),
		php_self:            toUnsafeChar(requestPath),
		php_self_len:        C.size_t(len(requestPath)),
		document_uri:        toUnsafeChar(fc.docURI),
		document_uri_len:    C.size_t(len(fc.docURI)),
		script_filename:     toUnsafeChar(fc.scriptFilename),
		script_filename_len: C.size_t(len(fc.scriptFilename)),
		script_name:         toUnsafeChar(fc.scriptName),
		script_name_len:     C.size_t(len(fc.scriptName)),
		server_name:         toUnsafeChar(reqHost),
		server_name_len:     C.size_t(len(reqHost)),
		server_port:         toUnsafeChar(serverPort),
		server_port_len:     C.size_t(len(serverPort)),
		content_length:      toUnsafeChar(contentLength),
		content_length_len:  C.size_t(len(contentLength)),
		server_protocol:     toUnsafeChar(request.Proto),
		server_protocol_len: C.size_t(len(request.Proto)),
		http_host:           toUnsafeChar(request.Host),
		http_host_len:       C.size_t(len(request.Host)),
		request_uri:         toUnsafeChar(requestURI),
		request_uri_len:     C.size_t(len(requestURI)),
		ssl_cipher:          toUnsafeChar(sslCipher),
		ssl_cipher_len:      C.size_t(len(sslCipher)),

		// CGI vars with known values
		request_scheme: rs,          // "http" or "https"
		ssl_protocol:   sslProtocol, // values from tlsProtocol
		https:          https,       // "on" or empty
	})
}

func addHeadersToServer(ctx context.Context, request *http.Request, trackVarsArray *C.zval) {
	for field, val := range request.Header {
		if k := commonHeaders[field]; k != nil {
			v := strings.Join(val, ", ")
			C.frankenphp_register_known_variable(k, toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
			continue
		}

		// if the header name could not be cached, it needs to be registered safely
		// this is more inefficient but allows additional sanitizing by PHP
		k := phpheaders.GetUnCommonHeader(ctx, field)
		v := strings.Join(val, ", ")
		C.frankenphp_register_variable_safe(toUnsafeChar(k), toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
	}
}

func addPreparedEnvToServer(fc *frankenPHPContext, trackVarsArray *C.zval) {
	for k, v := range fc.env {
		C.frankenphp_register_variable_safe(toUnsafeChar(k), toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
	}
	fc.env = nil
}

//export go_register_server_variables
func go_register_server_variables(threadIndex C.uintptr_t, trackVarsArray *C.zval) {
	thread := phpThreads[threadIndex]
	fc := thread.frankenPHPContext()

	if fc.request != nil {
		addKnownVariablesToServer(fc, trackVarsArray)
		addHeadersToServer(thread.context(), fc.request, trackVarsArray)
	}

	// The Prepared Environment is registered last and can overwrite any previous values
	addPreparedEnvToServer(fc, trackVarsArray)
}

// splitCgiPath splits the request path into SCRIPT_NAME, SCRIPT_FILENAME, PATH_INFO, DOCUMENT_URI
func splitCgiPath(fc *frankenPHPContext) {
	path := fc.request.URL.Path
	splitPath := fc.splitPath

	if splitPath == nil {
		splitPath = []string{".php"}
	}

	if splitPos := splitPos(path, splitPath); splitPos > -1 {
		fc.docURI = path[:splitPos]
		fc.pathInfo = path[splitPos:]

		// Strip PATH_INFO from SCRIPT_NAME
		fc.scriptName = strings.TrimSuffix(path, fc.pathInfo)

		// Ensure the SCRIPT_NAME has a leading slash for compliance with RFC3875
		// Info: https://tools.ietf.org/html/rfc3875#section-4.1.13
		if fc.scriptName != "" && !strings.HasPrefix(fc.scriptName, "/") {
			fc.scriptName = "/" + fc.scriptName
		}
	}

	// TODO: is it possible to delay this and avoid saving everything in the context?
	// SCRIPT_FILENAME is the absolute path of SCRIPT_NAME
	fc.scriptFilename = sanitizedPathJoin(fc.documentRoot, fc.scriptName)
	fc.worker = workersByPath[fc.scriptFilename]
}

var splitSearchNonASCII = search.New(language.Und, search.IgnoreCase)

// splitPos returns the index where path should be split based on splitPath.
// example: if splitPath is [".php"]
// "/path/to/script.php/some/path": ("/path/to/script.php", "/some/path")
func splitPos(path string, splitPath []string) int {
	if len(splitPath) == 0 {
		return 0
	}

	pathLen := len(path)

	// We are sure that split strings are all ASCII-only and lower-case because of validation and normalization in WithRequestSplitPath
	for _, split := range splitPath {
		splitLen := len(split)

		for i := 0; i < pathLen; i++ {
			if path[i] >= utf8.RuneSelf {
				if _, end := splitSearchNonASCII.IndexString(path, split); end > -1 {
					return end
				}

				break
			}

			if i+splitLen > pathLen {
				continue
			}

			match := true
			for j := 0; j < splitLen; j++ {
				c := path[i+j]

				if c >= utf8.RuneSelf {
					if _, end := splitSearchNonASCII.IndexString(path, split); end > -1 {
						return end
					}

					break
				}

				if 'A' <= c && c <= 'Z' {
					c += 'a' - 'A'
				}

				if c != split[j] {
					match = false

					break
				}
			}

			if match {
				return i + splitLen
			}
		}
	}

	return -1
}

// go_update_request_info updates the sapi_request_info struct
// See: https://github.com/php/php-src/blob/345e04b619c3bc11ea17ee02cdecad6ae8ce5891/main/SAPI.h#L72
//
//export go_update_request_info
func go_update_request_info(threadIndex C.uintptr_t, info *C.sapi_request_info) *C.char {
	thread := phpThreads[threadIndex]
	fc := thread.frankenPHPContext()
	request := fc.request

	if request == nil {
		return nil
	}

	if m, ok := cStringHTTPMethods[request.Method]; ok {
		info.request_method = m
	} else {
		info.request_method = thread.pinCString(request.Method)
	}
	info.query_string = thread.pinCString(request.URL.RawQuery)
	info.content_length = C.zend_long(request.ContentLength)

	if contentType := request.Header.Get("Content-Type"); contentType != "" {
		info.content_type = thread.pinCString(contentType)
	}

	if fc.pathInfo != "" {
		info.path_translated = thread.pinCString(sanitizedPathJoin(fc.documentRoot, fc.pathInfo)) // See: http://www.oreilly.com/openbook/cgi/ch02_04.html
	}

	info.request_uri = thread.pinCString(fc.requestURI)

	info.proto_num = C.int(request.ProtoMajor*1000 + request.ProtoMinor)

	authorizationHeader := request.Header.Get("Authorization")
	if authorizationHeader == "" {
		return nil
	}

	return thread.pinCString(authorizationHeader)
}

// SanitizedPathJoin performs filepath.Join(root, reqPath) that
// is safe against directory traversal attacks. It uses logic
// similar to that in the Go standard library, specifically
// in the implementation of http.Dir. The root is assumed to
// be a trusted path, but reqPath is not; and the output will
// never be outside of root. The resulting path can be used
// with the local file system.
//
// Adapted from https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Copyright 2015 Matthew Holt and The Caddy Authors
func sanitizedPathJoin(root, reqPath string) string {
	if root == "" {
		root = "."
	}

	path := filepath.Join(root, filepath.Clean("/"+reqPath))

	// filepath.Join also cleans the path, and cleaning strips
	// the trailing slash, so we need to re-add it afterward.
	// if the length is 1, then it's a path to the root,
	// and that should return ".", so we don't append the separator.
	if strings.HasSuffix(reqPath, "/") && len(reqPath) > 1 {
		path += separator
	}

	return path
}

const separator = string(filepath.Separator)

func ensureLeadingSlash(path string) string {
	if path == "" || path[0] == '/' {
		return path
	}

	return "/" + path
}

// toUnsafeChar returns a *C.char pointing at the backing bytes the Go string.
// If C does not store the string, it may be passed directly in a Cgo call (most efficient).
// If C stores the string, it must be pinned explicitly instead (inefficient).
// C may never modify the string.
func toUnsafeChar(s string) *C.char {
	return (*C.char)(unsafe.Pointer(unsafe.StringData(s)))
}

// initialize a global zend_string that must never be freed and is ignored by GC
func newPersistentZendString(str string) *C.zend_string {
	return C.frankenphp_init_persistent_string(toUnsafeChar(str), C.size_t(len(str)))
}

// Protocol versions, in Apache mod_ssl format: https://httpd.apache.org/docs/current/mod/mod_ssl.html
// Note that these are slightly different from SupportedProtocols in caddytls/config.go
func tlsProtocol(proto uint16) *C.zend_string {
	switch proto {
	case tls.VersionTLS10:
		return C.frankenphp_strings.tls1
	case tls.VersionTLS11:
		return C.frankenphp_strings.tls11
	case tls.VersionTLS12:
		return C.frankenphp_strings.tls12
	case tls.VersionTLS13:
		return C.frankenphp_strings.tls13
	default:
		return C.frankenphp_strings.empty
	}
}
