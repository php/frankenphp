package extgen

import (
	"bufio"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModuleParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *phpModule
	}{
		{
			name: "both init and shutdown",
			input: `package main

//export_php:module init
func initializeModule() {
	// Initialization code
}

//export_php:module shutdown
func cleanupModule() {
	// Cleanup code
}`,
			expected: &phpModule{
				InitFunc:     "initializeModule",
				ShutdownFunc: "cleanupModule",
			},
		},
		{
			name: "only init function",
			input: `package main

//export_php:module init
func initializeModule() {
	// Initialization code
}`,
			expected: &phpModule{
				InitFunc:     "initializeModule",
				ShutdownFunc: "",
			},
		},
		{
			name: "only shutdown function",
			input: `package main

//export_php:module shutdown
func cleanupModule() {
	// Cleanup code
}`,
			expected: &phpModule{
				InitFunc:     "",
				ShutdownFunc: "cleanupModule",
			},
		},
		{
			name: "with extra whitespace",
			input: `package main

//export_php:module   init  
func initModule() {
	// Initialization code
}

//export_php:module   shutdown  
func shutdownModule() {
	// Cleanup code
}`,
			expected: &phpModule{
				InitFunc:     "initModule",
				ShutdownFunc: "shutdownModule",
			},
		},
		{
			name: "no module directive",
			input: `package main

func regularFunction() {
	// Just a regular Go function
}`,
			expected: nil,
		},
		{
			name: "functions with braces",
			input: `package main

//export_php:module init
func initModule() {
	if true {
		// Do something
	}
	for i := 0; i < 10; i++ {
		// Loop
	}
}

//export_php:module shutdown
func shutdownModule() {
	if true {
		// Do something else
	}
}`,
			expected: &phpModule{
				InitFunc:     "initModule",
				ShutdownFunc: "shutdownModule",
			},
		},
		{
			name: "multiple functions between directives",
			input: `package main

//export_php:module init
func initModule() {
	// Init code
}

func someOtherFunction() {
	// This should be ignored
}

//export_php:module shutdown
func shutdownModule() {
	// Shutdown code
}`,
			expected: &phpModule{
				InitFunc:     "initModule",
				ShutdownFunc: "shutdownModule",
			},
		},
		{
			name: "directive without function",
			input: `package main

//export_php:module init
// No function follows

func regularFunction() {
	// This should be ignored
}`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fileName := filepath.Join(tmpDir, tt.name+".go")
			require.NoError(t, os.WriteFile(fileName, []byte(tt.input), 0644))

			parser := &ModuleParser{}
			module, err := parser.parse(fileName)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, module, "parse() should return nil for no module directive")
			} else {
				assert.NotNil(t, module, "parse() should not return nil")
				assert.Equal(t, tt.expected.InitFunc, module.InitFunc, "InitFunc mismatch")
				assert.Equal(t, tt.expected.ShutdownFunc, module.ShutdownFunc, "ShutdownFunc mismatch")
				
				// Check that function code was extracted
				if tt.expected.InitFunc != "" {
					assert.Contains(t, module.InitCode, "func "+tt.expected.InitFunc, "InitCode should contain function declaration")
					assert.True(t, strings.HasSuffix(module.InitCode, "}\n"), "InitCode should end with closing brace")
				}
				
				if tt.expected.ShutdownFunc != "" {
					assert.Contains(t, module.ShutdownCode, "func "+tt.expected.ShutdownFunc, "ShutdownCode should contain function declaration")
					assert.True(t, strings.HasSuffix(module.ShutdownCode, "}\n"), "ShutdownCode should end with closing brace")
				}
			}
		})
	}
}

func TestExtractGoFunction(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		firstLine      string
		expectedName   string
		expectedPrefix string
		expectedSuffix string
	}{
		{
			name:           "simple function",
			input:          "func testFunc() {\n\t// Some code\n}\n",
			firstLine:      "func testFunc() {",
			expectedName:   "testFunc",
			expectedPrefix: "func testFunc() {",
			expectedSuffix: "}\n",
		},
		{
			name:           "function with parameters",
			input:          "func initModule(param1 string, param2 int) {\n\t// Init code\n}\n",
			firstLine:      "func initModule(param1 string, param2 int) {",
			expectedName:   "initModule",
			expectedPrefix: "func initModule(param1 string, param2 int) {",
			expectedSuffix: "}\n",
		},
		{
			name:           "function with nested braces",
			input:          "func complexFunc() {\n\tif true {\n\t\t// Nested code\n\t}\n}\n",
			firstLine:      "func complexFunc() {",
			expectedName:   "complexFunc",
			expectedPrefix: "func complexFunc() {",
			expectedSuffix: "}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &ModuleParser{}
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			scanner.Scan() // Read the first line
			
			name, code, err := parser.extractGoFunction(scanner, tt.firstLine)
			
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedName, name)
			assert.True(t, strings.HasPrefix(code, tt.expectedPrefix), "Function code should start with the declaration")
			assert.True(t, strings.HasSuffix(code, tt.expectedSuffix), "Function code should end with closing brace")
		})
	}
}

func TestModuleParserFileErrors(t *testing.T) {
	parser := &ModuleParser{}
	
	// Test with non-existent file
	module, err := parser.parse("non_existent_file.go")
	assert.Error(t, err, "parse() should return error for non-existent file")
	assert.Nil(t, module, "parse() should return nil for non-existent file")
}