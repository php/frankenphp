package frankenphp

import (
	"os"
	"testing"
)

const startupLogTestExpectedVersionEnv = "FRANKENPHP_EXPECT_VERSION"

func TestStartupLogAttrsIncludeFrankenPHPVersion(t *testing.T) {
	const (
		testPHPVersion  = "8.2.31"
		testNumThreads  = 4
		testMaxThreads  = 8
		testMaxRequests = 0
	)

	expectedFrankenPHPVersion := os.Getenv(startupLogTestExpectedVersionEnv)
	if expectedFrankenPHPVersion == "" {
		expectedFrankenPHPVersion = frankenPHPVersion()
	}

	attrs := startupLogAttrs(testPHPVersion, testNumThreads, testMaxThreads, testMaxRequests)
	if len(attrs) == 0 {
		t.Fatal("expected startup log attrs")
	}
	if attrs[0].Key != startupLogAttrVersion {
		t.Fatalf("expected first startup log attr key %q, got %q", startupLogAttrVersion, attrs[0].Key)
	}
	if got := attrs[0].Value.String(); got != expectedFrankenPHPVersion {
		t.Fatalf("expected startup log version %q, got %q", expectedFrankenPHPVersion, got)
	}
}
