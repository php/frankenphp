//go:build integration

package extgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testModuleName = "github.com/frankenphp/test-extension"
)

type IntegrationTestSuite struct {
	tempDir        string
	genStubScript  string
	xcaddyPath     string
	frankenphpPath string
	phpConfigPath  string
	t              *testing.T
}

func setupTest(t *testing.T) *IntegrationTestSuite {
	t.Helper()

	suite := &IntegrationTestSuite{t: t}

	suite.genStubScript = os.Getenv("GEN_STUB_SCRIPT")
	if suite.genStubScript == "" {
		suite.genStubScript = "/usr/local/src/php/build/gen_stub.php"
	}

	if _, err := os.Stat(suite.genStubScript); os.IsNotExist(err) {
		t.Error("GEN_STUB_SCRIPT not found. Integration tests require PHP sources. Set GEN_STUB_SCRIPT environment variable.")
	}

	xcaddyPath, err := exec.LookPath("xcaddy")
	if err != nil {
		t.Error("xcaddy not found in PATH. Integration tests require xcaddy to build FrankenPHP.")
	}
	suite.xcaddyPath = xcaddyPath

	phpConfigPath, err := exec.LookPath("php-config")
	if err != nil {
		t.Error("php-config not found in PATH. Integration tests require PHP development headers.")
	}
	suite.phpConfigPath = phpConfigPath

	tempDir := t.TempDir()
	suite.tempDir = tempDir

	return suite
}

func (s *IntegrationTestSuite) createGoModule(sourceFile string) (string, error) {
	s.t.Helper()

	moduleDir := filepath.Join(s.tempDir, "module")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create module directory: %w", err)
	}

	// Get project root for replace directive
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", fmt.Errorf("failed to get project root: %w", err)
	}

	goModContent := fmt.Sprintf(`module %s

go 1.23

require github.com/dunglas/frankenphp v0.0.0

replace github.com/dunglas/frankenphp => %s
`, testModuleName, projectRoot)

	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		return "", fmt.Errorf("failed to create go.mod: %w", err)
	}

	sourceContent, err := os.ReadFile(sourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	targetFile := filepath.Join(moduleDir, filepath.Base(sourceFile))
	if err := os.WriteFile(targetFile, sourceContent, 0o644); err != nil {
		return "", fmt.Errorf("failed to write source file: %w", err)
	}

	return targetFile, nil
}

func (s *IntegrationTestSuite) runExtensionInit(sourceFile string) error {
	s.t.Helper()

	os.Setenv("GEN_STUB_SCRIPT", s.genStubScript)

	baseName := SanitizePackageName(strings.TrimSuffix(filepath.Base(sourceFile), ".go"))
	generator := Generator{
		BaseName:   baseName,
		SourceFile: sourceFile,
		BuildDir:   filepath.Dir(sourceFile),
	}

	if err := generator.Generate(); err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	return nil
}

// compileFrankenPHP compiles FrankenPHP with the generated extension
func (s *IntegrationTestSuite) compileFrankenPHP(moduleDir string) (string, error) {
	s.t.Helper()

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return "", fmt.Errorf("failed to get project root: %w", err)
	}

	cflags, err := exec.Command(s.phpConfigPath, "--includes").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get PHP includes: %w", err)
	}

	ldflags, err := exec.Command(s.phpConfigPath, "--ldflags").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get PHP ldflags: %w", err)
	}

	libs, err := exec.Command(s.phpConfigPath, "--libs").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get PHP libs: %w", err)
	}

	cgoCflags := strings.TrimSpace(string(cflags))
	cgoLdflags := strings.TrimSpace(string(ldflags)) + " " + strings.TrimSpace(string(libs))

	outputBinary := filepath.Join(s.tempDir, "frankenphp")

	cmd := exec.Command(
		s.xcaddyPath,
		"build",
		"--output", outputBinary,
		"--with", "github.com/dunglas/frankenphp="+projectRoot,
		"--with", "github.com/dunglas/frankenphp/caddy="+projectRoot+"/caddy",
		"--with", testModuleName+"="+moduleDir,
	)

	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"CGO_CFLAGS="+cgoCflags,
		"CGO_LDFLAGS="+cgoLdflags,
		fmt.Sprintf("XCADDY_GO_BUILD_FLAGS=-ldflags='-w -s' -tags=nobadger,nomysql,nopgx,nowatcher"),
	)

	cmd.Dir = s.tempDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("xcaddy build failed: %w\nOutput: %s", err, string(output))
	}

	s.frankenphpPath = outputBinary
	return outputBinary, nil
}

