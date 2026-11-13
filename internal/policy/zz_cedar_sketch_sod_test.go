// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

// SKETCH EXPERIMENT (cedar-integration-sketch, throwaway, not for merge).
// How many PLAIN gittuf rules (no Cedar) are needed to express separation of
// duties on git:refs/heads/main: "at least one approver from EACH of 3 groups,
// each group has 5 principals"? Brute force over every threshold rule, an
// exact set-cover bound, then a check against the real verifier. Uses the
// helpers in zz_cedar_sketch_gap_test.go.

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/require"
)

// sodRuleCount is the brute-force result for one access property over n
// principals (bit i = principal i).
type sodRuleCount struct {
	soundRules      int // (S, t) with every satisfying signer set satisfying prop
	minimalSets     int // minimal signer sets satisfying prop
	usefulRules     int // sound rules covering at least one minimal set
	maxCover        int // max minimal sets covered by a single sound rule
	uncovered       int // minimal sets no sound rule covers (must be 0)
	minRules        int // exact minimum number of rules
	lowerBoundTight bool
}

// sodBruteForce enumerates all threshold rules (S, t), 1 <= t <= |S|, over n
// principals. A gittuf rule set accepts T iff some rule has |T & S| >= t, so
// the accepted family is the upward closure of the rules' t-subsets. A rule
// is SOUND iff every t-subset of S satisfies prop (prop is monotone, so
// supersets do too). A rule set expresses prop exactly iff every rule is
// sound and every minimal satisfying set M is accepted by some rule, i.e.
// |M & S| >= t. The minimum rule count is the minimum set cover of the
// minimal sets by sound rules.
func sodBruteForce(n int, prop func(int) bool) sodRuleCount {
	full := 1<<n - 1

	// okAll[S] bit k set => every size-k submask of S satisfies prop.
	res := sodRuleCount{}
	soundRules := [][2]int{}
	for s := 1; s <= full; s++ {
		bad := 0 // bit k set => some size-k submask fails prop
		for sub := s; ; sub = (sub - 1) & s {
			if !prop(sub) {
				bad |= 1 << popcount(sub)
			}
			if sub == 0 {
				break
			}
		}
		for t := 1; t <= popcount(s); t++ {
			if bad&(1<<t) == 0 {
				soundRules = append(soundRules, [2]int{s, t})
			}
		}
	}
	res.soundRules = len(soundRules)

	minimal := []int{}
	for m := 0; m <= full; m++ {
		if !prop(m) {
			continue
		}
		isMin := true
		for b := 0; b < n; b++ {
			if m&(1<<b) != 0 && prop(m&^(1<<b)) {
				isMin = false
				break
			}
		}
		if isMin {
			minimal = append(minimal, m)
		}
	}
	res.minimalSets = len(minimal)

	coveredBy := make([]int, len(minimal))
	for _, r := range soundRules {
		c := 0
		for i, m := range minimal {
			if popcount(m&r[0]) >= r[1] {
				c++
				coveredBy[i]++
			}
		}
		if c > 0 {
			res.usefulRules++
		}
		if c > res.maxCover {
			res.maxCover = c
		}
	}
	for _, c := range coveredBy {
		if c == 0 {
			res.uncovered++
		}
	}

	// If no sound rule covers more than one minimal set, any valid rule set
	// needs >= |minimal| rules, and one rule per minimal set (S = M,
	// t = |M|, sound because prop(M) and M is its only |M|-subset) achieves
	// it. So the exact minimum is |minimal|.
	if res.maxCover <= 1 && res.uncovered == 0 {
		res.minRules = len(minimal)
		res.lowerBoundTight = true
	} else {
		res.minRules = -1
	}
	return res
}

// sodProp returns the separation-of-duties property for g groups of m
// principals: group i owns bits [i*m, (i+1)*m).
func sodProp(g, m int) func(int) bool {
	masks := make([]int, g)
	for i := 0; i < g; i++ {
		masks[i] = (1<<m - 1) << (i * m)
	}
	return func(x int) bool {
		for _, gm := range masks {
			if x&gm == 0 {
				return false
			}
		}
		return true
	}
}

func intPow(b, e int) int {
	r := 1
	for ; e > 0; e-- {
		r *= b
	}
	return r
}

