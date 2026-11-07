// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// SKETCH (cedar-integration-sketch, not for merge): milestone 5. The ways
// plain gittuf can keep mallory off refs/heads/main while a broad
// refs/heads/* rule admits her elsewhere.

// mainExcludedPatterns matches every branch except refs/heads/main, using
// fnmatch negated character classes. Settable through the CLI (`policy
// add-rule --rule-pattern`, repeated).
var mainExcludedPatterns = []string{
	"git:refs/heads/[!m]*",
	"git:refs/heads/m[!a]*",
	"git:refs/heads/ma[!i]*",
	"git:refs/heads/mai[!n]*",
	"git:refs/heads/main?*",
	"git:refs/heads/m",
	"git:refs/heads/ma",
	"git:refs/heads/mai",
}

func TestCedarSketchRule5PlainWays(t *testing.T) {
	ps := sketchPrincipals(t, "alice", "bob", "mallory")
	main := "git:refs/heads/main"
	protectMain := sketchRule{"protect-main", []string{"alice", "bob"}, []string{main}, 1, false}

	tests := map[string]struct {
		rules       []sketchRule
		delegations map[string][]sketchRule
		expected    map[string]bool // target -> mallory allowed
	}{
		"broad refs/heads/* rule (naive)": {
			rules: []sketchRule{protectMain, {"protect-branches", []string{"alice", "bob", "mallory"}, []string{"git:refs/heads/*"}, 1, false}},
			expected: map[string]bool{
				main: true, "git:refs/heads/dev": true,
			},
		},
		"CLI-settable: broad rule with main excluded by negated patterns": {
			rules: []sketchRule{protectMain, {"protect-branches", []string{"alice", "bob", "mallory"}, mainExcludedPatterns, 1, false}},
			expected: map[string]bool{
				main: false, "git:refs/heads/dev": true, "git:refs/heads/m": true, "git:refs/heads/maint": true,
				"git:refs/heads/mainline": true, "git:refs/heads/feature/x": true, "git:refs/heads/mai": true,
			},
		},
		"hand-edited metadata: terminating main rule with delegated rule file, ordered first": {
			rules:       []sketchRule{{"protect-main", []string{"alice", "bob"}, []string{main}, 1, true}, {"protect-branches", []string{"alice", "bob", "mallory"}, []string{"git:refs/heads/*"}, 1, false}},
			delegations: map[string][]sketchRule{"protect-main": nil},
			expected:    map[string]bool{main: false, "git:refs/heads/dev": true},
		},
		"hand-edited metadata: same, after `policy reorder-rules` puts the broad rule first": {
			rules:       []sketchRule{{"protect-branches", []string{"alice", "bob", "mallory"}, []string{"git:refs/heads/*"}, 1, false}, {"protect-main", []string{"alice", "bob"}, []string{main}, 1, true}},
			delegations: map[string][]sketchRule{"protect-main": nil},
			expected:    map[string]bool{main: true, "git:refs/heads/dev": true},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			state := sketchState(t, ps, test.rules, test.delegations, nil)
			for target, allowed := range test.expected {
				v, err := sketchDecide(t, state, target, sketchAuthorization(t, ps, "mallory"), nil)
				t.Logf("%s | mallory on %s: %s", name, target, verdict(v, err))
				if allowed {
					require.Nil(t, err, target)
				} else {
					require.ErrorIs(t, err, ErrVerifierConditionsUnmet, target)
				}
			}
			v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "alice"), nil)
			require.Nil(t, err, "alice on main, via %q", v)
		})
	}
}