func (s *IntegrationTestSuite) runPHPCode(phpCode string) (string, error) {
	s.t.Helper()

	if s.frankenphpPath == "" {
		return "", fmt.Errorf("FrankenPHP not compiled yet")
	}

	phpFile := filepath.Join(s.tempDir, "test.php")
	if err := os.WriteFile(phpFile, []byte(phpCode), 0o644); err != nil {
		return "", fmt.Errorf("failed to create PHP file: %w", err)
	}

	cmd := exec.Command(s.frankenphpPath, "php-cli", phpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("PHP execution failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// verifyPHPSymbols checks if PHP can find the exposed functions, classes, and constants
func (s *IntegrationTestSuite) verifyPHPSymbols(functions []string, classes []string, constants []string) error {
	s.t.Helper()

	var checks []string

	for _, fn := range functions {
		checks = append(checks, fmt.Sprintf("if (!function_exists('%s')) { echo 'MISSING_FUNCTION: %s'; exit(1); }", fn, fn))
	}

	for _, cls := range classes {
		checks = append(checks, fmt.Sprintf("if (!class_exists('%s')) { echo 'MISSING_CLASS: %s'; exit(1); }", cls, cls))
	}

	for _, cnst := range constants {
		checks = append(checks, fmt.Sprintf("if (!defined('%s')) { echo 'MISSING_CONSTANT: %s'; exit(1); }", cnst, cnst))
	}

	checks = append(checks, "echo 'OK';")

	phpCode := "<?php\n" + strings.Join(checks, "\n")

	output, err := s.runPHPCode(phpCode)
	if err != nil {
		return err
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("symbol verification failed: %s", output)
	}

	return nil
}

func TestBasicFunction(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "basic_function.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)

	targetFile, err := suite.createGoModule(sourceFile)
	require.NoError(t, err)

	err = suite.runExtensionInit(targetFile)
	require.NoError(t, err, "extension-init should succeed")

	baseDir := filepath.Dir(targetFile)
	baseName := strings.TrimSuffix(filepath.Base(targetFile), ".go")

	expectedFiles := []string{
		baseName + ".stub.php",
		baseName + "_arginfo.h",
		baseName + ".h",
		baseName + ".c",
		baseName + ".go",
		"README.md",
	}

	for _, file := range expectedFiles {
		fullPath := filepath.Join(baseDir, file)
		assert.FileExists(t, fullPath, "Generated file should exist: %s", file)
	}

	_, err = suite.compileFrankenPHP(filepath.Dir(targetFile))
	require.NoError(t, err, "FrankenPHP compilation should succeed")

	err = suite.verifyPHPSymbols(
		[]string{"test_uppercase", "test_add_numbers", "test_multiply", "test_is_enabled"},
		[]string{},
		[]string{},
	)
	require.NoError(t, err, "all functions should be accessible from PHP")
}

func TestClassMethodsIntegration(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "class_methods.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)

	targetFile, err := suite.createGoModule(sourceFile)
	require.NoError(t, err)

	err = suite.runExtensionInit(targetFile)
	require.NoError(t, err)

	_, err = suite.compileFrankenPHP(filepath.Dir(targetFile))
	require.NoError(t, err)

	err = suite.verifyPHPSymbols(
		[]string{},
		[]string{"Counter", "StringHolder"},
		[]string{},
	)
	require.NoError(t, err, "all classes should be accessible from PHP")
}

func TestConstants(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "constants.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)

	targetFile, err := suite.createGoModule(sourceFile)
	require.NoError(t, err)

	err = suite.runExtensionInit(targetFile)
	require.NoError(t, err)

	_, err = suite.compileFrankenPHP(filepath.Dir(targetFile))
	require.NoError(t, err)

	err = suite.verifyPHPSymbols(
		[]string{"test_with_constants"},
		[]string{"Config"},
		[]string{
			"TEST_MAX_RETRIES", "TEST_API_VERSION", "TEST_ENABLED", "TEST_PI",
			"STATUS_PENDING", "STATUS_PROCESSING", "STATUS_COMPLETED",
		},
	)
	require.NoError(t, err, "all constants, functions, and classes should be accessible from PHP")
}

func TestNamespace(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "namespace.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)

	targetFile, err := suite.createGoModule(sourceFile)
	require.NoError(t, err)

	err = suite.runExtensionInit(targetFile)
	require.NoError(t, err)

	_, err = suite.compileFrankenPHP(filepath.Dir(targetFile))
	require.NoError(t, err)

	err = suite.verifyPHPSymbols(
		[]string{`\\TestIntegration\\Extension\\greet`},
		[]string{`\\TestIntegration\\Extension\\Person`},
		[]string{`\\TestIntegration\\Extension\\NAMESPACE_VERSION`},
	)
	require.NoError(t, err, "all namespaced symbols should be accessible from PHP")
}

func TestInvalidSignature(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "invalid_signature.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)

	targetFile, err := suite.createGoModule(sourceFile)
	require.NoError(t, err)

	err = suite.runExtensionInit(targetFile)
	assert.Error(t, err, "extension-init should fail for invalid return type")
	assert.Contains(t, err.Error(), "no PHP functions, classes, or constants found", "invalid functions should be ignored, resulting in no valid exports")
}

func TestTypeMismatch(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "type_mismatch.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)

	targetFile, err := suite.createGoModule(sourceFile)
	require.NoError(t, err)

	err = suite.runExtensionInit(targetFile)
	assert.NoError(t, err, "generation should succeed - class is valid even though function/method have type mismatches")

	baseDir := filepath.Dir(targetFile)
	baseName := strings.TrimSuffix(filepath.Base(targetFile), ".go")
	stubFile := filepath.Join(baseDir, baseName+".stub.php")
	assert.FileExists(t, stubFile, "stub file should be generated for valid class")
}

func TestMissingGenStub(t *testing.T) {
	// temp override of GEN_STUB_SCRIPT
	originalValue := os.Getenv("GEN_STUB_SCRIPT")
	defer os.Setenv("GEN_STUB_SCRIPT", originalValue)

	os.Setenv("GEN_STUB_SCRIPT", "/nonexistent/gen_stub.php")

	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "test.go")

	err := os.WriteFile(sourceFile, []byte(`package test

//export_php:function dummy(): void
func dummy() {}
`), 0o644)
	require.NoError(t, err)

	baseName := SanitizePackageName(strings.TrimSuffix(filepath.Base(sourceFile), ".go"))
	gen := Generator{
		BaseName:   baseName,
		SourceFile: sourceFile,
		BuildDir:   filepath.Dir(sourceFile),
	}

	err = gen.Generate()
	assert.Error(t, err, "should fail when gen_stub.php is missing")
	assert.Contains(t, err.Error(), "gen_stub.php", "error should mention missing script")
}