func TestCedarSketchSoDRuleCount(t *testing.T) {
	main := "git:refs/heads/main"
	groups := []string{"A", "B", "C"}
	names := []string{}
	for _, g := range groups {
		for i := 1; i <= 5; i++ {
			names = append(names, fmt.Sprintf("%s%d", g, i))
		}
	}

	t.Run("brute force minimum rule count", func(t *testing.T) {
		shapes := [][2]int{{2, 2}, {2, 3}, {3, 2}, {2, 4}, {3, 3}, {4, 2}, {3, 5}}
		for _, sh := range shapes {
			g, m := sh[0], sh[1]
			r := sodBruteForce(g*m, sodProp(g, m))
			t.Logf("SoD g=%d groups x m=%d: principals=%d sound rules=%d (covering >=1 minimal set: %d) minimal sets=%d max minimal sets per sound rule=%d uncovered=%d => exact minimum rules=%d (m^g=%d)",
				g, m, g*m, r.soundRules, r.usefulRules, r.minimalSets, r.maxCover, r.uncovered, r.minRules, intPow(m, g))
			require.True(t, r.lowerBoundTight)
			require.Equal(t, intPow(m, g), r.minRules)
			// The only sound rules that accept anything minimal are the
			// m^g transversals with t = g; other sound rules (e.g.
			// {A1,A2,B1} t=3 for g=2) accept only non-minimal sets.
			require.Equal(t, intPow(m, g), r.usefulRules)
		}

		// Cross-check against milestone 2: R plus at least 3 of {R,a,b,c,d}.
		r := sodBruteForce(5, func(x int) bool { return x&1 == 1 && popcount(x) >= 3 })
		t.Logf("cross-check 3-of-{R,a,b,c,d} with R required: sound rules=%d (covering >=1: %d) minimal sets=%d max cover=%d => exact minimum rules=%d",
			r.soundRules, r.usefulRules, r.minimalSets, r.maxCover, r.minRules)
		require.Equal(t, 6, r.minRules)

		// The tempting single rule 3-of-15 is unsound: {A1,A2,B1} meets it.
		p := sodProp(3, 5)
		require.False(t, p(0b00000_00001_00011))
	})

	ps := sketchPrincipals(t, names...)

	sodRules := []sketchRule{}
	for _, a := range names[0:5] {
		for _, b := range names[5:10] {
			for _, c := range names[10:15] {
				sodRules = append(sodRules, sketchRule{fmt.Sprintf("sod-%s-%s-%s", a, b, c), []string{a, b, c}, []string{main}, 3, false})
			}
		}
	}
	require.Len(t, sodRules, 125)
	groupOf := func(n string) byte { return n[0] }

	t.Run("real gittuf: 125 rules, all 125 transversal triples + sampled other triples", func(t *testing.T) {
		state := sketchState(t, ps, sodRules, nil, nil)
		allowed, denied, idx := 0, 0, 0
		for i := 0; i < 15; i++ {
			for j := i + 1; j < 15; j++ {
				for k := j + 1; k < 15; k++ {
					a, b, c := names[i], names[j], names[k]
					want := groupOf(a) != groupOf(b) && groupOf(b) != groupOf(c) && groupOf(a) != groupOf(c)
					if !want {
						// Each DENY walks all 125 verifiers; sample every
						// 11th of the 330 non-transversal triples to keep
						// runtime down.
						idx++
						if idx%11 != 1 {
							continue
						}
					}
					v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, a, b, c), nil)
					if want {
						require.Nil(t, err, "%s,%s,%s", a, b, c)
						require.Equal(t, fmt.Sprintf("sod-%s-%s-%s", a, b, c), v)
						allowed++
					} else {
						require.ErrorIs(t, err, ErrVerifierConditionsUnmet, "%s,%s,%s", a, b, c)
						denied++
					}
				}
			}
		}
		t.Logf("signer triples: ALLOW=%d (all 125 one-per-group triples) DENY=%d (sampled of the 330 others)", allowed, denied)
		require.Equal(t, 125, allowed)
		require.Equal(t, 30, denied)

		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "B1", "C1"), nil)
		t.Logf("A1,B1,C1: %s", verdict(v, err))
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "A2", "B1"), nil)
		t.Logf("A1,A2,B1: %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, names[0:10]...), nil)
		t.Logf("all of A and B (10 signers, no C): %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "A2", "A3", "B4", "C5"), nil)
		t.Logf("A2,A3,B4,C5: %s", verdict(v, err))
		require.Nil(t, err)
	})

	t.Run("gap: single 3-of-15 rule", func(t *testing.T) {
		state := sketchState(t, ps, []sketchRule{{"protect-main", names, []string{main}, 3, false}}, nil, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "A2", "B1"), nil)
		t.Logf("3-of-15, A1,A2,B1: %s", verdict(v, err))
		require.Nil(t, err)
	})

	t.Run("delegation only adds alternatives (OR)", func(t *testing.T) {
		// Try to build "A AND B" as a parent rule gating a delegated file.
		state := sketchState(t, ps, []sketchRule{
			{"need-A", names[0:5], []string{main}, 1, false},
		}, map[string][]sketchRule{
			"need-A": {{"need-B", names[5:10], []string{main}, 1, false}},
		}, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1"), nil)
		t.Logf("parent need-A, child need-B; A1 only: %s", verdict(v, err))
		require.Nil(t, err)
		require.Equal(t, "need-A", v)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "B1"), nil)
		t.Logf("parent need-A, child need-B; B1 only: %s", verdict(v, err))
		require.Nil(t, err)
		require.Equal(t, "need-B", v)

		// Same with the parent terminating: still OR.
		state = sketchState(t, ps, []sketchRule{
			{"need-A", names[0:5], []string{main}, 1, true},
		}, map[string][]sketchRule{
			"need-A": {{"need-B", names[5:10], []string{main}, 1, false}},
		}, nil)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "B1"), nil)
		t.Logf("terminating parent need-A, child need-B; B1 only: %s", verdict(v, err))
		require.Nil(t, err)

		// A loose parent can't be tightened by the 125 rules below it.
		state = sketchState(t, ps, []sketchRule{
			{"loose", names, []string{main}, 3, false},
		}, map[string][]sketchRule{"loose": sodRules}, nil)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "A2", "B1"), nil)
		t.Logf("parent 3-of-15 delegating to the 125 rules; A1,A2,B1: %s", verdict(v, err))
		require.Nil(t, err)
		require.Equal(t, "loose", v)
	})

	t.Run("global threshold rules", func(t *testing.T) {
		// 125 SoD rules plus a global threshold of 3 on main.
		state := sketchState(t, ps, sodRules, nil, []tuf.GlobalRule{tufv01.NewGlobalRuleThreshold("three-main", []string{main}, 3)})
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "A2", "B1"), nil)
		t.Logf("125 rules + global threshold 3; A1,A2,B1: %s", verdict(v, err))
		obsGlobal3 := err == nil
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "B1", "C1"), nil)
		t.Logf("125 rules + global threshold 3; A1,B1,C1: %s", verdict(v, err))
		require.Nil(t, err)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "A2"), nil)
		t.Logf("125 rules + global threshold 3; A1,A2: %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)

		// Global rule on a different namespace only.
		state = sketchState(t, ps, sodRules, nil, []tuf.GlobalRule{tufv01.NewGlobalRuleThreshold("one-dev", []string{"git:refs/heads/dev"}, 1)})
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "A2", "B1"), nil)
		t.Logf("125 rules on main + global threshold 1 on dev only; A1,A2,B1 on main: %s", verdict(v, err))
		obsOtherNS := err == nil
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps), nil)
		t.Logf("125 rules on main + global threshold 1 on dev only; NO signers on main: %s", verdict(v, err))
		obsNoSigners := err == nil

		// Global threshold 11 alone forces all three groups (any two groups
		// hold only 10 principals) but also forces 11 signers.
		state = sketchState(t, ps, []sketchRule{{"any-main", names, []string{main}, 1, false}}, nil,
			[]tuf.GlobalRule{tufv01.NewGlobalRuleThreshold("eleven-main", []string{main}, 11)})
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, names[0:10]...), nil)
		t.Logf("1-of-15 rule + global threshold 11; all of A and B (10): %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, names[0:11]...), nil)
		t.Logf("1-of-15 rule + global threshold 11; A1-5,B1-5,C1 (11): %s", verdict(v, err))
		require.Nil(t, err)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "A1", "B1", "C1"), nil)
		t.Logf("1-of-15 rule + global threshold 11; A1,B1,C1: %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)

		t.Logf("observed: global3 lets A1,A2,B1 through=%v; unrelated global rule lets A1,A2,B1 through=%v; unrelated global rule lets zero signers through=%v", obsGlobal3, obsOtherNS, obsNoSigners)
		// Pin observed behavior at 324bb132: global rules are checked in
		// addition to the branch rules (getVerifierForGlobalRules,
		// policy.go:449-487), so they never let a non-SoD set through and
		// cannot reduce the rule count. At 16047b70 (before upstream 0b332fcc
		// "policy: Fix prioritization of global rule's exhaustive verifier")
		// the exhaustive verifier was tried first and both of these were true.
		require.False(t, obsGlobal3)
		require.False(t, obsOtherNS)
		require.False(t, obsNoSigners)
	})
}
