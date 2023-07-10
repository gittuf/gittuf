package policy

import (
	"errors"
	"fmt"
)

type RuleType uint

const (
	RuleGitRefOnly RuleType = iota
	RuleFile
)

const (
	RuleGitRefOnlyFormat = "git:%s"
	RuleFileFormat       = "git:%s&file:%s"
)

var (
	ErrFileRuleHasNoGitRefConstraint = errors.New("file specific rule must apply to enumerable Git references")
	ErrDelegationHasMixedRuleTypes   = errors.New("all rules in a delegation must be of the same type")
)

type Rule struct {
	Type          RuleType
	GitRefPattern string
	FilePattern   string
}

// NewGitRefRule creates a Rule object for a Git namespace only rule. The
// pattern must be in absolute reference form such as refs/heads/feature-* or
// refs/<othernamespace>/<pattern>.
func NewGitRefRule(pattern string) *Rule {
	if len(pattern) == 0 {
		pattern = "refs/heads/*" // FIXME: if we expect absolute form, what's happening here?
	}

	return &Rule{Type: RuleGitRefOnly, GitRefPattern: pattern}
}

func NewFileRule(gitRefPattern string, filePattern string) (*Rule, error) {
	if gitRefPattern == "*" || gitRefPattern == "refs/*" || gitRefPattern == "refs/heads/*" || len(gitRefPattern) == 0 {
		return nil, ErrFileRuleHasNoGitRefConstraint
	}

	if len(filePattern) == 0 {
		filePattern = "*"
	}

	return &Rule{
		Type:          RuleFile,
		GitRefPattern: gitRefPattern,
		FilePattern:   filePattern,
	}, nil
}

func (r *Rule) Validate() error {
	if r.Type == RuleFile {
		if r.GitRefPattern == "*" {
			return ErrFileRuleHasNoGitRefConstraint
		}
	}
	return nil
}

func (r *Rule) String() string {
	if r.Type == RuleGitRefOnly {
		return fmt.Sprintf(RuleGitRefOnlyFormat, r.GitRefPattern)
	}

	return fmt.Sprintf(RuleFileFormat, r.GitRefPattern, r.FilePattern)
}

type Delegation struct {
	Rules []*Rule
}

func NewDelegation(rules []*Rule) (*Delegation, error) {
	d := &Delegation{Rules: rules}

	return d, d.Validate()
}

func (d *Delegation) Validate() error {
	if len(d.Rules) == 0 {
		return nil
	}

	t := d.Rules[0].Type
	for _, r := range d.Rules {
		if r.Type != t {
			return ErrDelegationHasMixedRuleTypes
		}
	}

	return nil
}

func (d *Delegation) StringSlice() []string {
	patterns := []string{}
	for _, r := range d.Rules {
		patterns = append(patterns, r.String())
	}

	return patterns
}
