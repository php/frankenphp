package frankenphp

// #cgo nocallback frankenphp_register_bulk
// #cgo nocallback frankenphp_register_variables_from_request_info
// #cgo nocallback frankenphp_register_variable_safe
// #cgo nocallback frankenphp_register_single
// #cgo noescape frankenphp_register_bulk
// #cgo noescape frankenphp_register_variables_from_request_info
// #cgo noescape frankenphp_register_variable_safe
// #cgo noescape frankenphp_register_single
// #include <php_variables.h>
// #include "frankenphp.h"
import "C"
import (
	"crypto/tls"
	"net"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/dunglas/frankenphp/internal/phpheaders"
)

// Protocol versions, in Apache mod_ssl format: https://httpd.apache.org/docs/current/mod/mod_ssl.html
// Note that these are slightly different from SupportedProtocols in caddytls/config.go
var tlsProtocolStrings = map[uint16]string{
	tls.VersionTLS10: "TLSv1",
	tls.VersionTLS11: "TLSv1.1",
	tls.VersionTLS12: "TLSv1.2",
	tls.VersionTLS13: "TLSv1.3",
}

// Known $_SERVER keys
var knownServerKeys = []string{
	"CONTENT_LENGTH",
	"DOCUMENT_ROOT",
	"DOCUMENT_URI",
	"GATEWAY_INTERFACE",
	"HTTP_HOST",
	"HTTPS",
	"PATH_INFO",
	"PHP_SELF",
	"REMOTE_ADDR",
	"REMOTE_HOST",
	"REMOTE_PORT",
	"REQUEST_SCHEME",
	"SCRIPT_FILENAME",
	"SCRIPT_NAME",
	"SERVER_NAME",
	"SERVER_PORT",
	"SERVER_PROTOCOL",
	"SERVER_SOFTWARE",
	"SSL_PROTOCOL",
	"SSL_CIPHER",
	"AUTH_TYPE",
	"REMOTE_IDENT",
	"CONTENT_TYPE",
	"PATH_TRANSLATED",
	"QUERY_STRING",
	"REMOTE_USER",
	"REQUEST_METHOD",
	"REQUEST_URI",
}

// computeKnownVariables returns a set of CGI environment variables for the request.
//
// TODO: handle this case https://github.com/caddyserver/caddy/issues/3718
// Inspired by https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
func addKnownVariablesToServer(thread *phpThread, fc *frankenPHPContext, trackVarsArray *C.zval) {
	request := fc.request
	keys := mainThread.knownServerKeys
	// Separate remote IP and port; more lenient than net.SplitHostPort
	var ip, port string
	if idx := strings.LastIndex(request.RemoteAddr, ":"); idx > -1 {
		ip = request.RemoteAddr[:idx]
		port = request.RemoteAddr[idx+1:]
	} else {
		ip = request.RemoteAddr
	}

	// Remove [] from IPv6 addresses
	ip = strings.Replace(ip, "[", "", 1)
	ip = strings.Replace(ip, "]", "", 1)

	var https string
	var sslProtocol string
	var sslCipher string
	var rs string
	if request.TLS == nil {
		rs = "http"
		https = ""
		sslProtocol = ""
		sslCipher = ""
	} else {
		rs = "https"
		https = "on"

		// and pass the protocol details in a manner compatible with apache's mod_ssl
		// (which is why these have an SSL_ prefix and not TLS_).
		if v, ok := tlsProtocolStrings[request.TLS.Version]; ok {
			sslProtocol = v
		} else {
			sslProtocol = ""
		}

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
		case "https":
			reqPort = "443"
		case "http":
			reqPort = "80"
		}
	}

	serverPort := reqPort
	contentLength := request.Header.Get("Content-Length")

	var requestURI string
	if fc.originalRequest != nil {
		requestURI = fc.originalRequest.URL.RequestURI()
	} else {
		requestURI = request.URL.RequestURI()
	}

	C.frankenphp_register_bulk(
		trackVarsArray,
		packCgiVariable(keys["REMOTE_ADDR"], ip),
		packCgiVariable(keys["REMOTE_HOST"], ip),
		packCgiVariable(keys["REMOTE_PORT"], port),
		packCgiVariable(keys["DOCUMENT_ROOT"], fc.documentRoot),
		packCgiVariable(keys["PATH_INFO"], fc.pathInfo),
		packCgiVariable(keys["PHP_SELF"], request.URL.Path),
		packCgiVariable(keys["DOCUMENT_URI"], fc.docURI),
		packCgiVariable(keys["SCRIPT_FILENAME"], fc.scriptFilename),
		packCgiVariable(keys["SCRIPT_NAME"], fc.scriptName),
		packCgiVariable(keys["HTTPS"], https),
		packCgiVariable(keys["SSL_PROTOCOL"], sslProtocol),
		packCgiVariable(keys["REQUEST_SCHEME"], rs),
		packCgiVariable(keys["SERVER_NAME"], reqHost),
		packCgiVariable(keys["SERVER_PORT"], serverPort),
		// Variables defined in CGI 1.1 spec
		// Some variables are unused but cleared explicitly to prevent
		// the parent environment from interfering.
		// These values can not be overridden
		packCgiVariable(keys["CONTENT_LENGTH"], contentLength),
		packCgiVariable(keys["GATEWAY_INTERFACE"], "CGI/1.1"),
		packCgiVariable(keys["SERVER_PROTOCOL"], request.Proto),
		packCgiVariable(keys["SERVER_SOFTWARE"], "FrankenPHP"),
		packCgiVariable(keys["HTTP_HOST"], request.Host),
		// These values are always empty but must be defined:
		packCgiVariable(keys["AUTH_TYPE"], ""),
		packCgiVariable(keys["REMOTE_IDENT"], ""),
		// Request uri of the original request
		packCgiVariable(keys["REQUEST_URI"], requestURI),
		packCgiVariable(keys["SSL_CIPHER"], sslCipher),
	)

	// These values are already present in the SG(request_info), so we'll register them from there
	C.frankenphp_register_variables_from_request_info(
		trackVarsArray,
		keys["CONTENT_TYPE"],
		keys["PATH_TRANSLATED"],
		keys["QUERY_STRING"],
		keys["REMOTE_USER"],
		keys["REQUEST_METHOD"],
	)
}

