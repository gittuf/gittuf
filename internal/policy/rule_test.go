package policy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGitRefRule(t *testing.T) {
	tests := map[string]struct {
		pattern  string
		expected *Rule
	}{
		"single branch": {
			pattern:  "refs/heads/main",
			expected: &Rule{Type: RuleGitRefOnly, GitRefPattern: "refs/heads/main"},
		},
		"no pattern": {
			pattern:  "",
			expected: &Rule{Type: RuleGitRefOnly, GitRefPattern: "refs/heads/*"},
		},
		"with pattern": {
			pattern:  "refs/heads/feature-*",
			expected: &Rule{Type: RuleGitRefOnly, GitRefPattern: "refs/heads/feature-*"},
		},
	}

	for name, test := range tests {
		rule := NewGitRefRule(test.pattern)
		assert.Equal(t, test.expected, rule, fmt.Sprintf("unexpected Git ref rule generated for test '%s'", name))
	}
}

func TestNewFileRule(t *testing.T) {
	tests := map[string]struct {
		gitPattern  string
		filePattern string
		expected    *Rule
		err         error
	}{
		"single branch with file pattern": {
			gitPattern:  "refs/heads/main",
			filePattern: "foo/*",
			expected: &Rule{
				Type:          RuleFile,
				GitRefPattern: "refs/heads/main",
				FilePattern:   "foo/*",
			},
		},
		"* git pattern": {
			gitPattern:  "*",
			filePattern: "foo/*",
			err:         ErrFileRuleHasNoGitRefConstraint,
		},
		"refs/* git pattern": {
			gitPattern:  "refs/*",
			filePattern: "foo/*",
			err:         ErrFileRuleHasNoGitRefConstraint,
		},
		"refs/heads/* git pattern": {
			gitPattern:  "refs/heads/*",
			filePattern: "foo/*",
			err:         ErrFileRuleHasNoGitRefConstraint,
		},
		"no git pattern": {
			gitPattern:  "",
			filePattern: "foo/*",
			err:         ErrFileRuleHasNoGitRefConstraint,
		},
		"with git pattern and file pattern": {
			gitPattern:  "refs/heads/feature-*",
			filePattern: "foo/*",
			expected: &Rule{
				Type:          RuleFile,
				GitRefPattern: "refs/heads/feature-*",
				FilePattern:   "foo/*",
			},
		},
		"no file pattern": {
			gitPattern:  "refs/heads/main",
			filePattern: "",
			expected: &Rule{
				Type:          RuleFile,
				GitRefPattern: "refs/heads/main",
				FilePattern:   "*",
			},
		},
	}

	for name, test := range tests {
		rule, err := NewFileRule(test.gitPattern, test.filePattern)
		if err != nil {
			assert.ErrorIs(t, err, test.err, fmt.Sprintf("unexpected error in file ref rule test '%s'", name))
		} else {
			assert.Equal(t, test.expected, rule, fmt.Sprintf("unexpected file ref rule generated for test '%s'", name))
		}
	}
}
