package phpheaders

import (
	"context"
	"strings"

	"github.com/maypok86/otter/v2"
)

// Translate header names to PHP header names
// All headers in 'commonHeaders' can be cached and registered safely
// All other headers must be sanitized
// Note: net/http will capitalize lowercase headers, so we don't need to worry about case sensitivity
var CommonRequestHeaders = map[string]string{
	"Accept":                            "HTTP_ACCEPT",
	"Accept-Charset":                    "HTTP_ACCEPT_CHARSET",
	"Accept-Encoding":                   "HTTP_ACCEPT_ENCODING",
	"Accept-Language":                   "HTTP_ACCEPT_LANGUAGE",
	"Access-Control-Request-Headers":    "HTTP_ACCESS_CONTROL_REQUEST_HEADERS",
	"Access-Control-Request-Method":     "HTTP_ACCESS_CONTROL_REQUEST_METHOD",
	"Authorization":                     "HTTP_AUTHORIZATION",
	"Cache-Control":                     "HTTP_CACHE_CONTROL",
	"Connection":                        "HTTP_CONNECTION",
	"Content-Disposition":               "HTTP_CONTENT_DISPOSITION",
	"Content-Encoding":                  "HTTP_CONTENT_ENCODING",
	"Content-Length":                    "HTTP_CONTENT_LENGTH",
	"Content-Type":                      "HTTP_CONTENT_TYPE",
	"Cookie":                            "HTTP_COOKIE",
	"Date":                              "HTTP_DATE",
	"Device-Memory":                     "HTTP_DEVICE_MEMORY",
	"Dnt":                               "HTTP_DNT",
	"Downlink":                          "HTTP_DOWNLINK",
	"Dpr":                               "HTTP_DPR",
	"Early-Data":                        "HTTP_EARLY_DATA",
	"Ect":                               "HTTP_ECT",
	"Am-I":                              "HTTP_AM_I",
	"Expect":                            "HTTP_EXPECT",
	"Forwarded":                         "HTTP_FORWARDED",
	"From":                              "HTTP_FROM",
	"Host":                              "HTTP_HOST",
	"If-Match":                          "HTTP_IF_MATCH",
	"If-Modified-Since":                 "HTTP_IF_MODIFIED_SINCE",
	"If-None-Match":                     "HTTP_IF_NONE_MATCH",
	"If-Range":                          "HTTP_IF_RANGE",
	"If-Unmodified-Since":               "HTTP_IF_UNMODIFIED_SINCE",
	"Keep-Alive":                        "HTTP_KEEP_ALIVE",
	"Max-Forwards":                      "HTTP_MAX_FORWARDS",
	"Origin":                            "HTTP_ORIGIN",
	"Pragma":                            "HTTP_PRAGMA",
	"Proxy-Authorization":               "HTTP_PROXY_AUTHORIZATION",
	"Range":                             "HTTP_RANGE",
	"Referer":                           "HTTP_REFERER",
	"Rtt":                               "HTTP_RTT",
	"Save-Data":                         "HTTP_SAVE_DATA",
	"Sec-Ch-Ua":                         "HTTP_SEC_CH_UA",
	"Sec-Ch-Ua-Arch":                    "HTTP_SEC_CH_UA_ARCH",
	"Sec-Ch-Ua-Bitness":                 "HTTP_SEC_CH_UA_BITNESS",
	"Sec-Ch-Ua-Full-Version":            "HTTP_SEC_CH_UA_FULL_VERSION",
	"Sec-Ch-Ua-Full-Version-List":       "HTTP_SEC_CH_UA_FULL_VERSION_LIST",
	"Sec-Ch-Ua-Mobile":                  "HTTP_SEC_CH_UA_MOBILE",
	"Sec-Ch-Ua-Model":                   "HTTP_SEC_CH_UA_MODEL",
	"Sec-Ch-Ua-Platform":                "HTTP_SEC_CH_UA_PLATFORM",
	"Sec-Ch-Ua-Platform-Version":        "HTTP_SEC_CH_UA_PLATFORM_VERSION",
	"Sec-Fetch-Dest":                    "HTTP_SEC_FETCH_DEST",
	"Sec-Fetch-Mode":                    "HTTP_SEC_FETCH_MODE",
	"Sec-Fetch-Site":                    "HTTP_SEC_FETCH_SITE",
	"Sec-Fetch-User":                    "HTTP_SEC_FETCH_USER",
	"Sec-Gpc":                           "HTTP_SEC_GPC",
	"Service-Worker-Navigation-Preload": "HTTP_SERVICE_WORKER_NAVIGATION_PRELOAD",
	"Te":                                "HTTP_TE",
	"Priority":                          "HTTP_PRIORITY",
	"Trailer":                           "HTTP_TRAILER",
	"Transfer-Encoding":                 "HTTP_TRANSFER_ENCODING",
	"Upgrade":                           "HTTP_UPGRADE",
	"Upgrade-Insecure-Requests":         "HTTP_UPGRADE_INSECURE_REQUESTS",
	"User-Agent":                        "HTTP_USER_AGENT",
	"Via":                               "HTTP_VIA",
	"Viewport-Width":                    "HTTP_VIEWPORT_WIDTH",
	"Want-Digest":                       "HTTP_WANT_DIGEST",
	"Warning":                           "HTTP_WARNING",
	"Width":                             "HTTP_WIDTH",
	"X-Forwarded-For":                   "HTTP_X_FORWARDED_FOR",
	"X-Forwarded-Host":                  "HTTP_X_FORWARDED_HOST",
	"X-Forwarded-Path":                  "HTTP_X_FORWARDED_PATH",
	"X-Forwarded-Prefix":                "HTTP_X_FORWARDED_PREFIX",
	"X-Forwarded-Proto":                 "HTTP_X_FORWARDED_PROTO",
	"A-Im":                              "HTTP_A_IM",
	"Accept-Datetime":                   "HTTP_ACCEPT_DATETIME",
	"Content-Md5":                       "HTTP_CONTENT_MD5",
	"Http2-Settings":                    "HTTP_HTTP2_SETTINGS",
	"Prefer":                            "HTTP_PREFER",
	"X-Requested-With":                  "HTTP_X_REQUESTED_WITH",
	"Front-End-Https":                   "HTTP_FRONT_END_HTTPS",
	"X-Http-Method-Override":            "HTTP_X_HTTP_METHOD_OVERRIDE",
	"X-Att-Deviceid":                    "HTTP_X_ATT_DEVICEID",
	"X-Wap-Profile":                     "HTTP_X_WAP_PROFILE",
	"Proxy-Connection":                  "HTTP_PROXY_CONNECTION",
	"X-Uidh":                            "HTTP_X_UIDH",
	"X-Csrf-Token":                      "HTTP_X_CSRF_TOKEN",
	"X-Request-Id":                      "HTTP_X_REQUEST_ID",
	"X-Correlation-Id":                  "HTTP_X_CORRELATION_ID",
	// Additional CDN/Framework headers
	"Cloudflare-Visitor":        "HTTP_CLOUDFLARE_VISITOR",
	"Cloudfront-Viewer-Address": "HTTP_CLOUDFRONT_VIEWER_ADDRESS",
	"Cloudfront-Viewer-Country": "HTTP_CLOUDFRONT_VIEWER_COUNTRY",
	"X-Amzn-Trace-Id":           "HTTP_X_AMZN_TRACE_ID",
	"X-Cloud-Trace-Context":     "HTTP_X_CLOUD_TRACE_CONTEXT",
	"Cf-Ray":                    "HTTP_CF_RAY",
	"Cf-Visitor":                "HTTP_CF_VISITOR",
	"Cf-Request-Id":             "HTTP_CF_REQUEST_ID",
	"Cf-Ipcountry":              "HTTP_CF_IPCOUNTRY",
	"X-Device-Type":             "HTTP_X_DEVICE_TYPE",
	"X-Network-Info":            "HTTP_X_NETWORK_INFO",
	"X-Client-Id":               "HTTP_X_CLIENT_ID",
	"X-Livewire":                "HTTP_X_LIVEWIRE",
	"X-Real-Ip":                 "HTTP_X_REAL_IP",
}

// Cache up to 256 uncommon headers
// This is ~2.5x faster than converting the header each time
var headerKeyCache = otter.Must[string, string](&otter.Options[string, string]{MaximumSize: 256})

var headerNameReplacer = strings.NewReplacer(" ", "_", "-", "_")

func GetUnCommonHeader(ctx context.Context, key string) string {
	phpHeaderKey, err := headerKeyCache.Get(
		ctx,
		key,
		otter.LoaderFunc[string, string](func(_ context.Context, key string) (string, error) {
			return "HTTP_" + headerNameReplacer.Replace(strings.ToUpper(key)) + "\x00", nil
		}),
	)
	if err != nil {
		panic(err)
	}

	return phpHeaderKey
}
