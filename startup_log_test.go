package frankenphp

import (
	"errors"
	"runtime/debug"
	"testing"
)

const (
	startupLogTestDependencyVersion = "v1.12.6"
	startupLogTestMainVersion       = "v1.12.7"
	startupLogTestExecutablePath    = "/tmp/frankenphp-test"
	startupLogTestPHPVersion        = "8.2.31"
	startupLogTestNumThreads        = 4
	startupLogTestMaxThreads        = 8
	startupLogTestMaxRequests       = 0
	startupLogTestExecutableError   = "executable error"
	startupLogTestBuildInfoError    = "build info error"
)

func TestFrankenPHPVersionFromBuildInfoDependency(t *testing.T) {
	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{
				Path:    frankenPHPModulePath,
				Version: startupLogTestDependencyVersion,
			},
		},
	}

	if got := frankenPHPVersionFromBuildInfo(info); got != startupLogTestDependencyVersion {
		t.Fatalf("expected FrankenPHP dependency version %q, got %q", startupLogTestDependencyVersion, got)
	}
}

func TestFrankenPHPVersionFromBuildInfoMainModule(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{
			Path:    frankenPHPModulePath,
			Version: startupLogTestMainVersion,
		},
	}

	if got := frankenPHPVersionFromBuildInfo(info); got != startupLogTestMainVersion {
		t.Fatalf("expected FrankenPHP main module version %q, got %q", startupLogTestMainVersion, got)
	}
}

func TestFrankenPHPVersionFromExecutable(t *testing.T) {
	version, err := frankenPHPVersionFromExecutable(
		func() (string, error) {
			return startupLogTestExecutablePath, nil
		},
		func(path string) (*debug.BuildInfo, error) {
			if path != startupLogTestExecutablePath {
				t.Fatalf("expected executable path %q, got %q", startupLogTestExecutablePath, path)
			}
			return &debug.BuildInfo{
				Deps: []*debug.Module{
					{
						Path:    frankenPHPModulePath,
						Version: startupLogTestDependencyVersion,
					},
				},
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if version != startupLogTestDependencyVersion {
		t.Fatalf("expected FrankenPHP version %q, got %q", startupLogTestDependencyVersion, version)
	}
}

func TestFrankenPHPVersionFromExecutableErrors(t *testing.T) {
	_, err := frankenPHPVersionFromExecutable(
		func() (string, error) {
			return "", errors.New(startupLogTestExecutableError)
		},
		func(string) (*debug.BuildInfo, error) {
			t.Fatal("expected build info reader not to be called")
			return nil, nil
		},
	)
	if err == nil {
		t.Fatal("expected executable path error")
	}

	_, err = frankenPHPVersionFromExecutable(
		func() (string, error) {
			return startupLogTestExecutablePath, nil
		},
		func(string) (*debug.BuildInfo, error) {
			return nil, errors.New(startupLogTestBuildInfoError)
		},
	)
	if err == nil {
		t.Fatal("expected build info read error")
	}
}

func TestStartupLogAttrsIncludeFrankenPHPVersion(t *testing.T) {
	attrs := startupLogAttrs(startupLogTestDependencyVersion, startupLogTestPHPVersion, startupLogTestNumThreads, startupLogTestMaxThreads, startupLogTestMaxRequests)
	if len(attrs) == 0 {
		t.Fatal("expected startup log attrs")
	}
	if attrs[0].Key != startupLogAttrVersion {
		t.Fatalf("expected first startup log attr key %q, got %q", startupLogAttrVersion, attrs[0].Key)
	}
	if got := attrs[0].Value.String(); got != startupLogTestDependencyVersion {
		t.Fatalf("expected startup log version %q, got %q", startupLogTestDependencyVersion, got)
	}
}
