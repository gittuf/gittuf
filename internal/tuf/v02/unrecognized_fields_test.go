// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootMetadata_UnrecognizedFields(t *testing.T) {
	tests := []struct {
		name           string
		inputJSON      string
		expectedFields map[string]string
		expectWarning  bool
	}{
		{
			name: "no unrecognized fields",
			inputJSON: `{
				"type": "root",
				"schemaVersion": "https://gittuf.dev/policy/root/v0.2",
				"expires": "2025-01-01T00:00:00Z",
				"principals": {},
				"roles": {}
			}`,
			expectedFields: map[string]string{},
			expectWarning:  false,
		},
		{
			name: "single unrecognized field",
			inputJSON: `{
				"type": "root",
				"schemaVersion": "https://gittuf.dev/policy/root/v0.2",
				"expires": "2025-01-01T00:00:00Z",
				"principals": {},
				"roles": {},
				"newField": "newValue"
			}`,
			expectedFields: map[string]string{
				"newField": `"newValue"`,
			},
			expectWarning: true,
		},
		{
			name: "multiple unrecognized fields",
			inputJSON: `{
				"type": "root",
				"schemaVersion": "https://gittuf.dev/policy/root/v0.2",
				"expires": "2025-01-01T00:00:00Z",
				"principals": {},
				"roles": {},
				"newField1": "value1",
				"newField2": {"nested": "object"},
				"newField3": [1, 2, 3]
			}`,
			expectedFields: map[string]string{
				"newField1": `"value1"`,
				"newField2": `{"nested":"object"}`,
				"newField3": `[1,2,3]`,
			},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rootMetadata RootMetadata
			
			err := json.Unmarshal([]byte(tt.inputJSON), &rootMetadata)
			require.NoError(t, err)

			// Check that unrecognized fields are preserved
			assert.Len(t, rootMetadata.UnrecognizedFields, len(tt.expectedFields))
			for fieldName, expectedValue := range tt.expectedFields {
				actualValue, exists := rootMetadata.UnrecognizedFields[fieldName]
				assert.True(t, exists, "Expected field %s to be preserved", fieldName)
				assert.JSONEq(t, expectedValue, string(actualValue))
			}

			// Test that marshaling preserves unrecognized fields
			marshaledData, err := json.Marshal(&rootMetadata)
			require.NoError(t, err)

			// Unmarshal the marshaled data to verify fields are preserved
			var remarshaled map[string]interface{}
			err = json.Unmarshal(marshaledData, &remarshaled)
			require.NoError(t, err)

			for fieldName := range tt.expectedFields {
				_, exists := remarshaled[fieldName]
				assert.True(t, exists, "Expected field %s to be preserved after marshaling", fieldName)
			}
		})
	}
}

func TestTargetsMetadata_UnrecognizedFields(t *testing.T) {
	tests := []struct {
		name           string
		inputJSON      string
		expectedFields map[string]string
		expectWarning  bool
	}{
		{
			name: "no unrecognized fields",
			inputJSON: `{
				"type": "targets",
				"schemaVersion": "https://gittuf.dev/policy/targets/v0.2",
				"expires": "2025-01-01T00:00:00Z",
				"targets": {},
				"delegations": {
					"principals": {},
					"roles": []
				}
			}`,
			expectedFields: map[string]string{},
			expectWarning:  false,
		},
		{
			name: "single unrecognized field",
			inputJSON: `{
				"type": "targets",
				"schemaVersion": "https://gittuf.dev/policy/targets/v0.2",
				"expires": "2025-01-01T00:00:00Z",
				"targets": {},
				"delegations": {
					"principals": {},
					"roles": []
				},
				"newTargetField": "newValue"
			}`,
			expectedFields: map[string]string{
				"newTargetField": `"newValue"`,
			},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var targetsMetadata TargetsMetadata
			
			err := json.Unmarshal([]byte(tt.inputJSON), &targetsMetadata)
			require.NoError(t, err)

			// Check that unrecognized fields are preserved
			assert.Len(t, targetsMetadata.UnrecognizedFields, len(tt.expectedFields))
			for fieldName, expectedValue := range tt.expectedFields {
				actualValue, exists := targetsMetadata.UnrecognizedFields[fieldName]
				assert.True(t, exists, "Expected field %s to be preserved", fieldName)
				assert.JSONEq(t, expectedValue, string(actualValue))
			}

			// Test that marshaling preserves unrecognized fields
			marshaledData, err := json.Marshal(&targetsMetadata)
			require.NoError(t, err)

			// Unmarshal the marshaled data to verify fields are preserved
			var remarshaled map[string]interface{}
			err = json.Unmarshal(marshaledData, &remarshaled)
			require.NoError(t, err)

			for fieldName := range tt.expectedFields {
				_, exists := remarshaled[fieldName]
				assert.True(t, exists, "Expected field %s to be preserved after marshaling", fieldName)
			}
		})
	}
}