func packCgiVariable(key *C.zend_string, value string) C.ht_key_value_pair {
	return C.ht_key_value_pair{key, toUnsafeChar(value), C.size_t(len(value))}
}

func addHeadersToServer(fc *frankenPHPContext, trackVarsArray *C.zval) {
	for field, val := range fc.request.Header {
		if k := mainThread.commonHeaders[field]; k != nil {
			v := strings.Join(val, ", ")
			C.frankenphp_register_single(k, toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
			continue
		}

		// if the header name could not be cached, it needs to be registered safely
		// this is more inefficient but allows additional sanitizing by PHP
		k := phpheaders.GetUnCommonHeader(field)
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

//export go_register_variables
func go_register_variables(threadIndex C.uintptr_t, trackVarsArray *C.zval) {
	thread := phpThreads[threadIndex]
	fc := thread.getRequestContext()

	addKnownVariablesToServer(thread, fc, trackVarsArray)
	addHeadersToServer(fc, trackVarsArray)

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
	fc.worker = getWorkerByPath(fc.scriptFilename)
}

// splitPos returns the index where path should
// be split based on SplitPath.
// example: if splitPath is [".php"]
// "/path/to/script.php/some/path": ("/path/to/script.php", "/some/path")
//
// Adapted from https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go
// Copyright 2015 Matthew Holt and The Caddy Authors
func splitPos(path string, splitPath []string) int {
	if len(splitPath) == 0 {
		return 0
	}

	lowerPath := strings.ToLower(path)
	for _, split := range splitPath {
		if idx := strings.Index(lowerPath, strings.ToLower(split)); idx > -1 {
			return idx + len(split)
		}
	}
	return -1
}

// go_update_request_info updates the sapi_request_info struct
// See: https://github.com/php/php-src/blob/345e04b619c3bc11ea17ee02cdecad6ae8ce5891/main/SAPI.h#L72
//
//export go_update_request_info
func go_update_request_info(threadIndex C.uintptr_t, info *C.sapi_request_info) C.bool {
	thread := phpThreads[threadIndex]
	fc := thread.getRequestContext()
	request := fc.request

	authUser, authPassword, ok := request.BasicAuth()
	if ok {
		if authPassword != "" {
			info.auth_password = thread.pinCString(authPassword)
		}
		if authUser != "" {
			info.auth_user = thread.pinCString(authUser)
		}
	}

	info.request_method = thread.pinCString(request.Method)
	info.query_string = thread.pinCString(request.URL.RawQuery)
	info.content_length = C.zend_long(request.ContentLength)

	if contentType := request.Header.Get("Content-Type"); contentType != "" {
		info.content_type = thread.pinCString(contentType)
	}

	if fc.pathInfo != "" {
		info.path_translated = thread.pinCString(sanitizedPathJoin(fc.documentRoot, fc.pathInfo)) // See: http://www.oreilly.com/openbook/cgi/ch02_04.html
	}

	info.request_uri = thread.pinCString(request.URL.RequestURI())

	info.proto_num = C.int(request.ProtoMajor*1000 + request.ProtoMinor)

	return C.bool(fc.worker != nil)
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

func toUnsafeChar(s string) *C.char {
	sData := unsafe.StringData(s)
	return (*C.char)(unsafe.Pointer(sData))
}
