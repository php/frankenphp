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

	C.frankenphp_register_bulk(trackVarsArray, C.frankenphp_server_vars{
		remote_addr_key: zStrRemoteAddr,
		remote_addr_val: toUnsafeChar(ip),
		remote_addr_len: C.size_t(len(ip)),

		remote_host_key: zStrRemoteHost,
		remote_host_val: toUnsafeChar(ip),
		remote_host_len: C.size_t(len(ip)),

		remote_port_key: zStrRemotePort,
		remote_port_val: toUnsafeChar(port),
		remote_port_len: C.size_t(len(port)),

		document_root_key: zStrDocumentRoot,
		document_root_val: toUnsafeChar(fc.documentRoot),
		document_root_len: C.size_t(len(fc.documentRoot)),

		path_info_key: zStrPathInfo,
		path_info_val: toUnsafeChar(fc.pathInfo),
		path_info_len: C.size_t(len(fc.pathInfo)),

		php_self_key: zStrPhpSelf,
		php_self_val: toUnsafeChar(request.URL.Path),
		php_self_len: C.size_t(len(request.URL.Path)),

		document_uri_key: zStrDocumentURI,
		document_uri_val: toUnsafeChar(fc.docURI),
		document_uri_len: C.size_t(len(fc.docURI)),

		script_filename_key: zStrScriptFilename,
		script_filename_val: toUnsafeChar(fc.scriptFilename),
		script_filename_len: C.size_t(len(fc.scriptFilename)),

		script_name_key: zStrScriptName,
		script_name_val: toUnsafeChar(fc.scriptName),
		script_name_len: C.size_t(len(fc.scriptName)),

		https_key: zStrHttps,
		https_val: toUnsafeChar(https),
		https_len: C.size_t(len(https)),

		ssl_protocol_key: zStrSslProtocol,
		ssl_protocol_val: toUnsafeChar(sslProtocol),
		ssl_protocol_len: C.size_t(len(sslProtocol)),

		request_scheme_key: zStrRequestScheme,
		request_scheme_val: toUnsafeChar(rs),
		request_scheme_len: C.size_t(len(rs)),

		server_name_key: zStrServerName,
		server_name_val: toUnsafeChar(reqHost),
		server_name_len: C.size_t(len(reqHost)),

		server_port_key: zStrServerPort,
		server_port_val: toUnsafeChar(serverPort),
		server_port_len: C.size_t(len(serverPort)),

		content_length_key: zStrContentLength,
		content_length_val: toUnsafeChar(contentLength),
		content_length_len: C.size_t(len(contentLength)),

		gateway_interface_key: zStrGatewayIface,
		gateway_interface_str: zStrCgi1,

		server_protocol_key: zStrServerProtocol,
		server_protocol_val: toUnsafeChar(request.Proto),
		server_protocol_len: C.size_t(len(request.Proto)),

		server_software_key: zStrServerSoftware,
		server_software_str: zStrFrankenPHP,

		http_host_key: zStrHttpHost,
		http_host_val: toUnsafeChar(request.Host),
		http_host_len: C.size_t(len(request.Host)),

		auth_type_key: zStrAuthType,
		auth_type_val: toUnsafeChar(""),
		auth_type_len: C.size_t(0),

		remote_ident_key: zStrRemoteIdent,
		remote_ident_val: toUnsafeChar(""),
		remote_ident_len: C.size_t(0),

		request_uri_key: zStrRequestURI,
		request_uri_val: toUnsafeChar(requestURI),
		request_uri_len: C.size_t(len(requestURI)),

		ssl_cipher_key: zStrSslCipher,
		ssl_cipher_val: toUnsafeChar(sslCipher),
		ssl_cipher_len: C.size_t(len(sslCipher)),
	})

	// These values are already present in the SG(request_info), so we'll register them from there
	C.frankenphp_register_variables_from_request_info(
		trackVarsArray,
		zStrContentType,
		zStrPathTranslated,
		zStrQueryString,
		zStrRemoteUser,
		zStrRequestMethod,
	)
}

func addHeadersToServer(ctx context.Context, request *http.Request, trackVarsArray *C.zval) {
	for field, val := range request.Header {
		if k := commonHeaders[field]; k != nil {
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
