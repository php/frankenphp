package extgen

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespaceParser(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    string
		shouldError bool
	}{
		{
			name: "basic namespace",
			content: `package main

//export_php:namespace My\Test\Namespace

func main() {}`,
			expected: `My\Test\Namespace`,
		},
		{
			name: "namespace with spaces",
			content: `package main

//export_php:namespace   My\Test\Namespace   

func main() {}`,
			expected: `My\Test\Namespace`,
		},
		{
			name: "no namespace",
			content: `package main

func main() {}`,
			expected: "",
		},
		{
			name: "multiple namespaces should error",
			content: `package main

//export_php:namespace First\Namespace
//export_php:namespace Second\Namespace

func main() {}`,
			expected:    "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "test_namespace_*.go")
			require.NoError(t, err, "Failed to create temp file")
			defer func() {
				err := os.Remove(tmpfile.Name())
				assert.NoError(t, err, "Failed to remove temp file: %v", err)
			}()

			_, err = tmpfile.Write([]byte(tt.content))
			require.NoError(t, err, "Failed to write to temp file")

			err = tmpfile.Close()
			require.NoError(t, err, "Failed to close temp file")

			parser := NamespaceParser{}
			result, err := parser.parse(tmpfile.Name())

			if tt.shouldError {
				require.Error(t, err, "expected error but got none")
				return
			}
			require.NoError(t, err, "unexpected error")
			require.Equal(t, tt.expected, result, "expected %q, got %q", tt.expected, result)
		})
	}
}

func TestGeneratorWithNamespace(t *testing.T) {
	content := `package main

//export_php:namespace My\Test\Namespace

//export_php:function hello(): string
func hello() string {
	return "Hello from namespace!"
}

//export_php:constant TEST_CONSTANT = "test_value"
const TEST_CONSTANT = "test_value"
`

	tmpfile, err := os.CreateTemp("", "test_generator_namespace_*.go")
	require.NoError(t, err, "Failed to create temp file")
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	_, err = tmpfile.Write([]byte(content))
	require.NoError(t, err, "Failed to write to temp file")

	err = tmpfile.Close()
	require.NoError(t, err, "Failed to close temp file")

	parser := SourceParser{}
	namespace, err := parser.ParseNamespace(tmpfile.Name())
	require.NoErrorf(t, err, "Failed to parse namespace from %s: %v", tmpfile.Name(), err)

	require.Equal(t, `My\Test\Namespace`, namespace, "Namespace should match the parsed namespace")

	generator := &Generator{
		SourceFile: tmpfile.Name(),
		Namespace:  namespace,
	}

	require.Equal(t, `My\Test\Namespace`, generator.Namespace, "Namespace should match the parsed namespace")
}
