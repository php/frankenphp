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

go 1.25

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

// cleanupGeneratedFiles removes generated files from the original source directory
func (s *IntegrationTestSuite) cleanupGeneratedFiles(originalSourceFile string) {
	s.t.Helper()

	sourceDir := filepath.Dir(originalSourceFile)
	baseName := SanitizePackageName(strings.TrimSuffix(filepath.Base(originalSourceFile), ".go"))

	generatedFiles := []string{
		baseName + ".stub.php",
		baseName + "_arginfo.h",
		baseName + ".h",
		baseName + ".c",
		baseName + ".go",
		"README.md",
	}

	for _, file := range generatedFiles {
		fullPath := filepath.Join(sourceDir, file)
		if _, err := os.Stat(fullPath); err == nil {
			os.Remove(fullPath)
		}
	}
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

func (s *IntegrationTestSuite) verifyFunctionBehavior(phpCode string, expectedOutput string) error {
	s.t.Helper()

	output, err := s.runPHPCode(phpCode)
	if err != nil {
		return err
	}

	if !strings.Contains(output, expectedOutput) {
		return fmt.Errorf("unexpected output.\nExpected to contain: %q\nGot: %q", expectedOutput, output)
	}

	return nil
}

func TestBasicFunction(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "basic_function.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	defer suite.cleanupGeneratedFiles(sourceFile)

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

	err = suite.verifyFunctionBehavior(`<?php
$result = test_uppercase("hello world");
if ($result !== "HELLO WORLD") {
	echo "FAIL: test_uppercase expected 'HELLO WORLD', got '$result'";
	exit(1);
}

$result = test_uppercase("");
if ($result !== "") {
	echo "FAIL: test_uppercase with empty string expected '', got '$result'";
	exit(1);
}

$sum = test_add_numbers(5, 7);
if ($sum !== 12) {
	echo "FAIL: test_add_numbers(5, 7) expected 12, got $sum";
	exit(1);
}

$result = test_is_enabled(true);
if ($result !== false) {
	echo "FAIL: test_is_enabled(true) expected false, got " . ($result ? "true" : "false");
	exit(1);
}

$result = test_is_enabled(false);
if ($result !== true) {
	echo "FAIL: test_is_enabled(false) expected true, got " . ($result ? "true" : "false");
	exit(1);
}

echo "OK";
`, "OK")
	require.NoError(t, err, "all function calls should work correctly")
}

func TestClassMethodsIntegration(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "class_methods.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	defer suite.cleanupGeneratedFiles(sourceFile)

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

	err = suite.verifyFunctionBehavior(`<?php
$counter = new Counter();
if ($counter->getValue() !== 0) {
	echo "FAIL: Counter initial value expected 0, got " . $counter->getValue();
	exit(1);
}

$counter->increment();
if ($counter->getValue() !== 1) {
	echo "FAIL: Counter after increment expected 1, got " . $counter->getValue();
	exit(1);
}

$counter->decrement();
if ($counter->getValue() !== 0) {
	echo "FAIL: Counter after decrement expected 0, got " . $counter->getValue();
	exit(1);
}

$counter->setValue(10);
if ($counter->getValue() !== 10) {
	echo "FAIL: Counter after setValue(10) expected 10, got " . $counter->getValue();
	exit(1);
}

$newValue = $counter->addValue(5);
if ($newValue !== 15) {
	echo "FAIL: Counter addValue(5) expected to return 15, got $newValue";
	exit(1);
}
if ($counter->getValue() !== 15) {
	echo "FAIL: Counter value after addValue(5) expected 15, got " . $counter->getValue();
	exit(1);
}

$counter->updateWithNullable(50);
if ($counter->getValue() !== 50) {
	echo "FAIL: Counter after updateWithNullable(50) expected 50, got " . $counter->getValue();
	exit(1);
}

$counter->updateWithNullable(null);
if ($counter->getValue() !== 50) {
	echo "FAIL: Counter after updateWithNullable(null) expected 50 (unchanged), got " . $counter->getValue();
	exit(1);
}

$counter->reset();
if ($counter->getValue() !== 0) {
	echo "FAIL: Counter after reset expected 0, got " . $counter->getValue();
	exit(1);
}

$counter1 = new Counter();
$counter2 = new Counter();
$counter1->setValue(100);
$counter2->setValue(200);
if ($counter1->getValue() !== 100 || $counter2->getValue() !== 200) {
	echo "FAIL: Multiple Counter instances should be independent";
	exit(1);
}

$holder = new StringHolder();
$holder->setData("test string");
if ($holder->getData() !== "test string") {
	echo "FAIL: StringHolder getData expected 'test string', got '" . $holder->getData() . "'";
	exit(1);
}
if ($holder->getLength() !== 11) {
	echo "FAIL: StringHolder getLength expected 11, got " . $holder->getLength();
	exit(1);
}

$holder->setData("");
if ($holder->getData() !== "") {
	echo "FAIL: StringHolder empty string expected '', got '" . $holder->getData() . "'";
	exit(1);
}
if ($holder->getLength() !== 0) {
	echo "FAIL: StringHolder empty string length expected 0, got " . $holder->getLength();
	exit(1);
}

echo "OK";
`, "OK")
	require.NoError(t, err, "all class methods should work correctly")
}

func TestConstants(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "constants.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	defer suite.cleanupGeneratedFiles(sourceFile)

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
			"ONE", "TWO",
		},
	)
	require.NoError(t, err, "all constants, functions, and classes should be accessible from PHP")

	err = suite.verifyFunctionBehavior(`<?php
if (TEST_MAX_RETRIES !== 100) {
	echo "FAIL: TEST_MAX_RETRIES expected 100, got " . TEST_MAX_RETRIES;
	exit(1);
}

if (TEST_API_VERSION !== "2.0.0") {
	echo "FAIL: TEST_API_VERSION expected '2.0.0', got '" . TEST_API_VERSION . "'";
	exit(1);
}

if (TEST_ENABLED !== true) {
var_dump(TEST_ENABLED);
	echo "FAIL: TEST_ENABLED expected true, got " . (TEST_ENABLED ? "true" : "false");
	exit(1);
}

if (abs(TEST_PI - 3.14159) > 0.00001) {
	echo "FAIL: TEST_PI expected 3.14159, got " . TEST_PI;
	exit(1);
}

if (Config::MODE_DEBUG !== 1) {
	echo "FAIL: Config::MODE_DEBUG expected 1, got " . Config::MODE_DEBUG;
	exit(1);
}

if (Config::MODE_PRODUCTION !== 2) {
	echo "FAIL: Config::MODE_PRODUCTION expected 2, got " . Config::MODE_PRODUCTION;
	exit(1);
}

if (Config::DEFAULT_TIMEOUT !== 30) {
	echo "FAIL: Config::DEFAULT_TIMEOUT expected 30, got " . Config::DEFAULT_TIMEOUT;
	exit(1);
}

$config = new Config();
$config->setMode(Config::MODE_DEBUG);
if ($config->getMode() !== Config::MODE_DEBUG) {
	echo "FAIL: Config getMode expected MODE_DEBUG, got " . $config->getMode();
	exit(1);
}

$result = test_with_constants(STATUS_PENDING);
if ($result !== "pending") {
	echo "FAIL: test_with_constants(STATUS_PENDING) expected 'pending', got '$result'";
	exit(1);
}

$result = test_with_constants(STATUS_PROCESSING);
if ($result !== "processing") {
	echo "FAIL: test_with_constants(STATUS_PROCESSING) expected 'processing', got '$result'";
	exit(1);
}

$result = test_with_constants(STATUS_COMPLETED);
if ($result !== "completed") {
	echo "FAIL: test_with_constants(STATUS_COMPLETED) expected 'completed', got '$result'";
	exit(1);
}

$result = test_with_constants(999);
if ($result !== "unknown") {
	echo "FAIL: test_with_constants(999) expected 'unknown', got '$result'";
	exit(1);
}

echo "OK";
`, "OK")
	require.NoError(t, err, "all constants should have correct values and functions should work")
}

