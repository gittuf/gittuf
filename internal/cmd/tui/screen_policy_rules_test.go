// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyRulesDeletePrompt(t *testing.T) {
	s := &policyRulesScreen{
		confirmDelete: true,
		deleteTarget:  "test-rule-main",
	}
	m := &model{
		screen: screenPolicyRules,
		width:  80,
		height: 24,
	}

	out := s.View(m)
	assert.Contains(t, out, `Delete rule "test-rule-main"? [y/n]`)
	assert.NotContains(t, out, `Delete principal`)
	assert.NotContains(t, out, `Delete global rule`)
}
