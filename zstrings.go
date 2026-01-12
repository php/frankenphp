package frankenphp

// #include "frankenphp.h"
import "C"
import (
	"github.com/dunglas/frankenphp/internal/phpheaders"
)

// cached zend_strings for optimization
var (
	commonHeaders map[string]*C.zend_string

	zStrContentLength  *C.zend_string
	zStrDocumentRoot   *C.zend_string
	zStrDocumentURI    *C.zend_string
	zStrGatewayIface   *C.zend_string
	zStrHttpHost       *C.zend_string
	zStrHttps          *C.zend_string
	zStrPathInfo       *C.zend_string
	zStrPhpSelf        *C.zend_string
	zStrRemoteAddr     *C.zend_string
	zStrRemoteHost     *C.zend_string
	zStrRemotePort     *C.zend_string
	zStrRequestScheme  *C.zend_string
	zStrScriptFilename *C.zend_string
	zStrScriptName     *C.zend_string
	zStrServerName     *C.zend_string
	zStrServerPort     *C.zend_string
	zStrServerProtocol *C.zend_string
	zStrServerSoftware *C.zend_string
	zStrSslProtocol    *C.zend_string
	zStrSslCipher      *C.zend_string
	zStrAuthType       *C.zend_string
	zStrRemoteIdent    *C.zend_string
	zStrContentType    *C.zend_string
	zStrPathTranslated *C.zend_string
	zStrQueryString    *C.zend_string
	zStrRemoteUser     *C.zend_string
	zStrRequestMethod  *C.zend_string
	zStrRequestURI     *C.zend_string
	zStrCgi1           *C.zend_string
	zStrFrankenPHP     *C.zend_string
)

func initZendStrings() {
	if commonHeaders != nil {
		return // already initialized
	}

	// cache common request headers as zend_strings (HTTP_ACCEPT, HTTP_USER_AGENT, etc.)
	commonHeaders = make(map[string]*C.zend_string, len(phpheaders.CommonRequestHeaders))
	for key, phpKey := range phpheaders.CommonRequestHeaders {
		commonHeaders[key] = C.frankenphp_init_persistent_string(C.CString(phpKey), C.size_t(len(phpKey)))
	}

	// cache known $_SERVER KEYs as zend_strings
	zStrContentLength = internedZendString("CONTENT_LENGTH")
	zStrDocumentRoot = internedZendString("DOCUMENT_ROOT")
	zStrDocumentURI = internedZendString("DOCUMENT_URI")
	zStrGatewayIface = internedZendString("GATEWAY_INTERFACE")
	zStrHttpHost = internedZendString("HTTP_HOST")
	zStrHttps = internedZendString("HTTPS")
	zStrPathInfo = internedZendString("PATH_INFO")
	zStrPhpSelf = internedZendString("PHP_SELF")
	zStrRemoteAddr = internedZendString("REMOTE_ADDR")
	zStrRemoteHost = internedZendString("REMOTE_HOST")
	zStrRemotePort = internedZendString("REMOTE_PORT")
	zStrRequestScheme = internedZendString("REQUEST_SCHEME")
	zStrScriptFilename = internedZendString("SCRIPT_FILENAME")
	zStrScriptName = internedZendString("SCRIPT_NAME")
	zStrServerName = internedZendString("SERVER_NAME")
	zStrServerPort = internedZendString("SERVER_PORT")
	zStrServerProtocol = internedZendString("SERVER_PROTOCOL")
	zStrServerSoftware = internedZendString("SERVER_SOFTWARE")
	zStrSslProtocol = internedZendString("SSL_PROTOCOL")
	zStrSslCipher = internedZendString("SSL_CIPHER")
	zStrAuthType = internedZendString("AUTH_TYPE")
	zStrRemoteIdent = internedZendString("REMOTE_IDENT")
	zStrContentType = internedZendString("CONTENT_TYPE")
	zStrPathTranslated = internedZendString("PATH_TRANSLATED")
	zStrQueryString = internedZendString("QUERY_STRING")
	zStrRemoteUser = internedZendString("REMOTE_USER")
	zStrRequestMethod = internedZendString("REQUEST_METHOD")
	zStrRequestURI = internedZendString("REQUEST_URI")
	zStrCgi1 = internedZendString("CGI/1.1")
	zStrFrankenPHP = internedZendString("FrankenPHP")
}

func internedZendString(s string) *C.zend_string {
	return C.frankenphp_init_persistent_string(toUnsafeChar(s), C.size_t(len(s)))
}