func TestNamespace(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "namespace.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	defer suite.cleanupGeneratedFiles(sourceFile)

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

	err = suite.verifyFunctionBehavior(`<?php
use TestIntegration\Extension;

if (Extension\NAMESPACE_VERSION !== "1.0.0") {
	echo "FAIL: NAMESPACE_VERSION expected '1.0.0', got '" . Extension\NAMESPACE_VERSION . "'";
	exit(1);
}

$greeting = Extension\greet("Alice");
if ($greeting !== "Hello, Alice!") {
	echo "FAIL: greet('Alice') expected 'Hello, Alice!', got '$greeting'";
	exit(1);
}

$greeting = Extension\greet("");
if ($greeting !== "Hello, !") {
	echo "FAIL: greet('') expected 'Hello, !', got '$greeting'";
	exit(1);
}

if (Extension\Person::DEFAULT_AGE !== 18) {
	echo "FAIL: Person::DEFAULT_AGE expected 18, got " . Extension\Person::DEFAULT_AGE;
	exit(1);
}

$person = new Extension\Person();
$person->setName("Bob");
$person->setAge(25);

if ($person->getName() !== "Bob") {
	echo "FAIL: Person getName expected 'Bob', got '" . $person->getName() . "'";
	exit(1);
}

if ($person->getAge() !== 25) {
	echo "FAIL: Person getAge expected 25, got " . $person->getAge();
	exit(1);
}

$person->setAge(Extension\Person::DEFAULT_AGE);
if ($person->getAge() !== 18) {
	echo "FAIL: Person setAge(DEFAULT_AGE) expected 18, got " . $person->getAge();
	exit(1);
}

$person1 = new Extension\Person();
$person2 = new Extension\Person();
$person1->setName("Alice");
$person1->setAge(30);
$person2->setName("Charlie");
$person2->setAge(40);

if ($person1->getName() !== "Alice" || $person1->getAge() !== 30) {
	echo "FAIL: person1 should have independent state";
	exit(1);
}
if ($person2->getName() !== "Charlie" || $person2->getAge() !== 40) {
	echo "FAIL: person2 should have independent state";
	exit(1);
}

echo "OK";
`, "OK")
	require.NoError(t, err, "all namespaced symbols should work correctly")
}