func TestDelegations_UnrecognizedFields(t *testing.T) {
	tests := []struct {
		name           string
		inputJSON      string
		expectedFields map[string]string
		expectWarning  bool
	}{
		{
			name: "no unrecognized fields",
			inputJSON: `{
				"principals": {},
				"roles": []
			}`,
			expectedFields: map[string]string{},
			expectWarning:  false,
		},
		{
			name: "single unrecognized field",
			inputJSON: `{
				"principals": {},
				"roles": [],
				"newDelegationField": "delegationValue"
			}`,
			expectedFields: map[string]string{
				"newDelegationField": `"delegationValue"`,
			},
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var delegations Delegations
			
			err := json.Unmarshal([]byte(tt.inputJSON), &delegations)
			require.NoError(t, err)

			// Check that unrecognized fields are preserved
			assert.Len(t, delegations.UnrecognizedFields, len(tt.expectedFields))
			for fieldName, expectedValue := range tt.expectedFields {
				actualValue, exists := delegations.UnrecognizedFields[fieldName]
				assert.True(t, exists, "Expected field %s to be preserved", fieldName)
				assert.JSONEq(t, expectedValue, string(actualValue))
			}

			// Test that marshaling preserves unrecognized fields
			marshaledData, err := json.Marshal(&delegations)
			require.NoError(t, err)

			// Unmarshal the marshaled data to verify fields are preserved
			var remarshaled map[string]interface{}
			err = json.Unmarshal(marshaledData, &remarshaled)
			require.NoError(t, err)

			for fieldName := range tt.expectedFields {
				_, exists := remarshaled[fieldName]
				assert.True(t, exists, "Expected field %s to be preserved after marshaling", fieldName)
			}
		})
	}
}

func TestUnrecognizedFields_RoundTrip(t *testing.T) {
	// Test that unrecognized fields survive multiple marshal/unmarshal cycles
	originalJSON := `{
		"type": "root",
		"schemaVersion": "https://gittuf.dev/policy/root/v0.2",
		"expires": "2025-01-01T00:00:00Z",
		"principals": {},
		"roles": {},
		"futureField1": "value1",
		"futureField2": {"complex": {"nested": "object"}},
		"futureField3": [1, 2, {"array": "element"}]
	}`

	// First unmarshal
	var rootMetadata1 RootMetadata
	err := json.Unmarshal([]byte(originalJSON), &rootMetadata1)
	require.NoError(t, err)

	// First marshal
	marshaledData1, err := json.Marshal(&rootMetadata1)
	require.NoError(t, err)

	// Second unmarshal
	var rootMetadata2 RootMetadata
	err = json.Unmarshal(marshaledData1, &rootMetadata2)
	require.NoError(t, err)

	// Second marshal
	marshaledData2, err := json.Marshal(&rootMetadata2)
	require.NoError(t, err)

	// Verify that the final result contains all the original fields
	var finalResult map[string]interface{}
	err = json.Unmarshal(marshaledData2, &finalResult)
	require.NoError(t, err)

	expectedFields := []string{"futureField1", "futureField2", "futureField3"}
	for _, fieldName := range expectedFields {
		_, exists := finalResult[fieldName]
		assert.True(t, exists, "Expected field %s to survive round-trip", fieldName)
	}
}

func TestUnrecognizedFields_ModificationPreservation(t *testing.T) {
	// Test that unrecognized fields are preserved when making modifications
	originalJSON := `{
		"type": "root",
		"schemaVersion": "https://gittuf.dev/policy/root/v0.2",
		"expires": "2025-01-01T00:00:00Z",
		"principals": {},
		"roles": {},
		"futureFeature": "importantValue"
	}`

	var rootMetadata RootMetadata
	err := json.Unmarshal([]byte(originalJSON), &rootMetadata)
	require.NoError(t, err)

	// Modify a known field
	rootMetadata.Expires = "2026-01-01T00:00:00Z"

	// Marshal the modified metadata
	marshaledData, err := json.Marshal(&rootMetadata)
	require.NoError(t, err)

	// Verify that both the modification and unrecognized field are preserved
	var result map[string]interface{}
	err = json.Unmarshal(marshaledData, &result)
	require.NoError(t, err)

	assert.Equal(t, "2026-01-01T00:00:00Z", result["expires"])
	assert.Equal(t, "importantValue", result["futureFeature"])
}