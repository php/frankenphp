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
	"context"
	"crypto/tls"
	"net"
	"net/http"
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

var (
	contentLengthKey  *C.zend_string
	documentRoot      *C.zend_string
	documentURI       *C.zend_string
	gatewayIface      *C.zend_string
	httpHost          *C.zend_string
	httpsKey          *C.zend_string
	pathInfo          *C.zend_string
	phpSelf           *C.zend_string
	remoteAddr        *C.zend_string
	remoteHost        *C.zend_string
	remotePort        *C.zend_string
	requestScheme     *C.zend_string
	scriptFilename    *C.zend_string
	scriptName        *C.zend_string
	serverName        *C.zend_string
	serverPortKey     *C.zend_string
	serverProtocolKey *C.zend_string
	serverSoftware    *C.zend_string
	sslProtocolKey    *C.zend_string
	sslCipherKey      *C.zend_string
	authType          *C.zend_string
	remoteIdent       *C.zend_string
	contentTypeKey    *C.zend_string
	pathTranslated    *C.zend_string
	queryString       *C.zend_string
	remoteUser        *C.zend_string
	requestMethodKey  *C.zend_string
	requestURIKey     *C.zend_string
)

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
	ip = strings.Replace(ip, "[", "", 1)
	ip = strings.Replace(ip, "]", "", 1)

	var https, sslProtocol, sslCipher, rs string

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
		packCgiVariable(remoteAddr, ip),
		packCgiVariable(remoteHost, ip),
		packCgiVariable(remotePort, port),
		packCgiVariable(documentRoot, fc.documentRoot),
		packCgiVariable(pathInfo, fc.pathInfo),
		packCgiVariable(phpSelf, request.URL.Path),
		packCgiVariable(documentURI, fc.docURI),
		packCgiVariable(scriptFilename, fc.scriptFilename),
		packCgiVariable(scriptName, fc.scriptName),
		packCgiVariable(httpsKey, https),
		packCgiVariable(sslProtocolKey, sslProtocol),
		packCgiVariable(requestScheme, rs),
		packCgiVariable(serverName, reqHost),
		packCgiVariable(serverPortKey, serverPort),
		// Variables defined in CGI 1.1 spec
		// Some variables are unused but cleared explicitly to prevent
		// the parent environment from interfering.
		// These values can not be overridden
		packCgiVariable(contentLengthKey, contentLength),
		packCgiVariable(gatewayIface, "CGI/1.1"),
		packCgiVariable(serverProtocolKey, request.Proto),
		packCgiVariable(serverSoftware, "FrankenPHP"),
		packCgiVariable(httpHost, request.Host),
		// These values are always empty but must be defined:
		packCgiVariable(authType, ""),
		packCgiVariable(remoteIdent, ""),
		// Request uri of the original request
		packCgiVariable(requestURIKey, requestURI),
		packCgiVariable(sslCipherKey, sslCipher),
	)

	// These values are already present in the SG(request_info), so we'll register them from there
	C.frankenphp_register_variables_from_request_info(
		trackVarsArray,
		contentTypeKey,
		pathTranslated,
		queryString,
		remoteUser,
		requestMethodKey,
	)
}

func packCgiVariable(key *C.zend_string, value string) C.ht_key_value_pair {
	return C.ht_key_value_pair{key, toUnsafeChar(value), C.size_t(len(value))}
}

func addHeadersToServer(ctx context.Context, request *http.Request, trackVarsArray *C.zval) {
	for field, val := range request.Header {
		if k := mainThread.commonHeaders[field]; k != nil {
			v := strings.Join(val, ", ")
			C.frankenphp_register_single(k, toUnsafeChar(v), C.size_t(len(v)), trackVarsArray)
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

//export go_register_variables
func go_register_variables(threadIndex C.uintptr_t, trackVarsArray *C.zval) {
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
func go_update_request_info(threadIndex C.uintptr_t, info *C.sapi_request_info) {
	thread := phpThreads[threadIndex]
	fc := thread.frankenPHPContext()
	request := fc.request

	if request == nil {
		return
	}

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
func initKnownServerKeys() {
	contentLengthKey = internedZendString("CONTENT_LENGTH")
	documentRoot = internedZendString("DOCUMENT_ROOT")
	documentURI = internedZendString("DOCUMENT_URI")
	gatewayIface = internedZendString("GATEWAY_INTERFACE")
	httpHost = internedZendString("HTTP_HOST")
	httpsKey = internedZendString("HTTPS")
	pathInfo = internedZendString("PATH_INFO")
	phpSelf = internedZendString("PHP_SELF")
	remoteAddr = internedZendString("REMOTE_ADDR")
	remoteHost = internedZendString("REMOTE_HOST")
	remotePort = internedZendString("REMOTE_PORT")
	requestScheme = internedZendString("REQUEST_SCHEME")
	scriptFilename = internedZendString("SCRIPT_FILENAME")
	scriptName = internedZendString("SCRIPT_NAME")
	serverName = internedZendString("SERVER_NAME")
	serverPortKey = internedZendString("SERVER_PORT")
	serverProtocolKey = internedZendString("SERVER_PROTOCOL")
	serverSoftware = internedZendString("SERVER_SOFTWARE")
	sslProtocolKey = internedZendString("SSL_PROTOCOL")
	sslCipherKey = internedZendString("SSL_CIPHER")
	authType = internedZendString("AUTH_TYPE")
	remoteIdent = internedZendString("REMOTE_IDENT")
	contentTypeKey = internedZendString("CONTENT_TYPE")
	pathTranslated = internedZendString("PATH_TRANSLATED")
	queryString = internedZendString("QUERY_STRING")
	remoteUser = internedZendString("REMOTE_USER")
	requestMethodKey = internedZendString("REQUEST_METHOD")
	requestURIKey = internedZendString("REQUEST_URI")
}

func internedZendString(s string) *C.zend_string {
	return C.frankenphp_init_persistent_string(toUnsafeChar(s), C.size_t(len(s)))
}
