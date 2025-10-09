package extgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestSourceAnalyzer_Analyze(t *testing.T) {
	tests := []struct {
		name              string
		sourceContent     string
		expectedImports   []string
		expectedVariables []string
		expectedFunctions []string
		expectError       bool
	}{
		{
			name: "simple file with imports and functions",
			sourceContent: `package main

import (
	"fmt"
	"strings"
)

func regularFunction() {
	fmt.Println("hello")
}

//export_php:function
func exportedFunction() string {
	return "exported"
}`,
			expectedImports:   []string{`"fmt"`, `"strings"`},
			expectedVariables: nil,
			expectedFunctions: []string{
				`func regularFunction() {
	fmt.Println("hello")
}`,
			},
			expectError: false,
		},
		{
			name: "file with named imports",
			sourceContent: `package main

import (
	custom "fmt"
	. "strings"
	_ "os"
)

func test() {}`,
			expectedImports:   []string{`custom "fmt"`, `. "strings"`, `_ "os"`},
			expectedVariables: nil,
			expectedFunctions: []string{
				`func test() {}`,
			},
			expectError: false,
		},
		{
			name: "file with multiple functions and export comments",
			sourceContent: `package main

func internalOne() {
	// some code
}

// This function is exported to PHP
//export_php:function
func exportedOne() int {
	return 42
}

func internalTwo() string {
	return "internal"
}

// Another exported function
//export_php:function  
func exportedTwo() bool {
	return true
}`,
			expectedImports:   []string{},
			expectedVariables: nil,
			expectedFunctions: []string{
				`func internalOne() {
	// some code
}`,
				`func internalTwo() string {
	return "internal"
}`,
			},
			expectError: false,
		},
		{
			name: "file with nested braces",
			sourceContent: `package main

func complexFunction() {
	if true {
		for i := 0; i < 10; i++ {
			if i%2 == 0 {
				fmt.Println(i)
			}
		}
	}
}

//export_php:function
func exportedComplex() {
	obj := struct{
		field string
	}{
		field: "value",
	}
	fmt.Println(obj)
}`,
			expectedImports:   []string{},
			expectedVariables: nil,
			expectedFunctions: []string{
				`func complexFunction() {
	if true {
		for i := 0; i < 10; i++ {
			if i%2 == 0 {
				fmt.Println(i)
			}
		}
	}
}`,
			},
			expectError: false,
		},
		{
			name:              "empty file",
			sourceContent:     `package main`,
			expectedImports:   []string{},
			expectedFunctions: []string{},
			expectError:       false,
		},
		{
			name: "file with only exported functions",
			sourceContent: `package main

//export_php:function
func onlyExported() {}

//export_php:function
func anotherExported() string {
	return "test"
}`,
			expectedImports:   []string{},
			expectedFunctions: []string{},
			expectError:       false,
		},
		{
			name: "file with export comment not immediately before function",
			sourceContent: `package main

//export_php:function
// Some other comment
func shouldNotBeExported() {}

func normalFunction() {
	//export_php:function inside function should not count
}`,
			expectedImports:   []string{},
			expectedVariables: nil,
			expectedFunctions: []string{
				`func normalFunction() {
	//export_php:function inside function should not count
}`,
			},
			expectError: false,
		},
		{
			name: "file with variable blocks",
			sourceContent: `package main

import (
	"sync"
)

var (
	mu    sync.RWMutex
	store = map[string]struct {
		val     string
		expires int64
	}{}
)

var singleVar = "test"

func testFunction() {
	// test function
}`,
			expectedImports: []string{`"sync"`},
			expectedVariables: []string{
				`var (
	mu    sync.RWMutex
	store = map[string]struct {
		val     string
		expires int64
	}{}
)`,
				`var singleVar = "test"`,
			},
			expectedFunctions: []string{
				`func testFunction() {
	// test function
}`,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "test.go")

			require.NoError(t, os.WriteFile(filename, []byte(tt.sourceContent), 0644))

			analyzer := &SourceAnalyzer{}
			imports, variables, functions, err := analyzer.analyze(filename)

			if tt.expectError {
				assert.Error(t, err, "expected error")
				return
			}

			assert.NoError(t, err, "unexpected error")

			if len(imports) != 0 && len(tt.expectedImports) != 0 {
				assert.Equal(t, tt.expectedImports, imports, "imports mismatch")
			}

			assert.Equal(t, tt.expectedVariables, variables, "variables mismatch")
			assert.Len(t, functions, len(tt.expectedFunctions), "function count mismatch")

			for i, expected := range tt.expectedFunctions {
				assert.Equal(t, expected, functions[i], "function %d mismatch", i)
			}
		})
	}
}