func TestInvalidSignature(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "invalid_signature.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	defer suite.cleanupGeneratedFiles(sourceFile)

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
	defer suite.cleanupGeneratedFiles(sourceFile)

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

func TestCallable(t *testing.T) {
	suite := setupTest(t)

	sourceFile := filepath.Join("..", "..", "testdata", "integration", "callable.go")
	sourceFile, err := filepath.Abs(sourceFile)
	require.NoError(t, err)
	defer suite.cleanupGeneratedFiles(sourceFile)

	targetFile, err := suite.createGoModule(sourceFile)
	require.NoError(t, err)

	err = suite.runExtensionInit(targetFile)
	require.NoError(t, err)

	_, err = suite.compileFrankenPHP(filepath.Dir(targetFile))
	require.NoError(t, err)

	err = suite.verifyPHPSymbols(
		[]string{"my_array_map", "my_filter"},
		[]string{"Processor"},
		[]string{},
	)
	require.NoError(t, err, "all functions and classes should be accessible from PHP")

	err = suite.verifyFunctionBehavior(`<?php

$result = my_array_map([1, 2, 3], function($x) { return $x * 2; });
if ($result !== [2, 4, 6]) {
	echo "FAIL: my_array_map with closure expected [2, 4, 6], got " . json_encode($result);
	exit(1);
}

$result = my_array_map(['hello', 'world'], 'strtoupper');
if ($result !== ['HELLO', 'WORLD']) {
	echo "FAIL: my_array_map with function name expected ['HELLO', 'WORLD'], got " . json_encode($result);
	exit(1);
}

$result = my_array_map([], function($x) { return $x; });
if ($result !== []) {
	echo "FAIL: my_array_map with empty array expected [], got " . json_encode($result);
	exit(1);
}

$result = my_filter([1, 2, 3, 4, 5, 6], function($x) { return $x % 2 === 0; });
if ($result !== [2, 4, 6]) {
	echo "FAIL: my_filter expected [2, 4, 6], got " . json_encode($result);
	exit(1);
}

$result = my_filter([1, 2, 3, 4], null);
if ($result !== [1, 2, 3, 4]) {
	echo "FAIL: my_filter with null callback expected [1, 2, 3, 4], got " . json_encode($result);
	exit(1);
}

$processor = new Processor();
$result = $processor->transform('hello', function($s) { return strtoupper($s); });
if ($result !== 'HELLO') {
	echo "FAIL: Processor::transform with closure expected 'HELLO', got '$result'";
	exit(1);
}

$result = $processor->transform('world', 'strtoupper');
if ($result !== 'WORLD') {
	echo "FAIL: Processor::transform with function name expected 'WORLD', got '$result'";
	exit(1);
}

$result = $processor->transform('  test  ', 'trim');
if ($result !== 'test') {
	echo "FAIL: Processor::transform with trim expected 'test', got '$result'";
	exit(1);
}

echo "OK";
`, "OK")
	require.NoError(t, err, "all callable tests should pass")
}
