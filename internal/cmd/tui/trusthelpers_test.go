// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/tuf"
)

func TestGetGlobalRulesNonExistentRepo(t *testing.T) {
	o := &options{
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: ""},
	}

	rules := getGlobalRules(context.Background(), o)
	if len(rules) != 0 {
		t.Errorf("expected 0 global rules for non-existent repo, got %d", len(rules))
	}
}

func TestRepoAddGlobalRuleNoRepoError(t *testing.T) {
	o := &options{
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: ""},
	}

	ctx := context.Background()

	grThreshold := globalRule{
		ruleName:     "my-threshold",
		ruleType:     tuf.GlobalRuleThresholdType,
		rulePatterns: []string{"refs/heads/*"},
		threshold:    1,
	}

	err := repoAddGlobalRule(ctx, o, grThreshold)
	if err == nil {
		t.Error("expected error when adding global rule without repo, got nil")
	}
}

func TestRepoUpdateGlobalRuleNoRepoError(t *testing.T) {
	o := &options{
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: ""},
	}

	ctx := context.Background()

	grBlock := globalRule{
		ruleName:     "my-block",
		ruleType:     tuf.GlobalRuleBlockForcePushesType,
		rulePatterns: []string{"refs/heads/main"},
	}

	err := repoUpdateGlobalRule(ctx, o, grBlock)
	if err == nil {
		t.Error("expected error when updating global rule without repo, got nil")
	}
}

func TestRepoRemoveGlobalRuleNoRepoError(t *testing.T) {
	o := &options{
		targetRef: "policy",
		p:         &persistent.Options{SigningKey: ""},
	}

	gr := globalRule{
		ruleName: "test-rule",
	}

	err := repoRemoveGlobalRule(context.Background(), o, gr)
	if err == nil {
		t.Error("expected error when removing global rule without repo, got nil")
	}
}
