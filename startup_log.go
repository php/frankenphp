package frankenphp

import "log/slog"

const (
	startupLogMessage         = "FrankenPHP started 🐘"
	startupLogAttrVersion     = "version"
	startupLogAttrPHPVersion  = "php_version"
	startupLogAttrNumThreads  = "num_threads"
	startupLogAttrMaxThreads  = "max_threads"
	startupLogAttrMaxRequests = "max_requests"
	startupLogAttrCapacity    = 5
)

func startupLogAttrs(phpVersion string, numThreads int, maxThreads int, maxRequests int) []slog.Attr {
	attrs := make([]slog.Attr, 0, startupLogAttrCapacity)
	if version := frankenPHPVersion(); version != "" {
		attrs = append(attrs, slog.String(startupLogAttrVersion, version))
	}

	return append(attrs,
		slog.String(startupLogAttrPHPVersion, phpVersion),
		slog.Int(startupLogAttrNumThreads, numThreads),
		slog.Int(startupLogAttrMaxThreads, maxThreads),
		slog.Int(startupLogAttrMaxRequests, maxRequests),
	)
}
