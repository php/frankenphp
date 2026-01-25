package extgen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNamespacedName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		itemName  string
		expected  string
	}{
		{
			name:      "no namespace",
			namespace: "",
			itemName:  "TestItem",
			expected:  "TestItem",
		},
		{
			name:      "single level namespace",
			namespace: "MyNamespace",
			itemName:  "TestItem",
			expected:  "MyNamespace_TestItem",
		},
		{
			name:      "multi level namespace",
			namespace: `Go\Extension`,
			itemName:  "TestItem",
			expected:  "Go_Extension_TestItem",
		},
		{
			name:      "deep namespace",
			namespace: `Very\Deep\Nested\Namespace`,
			itemName:  "MyItem",
			expected:  "Very_Deep_Nested_Namespace_MyItem",
		},
		{
			name:      "function name",
			namespace: `Go\Extension`,
			itemName:  "multiply",
			expected:  "Go_Extension_multiply",
		},
		{
			name:      "class name",
			namespace: `Go\Extension`,
			itemName:  "MySuperClass",
			expected:  "Go_Extension_MySuperClass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NamespacedName(tt.namespace, tt.itemName)
			require.Equal(t, tt.expected, result, "NamespacedName(%q, %q) = %q, expected %q", tt.namespace, tt.itemName, result, tt.expected)
		})
	}
}