func TestSourceAnalyzer_Analyze_InvalidFile(t *testing.T) {
	analyzer := &SourceAnalyzer{}

	t.Run("nonexistent file", func(t *testing.T) {
		_, _, _, err := analyzer.analyze("/nonexistent/file.go")
		assert.Error(t, err, "expected error for nonexistent file")
	})

	t.Run("invalid Go syntax", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "invalid.go")

		invalidContent := `package main
		func incomplete( {
			// invalid syntax
		`

		require.NoError(t, os.WriteFile(filename, []byte(invalidContent), 0644))

		_, _, _, err := analyzer.analyze(filename)
		assert.Error(t, err, "expected error for invalid syntax")
	})
}

func TestSourceAnalyzer_ExtractInternalFunctions(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "single function without export",
			content: `func test() {
	fmt.Println("test")
}`,
			expected: []string{
				`func test() {
	fmt.Println("test")
}`,
			},
		},
		{
			name: "function with export comment",
			content: `//export_php:function
func exported() {}`,
			expected: []string{},
		},
		{
			name: "mixed functions",
			content: `func internal() {}

//export_php:function
func exported() {}

func anotherInternal() {
	return "test"
}`,
			expected: []string{
				"func internal() {}",
				`func anotherInternal() {
	return "test"
}`,
			},
		},
		{
			name: "export comment with spacing",
			content: `//export_php:function
func exported1() {}

//export_php:function
func exported2() {}

// export_php:function   
func exported3() {}`,
			expected: []string{},
		},
		{
			name: "complex function with nested braces",
			content: `func complex() {
	if true {
		for {
			switch x {
			case 1:
				{
					// nested block
				}
			}
		}
	}
}`,
			expected: []string{
				`func complex() {
	if true {
		for {
			switch x {
			case 1:
				{
					// nested block
				}
			}
		}
	}
}`,
			},
		},
		{
			name:     "empty content",
			content:  "",
			expected: []string{},
		},
		{
			name: "no functions",
			content: `package main

import "fmt"

var x = 10`,
			expected: []string{},
		},
	}

	analyzer := &SourceAnalyzer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.extractInternalFunctions(tt.content)

			assert.Len(t, result, len(tt.expected), "function count mismatch")

			for i, expected := range tt.expected {
				assert.Equal(t, expected, result[i], "function %d mismatch", i)
			}
		})
	}
}

func BenchmarkSourceAnalyzer_Analyze(b *testing.B) {
	content := `package main

import (
	"fmt"
	"strings"
	"os"
)

func internalOne() {
	fmt.Println("test")
}

//export_php:function
func exported() string {
	return "exported"
}

func internalTwo() {
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			fmt.Println(i)
		}
	}
}`

	tempDir := b.TempDir()
	filename := filepath.Join(tempDir, "bench.go")

	require.NoError(b, os.WriteFile(filename, []byte(content), 0644))

	analyzer := &SourceAnalyzer{}

	for b.Loop() {
		_, _, _, err := analyzer.analyze(filename)
		require.NoError(b, err)
	}
}

func BenchmarkSourceAnalyzer_ExtractInternalFunctions(b *testing.B) {
	content := `func test1() { fmt.Println("1") }
func test2() { fmt.Println("2") }
//export_php:function
func exported() {}
func test3() { 
	for i := 0; i < 10; i++ {
		fmt.Println(i)
	}
}`

	analyzer := &SourceAnalyzer{}

	for b.Loop() {
		analyzer.extractInternalFunctions(content)
	}
}
