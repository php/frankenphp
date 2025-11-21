# Integration Test Fixtures

This directory contains Go source files used as test fixtures for the FrankenPHP extension-init integration tests.

## Overview

These fixtures test the full end-to-end workflow of the extension-init command:
1. Generating extension files from Go source code
2. Compiling FrankenPHP with the generated extension
3. Executing PHP code that uses the extension
4. Verifying the output

## Test Fixtures

### Happy Path Tests

#### `basic_function.go`
Tests basic function generation with primitive types:
- `test_uppercase(string): string` - String parameter and return
- `test_add_numbers(int, int): int` - Integer parameters
- `test_multiply(float, float): float` - Float parameters
- `test_is_enabled(bool): bool` - Boolean parameter

**What it tests:**
- Function parsing and generation
- Type conversion for all primitive types
- C/Go bridge code generation
- PHP stub file generation

#### `class_methods.go`
Tests opaque class generation with methods:
- `Counter` class - Integer counter with increment/decrement operations
- `StringHolder` class - String storage and manipulation

**What it tests:**
- Class declaration with `//export_php:class`
- Method declaration with `//export_php:method`
- Object lifecycle (creation and destruction)
- Method calls with various parameter and return types
- Nullable parameters (`?int`)
- Opaque object encapsulation (no direct property access)

#### `constants.go`
Tests constant generation and usage:
- Global constants (int, string, bool, float)
- Iota sequences for enumerations
- Class constants
- Functions using constants

**What it tests:**
- `//export_php:const` directive
- `//export_php:classconstant` directive
- Constant type detection and conversion
- Iota sequence handling
- Integration of constants with functions and classes

#### `namespace.go`
Tests namespace support:
- Functions in namespace `TestIntegration\Extension`
- Classes in namespace
- Constants in namespace

**What it tests:**
- `//export_php:namespace` directive
- Namespace declaration in stub files
- C name mangling for namespaces
- Proper scoping of functions, classes, and constants

### Error Case Tests

#### `invalid_signature.go`
Tests error handling for invalid function signatures:
- Function with unsupported return type

**What it tests:**
- Validation of return types
- Clear error messages for unsupported types
- Graceful failure during generation

#### `type_mismatch.go`
Tests error handling for type mismatches:
- PHP signature declares `int` but Go function expects `string`
- Method return type mismatch

**What it tests:**
- Parameter type validation
- Return type validation
- Type compatibility checking between PHP and Go

## Running Integration Tests Locally

Integration tests are tagged with `//go:build integration` and are skipped by default because they require:
1. PHP development headers (`php-config`)
2. PHP sources (for `gen_stub.php` script)
3. xcaddy (for building FrankenPHP)

### Prerequisites

1. **Install PHP development headers:**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install php-dev

   # macOS
   brew install php
   ```

2. **Download PHP sources:**
   ```bash
   wget https://www.php.net/distributions/php-8.4.0.tar.gz
   tar xzf php-8.4.0.tar.gz
   export GEN_STUB_SCRIPT=$PWD/php-8.4.0/build/gen_stub.php
   ```

3. **Install xcaddy:**
   ```bash
   go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
   ```

### Running the Tests

```bash
cd internal/extgen
go test -tags integration -v -timeout 30m
```

The timeout is set to 30 minutes because:
- Each test compiles a full FrankenPHP binary with xcaddy
- Multiple test scenarios are run sequentially
- Compilation can be slow on CI runners

### Skipping Tests

If any of the prerequisites are not met, the tests will be skipped automatically with a clear message:
- Missing `GEN_STUB_SCRIPT`: "Integration tests require PHP sources"
- Missing `xcaddy`: "Integration tests require xcaddy to build FrankenPHP"
- Missing `php-config`: "Integration tests require PHP development headers"

## CI Integration

Integration tests run automatically in CI on:
- Pull requests to `main` branch
- Pushes to `main` branch
- PHP versions: 8.3, 8.4
- Platform: Linux (Ubuntu)

The CI workflow (`.github/workflows/tests.yaml`) automatically:
1. Sets up Go and PHP
2. Installs xcaddy
3. Downloads PHP sources
4. Sets `GEN_STUB_SCRIPT` environment variable
5. Runs integration tests with 30-minute timeout

## Adding New Test Fixtures

To add a new integration test fixture:

1. **Create a new Go file** in this directory with your test code
2. **Use export_php directives** to declare functions, classes, or constants
3. **Add a new test function** in `internal/extgen/integration_test.go`:
   ```go
   func TestYourFeature(t *testing.T) {
       suite := setupTest(t)

       sourceFile := filepath.Join("..", "..", "testdata", "integration", "your_file.go")
       sourceFile, err := filepath.Abs(sourceFile)
       require.NoError(t, err)

       targetFile, err := suite.createGoModule(sourceFile)
       require.NoError(t, err)

       err = suite.runExtensionInit(targetFile)
       require.NoError(t, err)

       _, err = suite.compileFrankenPHP(filepath.Dir(targetFile))
       require.NoError(t, err)

       phpCode := `<?php /* your test PHP code */ `

       output, err := suite.runPHPCode(phpCode)
       require.NoError(t, err)

       // Add assertions
       assert.Contains(t, output, "expected output")
   }
   ```

4. **Document your fixture** in this README

## Test Coverage

Current integration test coverage:
- ✅ Basic functions with primitive types
- ✅ Classes with methods
- ✅ Nullable parameters
- ✅ Global and class constants
- ✅ Iota sequences
- ✅ Namespaces
- ✅ Invalid signatures (error case)
- ✅ Type mismatches (error case)
- ✅ Missing gen_stub.php (error case)

Future coverage goals:
- Array handling (packed, associative, maps)
- Default parameter values
- Multiple namespaces (error case)
- Complex nested types
- Performance benchmarks
