// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIssue751_UnrecognizedFieldsPreservation tests the specific scenario described in issue #751
// where an older client modifies metadata containing newer fields, ensuring those fields are preserved
func TestIssue751_UnrecognizedFieldsPreservation(t *testing.T) {
	// Simulate metadata from a newer gittuf version with additional fields
	newerVersionJSON := `{
		"type": "root",
		"schemaVersion": "https://gittuf.dev/policy/root/v0.2",
		"expires": "2025-01-01T00:00:00Z",
		"principals": {},
		"roles": {},
		"newVerificationRule": "strict",
		"futureSecurityFeature": {
			"enabled": true,
			"algorithm": "post-quantum-crypto",
			"parameters": {
				"keySize": 4096,
				"rounds": 100
			}
		},
		"experimentalFeatures": ["feature1", "feature2"]
	}`

	// Step 1: Older client loads the metadata (this should warn about unrecognized fields)
	var rootMetadata RootMetadata
	err := json.Unmarshal([]byte(newerVersionJSON), &rootMetadata)
	require.NoError(t, err)

	// Verify that unrecognized fields are preserved
	assert.Len(t, rootMetadata.UnrecognizedFields, 3)
	assert.Contains(t, rootMetadata.UnrecognizedFields, "newVerificationRule")
	assert.Contains(t, rootMetadata.UnrecognizedFields, "futureSecurityFeature")
	assert.Contains(t, rootMetadata.UnrecognizedFields, "experimentalFeatures")

	// Step 2: Older client makes a modification to a known field
	rootMetadata.Expires = "2026-01-01T00:00:00Z"

	// Step 3: Older client saves the metadata
	modifiedJSON, err := json.Marshal(&rootMetadata)
	require.NoError(t, err)

	// Step 4: Verify that both the modification and unrecognized fields are preserved
	var result map[string]interface{}
	err = json.Unmarshal(modifiedJSON, &result)
	require.NoError(t, err)

	// Check that the modification was applied
	assert.Equal(t, "2026-01-01T00:00:00Z", result["expires"])

	// Check that all unrecognized fields are preserved
	assert.Equal(t, "strict", result["newVerificationRule"])
	
	futureFeature, ok := result["futureSecurityFeature"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, futureFeature["enabled"])
	assert.Equal(t, "post-quantum-crypto", futureFeature["algorithm"])
	
	experimentalFeatures, ok := result["experimentalFeatures"].([]interface{})
	require.True(t, ok)
	assert.Len(t, experimentalFeatures, 2)
	assert.Equal(t, "feature1", experimentalFeatures[0])
	assert.Equal(t, "feature2", experimentalFeatures[1])

	// Step 5: Verify that a newer client can still load the modified metadata
	var reloadedMetadata RootMetadata
	err = json.Unmarshal(modifiedJSON, &reloadedMetadata)
	require.NoError(t, err)

	// The newer client should see both the modification and the preserved fields
	assert.Equal(t, "2026-01-01T00:00:00Z", reloadedMetadata.Expires)
	assert.Len(t, reloadedMetadata.UnrecognizedFields, 3)
}

// TestIssue751_TargetsMetadataScenario tests the same scenario for targets metadata
func TestIssue751_TargetsMetadataScenario(t *testing.T) {
	// Simulate targets metadata from a newer version
	newerVersionJSON := `{
		"type": "targets",
		"schemaVersion": "https://gittuf.dev/policy/targets/v0.2",
		"expires": "2025-01-01T00:00:00Z",
		"targets": {},
		"delegations": {
			"principals": {},
			"roles": [],
			"newDelegationFeature": "enhanced-verification"
		},
		"advancedTargetValidation": {
			"checksumAlgorithms": ["sha256", "sha3-256"],
			"requireSignedTargets": true
		}
	}`

	// Older client loads and modifies the metadata
	var targetsMetadata TargetsMetadata
	err := json.Unmarshal([]byte(newerVersionJSON), &targetsMetadata)
	require.NoError(t, err)

	// Verify unrecognized fields are preserved at both levels
	assert.Len(t, targetsMetadata.UnrecognizedFields, 1)
	assert.Contains(t, targetsMetadata.UnrecognizedFields, "advancedTargetValidation")
	
	assert.Len(t, targetsMetadata.Delegations.UnrecognizedFields, 1)
	assert.Contains(t, targetsMetadata.Delegations.UnrecognizedFields, "newDelegationFeature")

	// Modify a known field
	targetsMetadata.Expires = "2026-01-01T00:00:00Z"

	// Save and verify preservation
	modifiedJSON, err := json.Marshal(&targetsMetadata)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(modifiedJSON, &result)
	require.NoError(t, err)

	// Check modification and preservation
	assert.Equal(t, "2026-01-01T00:00:00Z", result["expires"])
	assert.Contains(t, result, "advancedTargetValidation")
	
	delegations, ok := result["delegations"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "enhanced-verification", delegations["newDelegationFeature"])
}

// TestIssue751_MultipleModificationCycles tests that fields survive multiple modification cycles
func TestIssue751_MultipleModificationCycles(t *testing.T) {
	originalJSON := `{
		"type": "root",
		"schemaVersion": "https://gittuf.dev/policy/root/v0.2",
		"expires": "2025-01-01T00:00:00Z",
		"principals": {},
		"roles": {},
		"criticalFutureField": "must-not-be-lost",
		"versionInfo": {
			"clientVersion": "v0.15.0",
			"features": ["quantum-resistant", "zero-knowledge-proofs"]
		}
	}`

	// Cycle 1: Load, modify, save
	var metadata1 RootMetadata
	err := json.Unmarshal([]byte(originalJSON), &metadata1)
	require.NoError(t, err)
	
	metadata1.Expires = "2026-01-01T00:00:00Z"
	
	json1, err := json.Marshal(&metadata1)
	require.NoError(t, err)

	// Cycle 2: Load, modify, save again
	var metadata2 RootMetadata
	err = json.Unmarshal(json1, &metadata2)
	require.NoError(t, err)
	
	metadata2.Expires = "2027-01-01T00:00:00Z"
	
	json2, err := json.Marshal(&metadata2)
	require.NoError(t, err)

	// Cycle 3: Load, modify, save one more time
	var metadata3 RootMetadata
	err = json.Unmarshal(json2, &metadata3)
	require.NoError(t, err)
	
	metadata3.Expires = "2028-01-01T00:00:00Z"
	
	finalJSON, err := json.Marshal(&metadata3)
	require.NoError(t, err)

	// Verify that after multiple cycles, unrecognized fields are still preserved
	var finalResult map[string]interface{}
	err = json.Unmarshal(finalJSON, &finalResult)
	require.NoError(t, err)

	assert.Equal(t, "2028-01-01T00:00:00Z", finalResult["expires"])
	assert.Equal(t, "must-not-be-lost", finalResult["criticalFutureField"])
	
	versionInfo, ok := finalResult["versionInfo"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "v0.15.0", versionInfo["clientVersion"])
	
	features, ok := versionInfo["features"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, features, "quantum-resistant")
	assert.Contains(t, features, "zero-knowledge-proofs")
}