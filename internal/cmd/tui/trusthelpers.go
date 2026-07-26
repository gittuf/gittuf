// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/tuf"
)

type globalRule struct {
	ruleName     string
	ruleType     string
	rulePatterns []string
	threshold    int
}

type trustHook struct {
	stage        tuf.HookStage
	name         string
	principalIDs []string
	hashes       map[string]string
	environment  tuf.HookEnvironment
	timeout      int
}

type trustPropagationDirective struct {
	name                string
	upstreamRepository  string
	upstreamReference   string
	upstreamPath        string
	downstreamReference string
	downstreamPath      string
}

// getGlobalRules returns a slice of globalRule for the TUI
func getGlobalRules(ctx context.Context, o *options) []globalRule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListGlobalRules(ctx, o.targetRef)
	if err != nil {
		return nil
	}

	var currRules = make([]globalRule, len(rules))
	for i, r := range rules {
		switch gRule := r.(type) {
		case tuf.GlobalRuleThreshold:
			currRules[i] = globalRule{
				ruleName:     gRule.GetName(),
				ruleType:     tuf.GlobalRuleThresholdType,
				rulePatterns: gRule.GetProtectedNamespaces(),
				threshold:    gRule.GetThreshold(),
			}
		case tuf.GlobalRuleBlockForcePushes:
			currRules[i] = globalRule{
				ruleName:     gRule.GetName(),
				ruleType:     tuf.GlobalRuleBlockForcePushesType,
				rulePatterns: gRule.GetProtectedNamespaces(),
			}
		}
	}
	return currRules
}

// repoAddGlobalRule adds a global rule
func repoAddGlobalRule(ctx context.Context, o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	switch gr.ruleType {
	case tuf.GlobalRuleThresholdType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleThresholdType)
		}
		return repo.AddGlobalRuleThreshold(
			ctx, signer,
			gr.ruleName, gr.rulePatterns,
			gr.threshold, true, opts...,
		)
	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}
		return repo.AddGlobalRuleBlockForcePushes(
			ctx, signer,
			gr.ruleName, gr.rulePatterns,
			true, opts...,
		)
	default:
		return fmt.Errorf("unknown global rule type %q", gr.ruleType)
	}
}

// repoRemoveGlobalRule removes a global rule
func repoRemoveGlobalRule(ctx context.Context, o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.RemoveGlobalRule(
		ctx, signer, gr.ruleName, true, opts...,
	)
}

// repoUpdateGlobalRule updates a global rule
func repoUpdateGlobalRule(ctx context.Context, o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	switch gr.ruleType {
	case tuf.GlobalRuleThresholdType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleThresholdType)
		}

		return repo.UpdateGlobalRuleThreshold(ctx, signer, gr.ruleName, gr.rulePatterns, gr.threshold, true, opts...)

	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}

		return repo.UpdateGlobalRuleBlockForcePushes(ctx, signer, gr.ruleName, gr.rulePatterns, true, opts...)

	default:
		return tuf.ErrUnknownGlobalRuleType
	}
}

// repoAddRootKey take the TUI input and adds a root key to the repository
func repoAddRootKey(ctx context.Context, o *options, keyInput string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	newRootKey, err := gittuf.LoadPublicKey(keyInput)
	if err != nil {
		return err
	}

	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddRootKey(ctx, signer, newRootKey, true, opts...)
}

// repoRemoveRootKey take the TUI input and removes a root key from the repository
func repoRemoveRootKey(ctx context.Context, o *options, keyInput string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.RemoveRootKey(ctx, signer, strings.ToLower(keyInput), true, opts...)
}

// repoAddPolicyKey takes the TUI input and adds a policy key to the repository
func repoAddPolicyKey(ctx context.Context, o *options, keyInput string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	targetsKey, err := gittuf.LoadPublicKey(keyInput)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddTopLevelTargetsKey(ctx, signer, targetsKey, true, opts...)
}

// repoRemovePolicyKey takes the TUI input and removes a policy key from the repository
func repoRemovePolicyKey(ctx context.Context, o *options, keyInput string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.RemoveTopLevelTargetsKey(ctx, signer, strings.ToLower(keyInput), true, opts...)
}

func repoUpdateRootThreshold(ctx context.Context, o *options, thresholdInput int) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.UpdateRootThreshold(ctx, signer, thresholdInput, true, opts...)
}

func repoUpdatePolicyThreshold(ctx context.Context, o *options, thresholdInput int) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.UpdateTopLevelTargetsThreshold(ctx, signer, thresholdInput, true, opts...)
}

// repoStageTrustChanges stages trust changes to the repository
func repoStageTrustChanges(ctx context.Context, o *options) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.StagePolicy(ctx, "", false, true)
}

// repoSignTrustChanges signs trust changes to the repository
func repoSignTrustChanges(ctx context.Context, o *options) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.SignRoot(ctx, signer, true, opts...)
}

// repoApplyTrustChanges applies trust changes to the repository
func repoApplyTrustChanges(ctx context.Context, o *options) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.ApplyPolicy(ctx, "", false, true)
}

// renderHooks turns []trustHook into screen text
func (s *trustHookScreen) renderHooks() string {
	if len(s.hooks) == 0 {
		return "No hooks configured"
	}

	var b strings.Builder
	for _, hook := range s.hooks {
		b.WriteString("Stage: " + hook.stage.String() + "\n")
		b.WriteString("Hook: " + hook.name + "\n")
		b.WriteString("Environment: " + hook.environment.String() + "\n")
		b.WriteString("Principals: " + strings.Join(hook.principalIDs, ", ") + "\n")
		b.WriteString(fmt.Sprintf("Timeout: %d\n\n", hook.timeout))
	}

	return b.String()
}

// repoListHooks lists hooks in the repository
func repoListHooks(ctx context.Context, o *options) ([]trustHook, error) {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil, err
	}

	hookStages, err := repo.ListHooks(ctx, o.targetRef)
	if err != nil {
		return nil, err
	}

	var trustHooks []trustHook
	for stage, data := range hookStages {
		for _, hook := range data {
			trustHooks = append(trustHooks, trustHook{
				stage:        stage,
				name:         hook.ID(),
				principalIDs: hook.GetPrincipalIDs().Contents(),
				hashes:       hook.GetHashes(),
				environment:  hook.GetEnvironment(),
				timeout:      hook.GetTimeout(),
			})
		}
	}

	return trustHooks, nil
}

// repoAddHook adds a hook to the repository
func repoAddHook(ctx context.Context, o *options, hookName string, isPreCommit, isPrePush bool, timeout int, env string, principalIDs []string) error {
	var environment tuf.HookEnvironment
	switch strings.ToLower(env) {
	case tuf.HookEnvironmentLuaString:
		environment = tuf.HookEnvironmentLua
	default:
		return tuf.ErrInvalidHookEnvironment
	}

	stages := []tuf.HookStage{}
	if isPreCommit {
		stages = append(stages, tuf.HookStagePreCommit)
	}

	if isPrePush {
		stages = append(stages, tuf.HookStagePrePush)
	}

	if timeout < 1 {
		return gittuf.ErrInvalidHookTimeout
	}

	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	hookBytes := []byte{}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddHook(ctx, signer, stages, hookName, hookBytes, environment, principalIDs, timeout, true, opts...)
}

// repoUpdateHook updates a hook in the repository
func repoUpdateHook(ctx context.Context, o *options, hookName string, isPreCommit, isPrePush bool, timeout int, env string, principalIDs []string) error {
	var environment tuf.HookEnvironment
	switch strings.ToLower(env) {
	case tuf.HookEnvironmentLuaString:
		environment = tuf.HookEnvironmentLua
	default:
		return tuf.ErrInvalidHookEnvironment
	}

	stages := []tuf.HookStage{}
	if isPreCommit {
		stages = append(stages, tuf.HookStagePreCommit)
	}

	if isPrePush {
		stages = append(stages, tuf.HookStagePrePush)
	}

	if timeout < 1 {
		return gittuf.ErrInvalidHookTimeout
	}

	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	hookBytes := []byte{}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.UpdateHook(ctx, signer, stages, hookName, hookBytes, environment, principalIDs, timeout, true, opts...)
}

// repoRemoveHook removes a hook from the repository
func repoRemoveHook(ctx context.Context, o *options, hookName string, isPreCommit, isPrePush bool) error {
	stages := []tuf.HookStage{}
	if isPreCommit {
		stages = append(stages, tuf.HookStagePreCommit)
	}

	if isPrePush {
		stages = append(stages, tuf.HookStagePrePush)
	}

	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.RemoveHook(ctx, signer, stages, hookName, true, opts...)
}

// renderForm renders the add/update hook form for the TUI
func (s *trustHookScreen) renderHookForm() string {
	stage := "pre-commit"
	if s.isPrePush {
		stage = "pre-push"
	}

	return strings.Join([]string{
		"Add/Update a trust hook",
		"",
		s.inputs[0].View(),
		s.inputs[1].View(),
		s.inputs[2].View(),
		s.inputs[3].View(),
		"",
		"Stage: " + stage,
		"Use arrow keys or tab to switch stage.",
		"Press Enter to submit.",
	}, "\n")
}

// renderRemoveHookForm renders the remove hook form for the TUI
func (s *trustHookScreen) renderRemoveHookForm() string {
	stage := "pre-commit"
	if s.isPrePush {
		stage = "pre-push"
	}

	return strings.Join([]string{
		"Remove a trust hook",
		"",
		s.inputs[0].View(),
		"",
		"Stage: " + stage,
		"Use arrow keys or tab to switch stage.",
		"Press Enter to submit.",
	}, "\n")
}

// handleHookFormSubmit handles the submission of the add/update hook form
func (s *trustHookScreen) handleHookFormSubmit(m *model) (tea.Model, tea.Cmd) {
	hookName := strings.TrimSpace(s.inputs[0].Value())
	environment := strings.TrimSpace(s.inputs[1].Value())
	principalIDs := splitAndTrim(s.inputs[2].Value())
	timeout, err := strconv.Atoi(strings.TrimSpace(s.inputs[3].Value()))
	if err != nil || timeout <= 0 {
		m.errorMsg = "Timeout must be a positive integer"
		return *m, nil
	}

	switch s.selectedAction {
	case trustAddHookAction:
		err = repoAddHook(m.ctx, m.options, hookName, s.isPreCommit, s.isPrePush, timeout, environment, principalIDs)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error adding hook: %v", err)
			return *m, nil
		}
		m.footer = "Hook added!"
	case trustUpdateHookAction:
		err = repoUpdateHook(m.ctx, m.options, hookName, s.isPreCommit, s.isPrePush, timeout, environment, principalIDs)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error adding hook: %v", err)
			return *m, nil
		}
		m.footer = "Hook updated!"
	}

	m.errorMsg = ""
	s.inputs = nil
	s.isPreCommit = false
	s.isPrePush = false
	s.hooks = nil
	s.showHooks = false
	s.selectedAction = trustHookActionNone
	m.screen = screenTrustHooks
	return *m, nil
}

// handleRemoveHookSubmit handles the submission of the remove hook form
func (s *trustHookScreen) handleRemoveHookSubmit(m *model) (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(s.inputs[0].Value())
	if value == "" {
		m.errorMsg = "Hook name is required"
		return *m, nil
	}

	err := repoRemoveHook(m.ctx, m.options, value, s.isPreCommit, s.isPrePush)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Error removing hook: %v", err)
		return *m, nil
	}

	m.errorMsg = ""
	m.footer = "Hook removed!"
	s.inputs = nil
	s.isPreCommit = false
	s.isPrePush = false
	s.hooks = nil
	s.showHooks = false
	s.selectedAction = trustHookActionNone
	m.screen = screenTrustHooks
	return *m, nil
}

// repoListPropagationDirectives returns propagation directives for the TUI.
func repoListPropagationDirectives(ctx context.Context, o *options) ([]trustPropagationDirective, error) {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil, err
	}

	directives, err := repo.ListPropagationDirectives(ctx, o.targetRef)
	if err != nil {
		return nil, err
	}

	var trustDirectives []trustPropagationDirective
	for _, pd := range directives {
		trustDirectives = append(trustDirectives, trustPropagationDirective{
			name:                pd.GetName(),
			upstreamRepository:  pd.GetUpstreamRepository(),
			upstreamReference:   pd.GetUpstreamReference(),
			upstreamPath:        pd.GetUpstreamPath(),
			downstreamReference: pd.GetDownstreamReference(),
			downstreamPath:      pd.GetDownstreamPath(),
		})
	}

	return trustDirectives, nil
}

// repoAddPropagationDirective adds a propagation directive.
func repoAddPropagationDirective(ctx context.Context, o *options, name, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddPropagationDirective(ctx, signer, name, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath, true, opts...)
}

// repoUpdatePropagationDirective updates a propagation directive.
func repoUpdatePropagationDirective(ctx context.Context, o *options, name, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.UpdatePropagationDirective(ctx, signer, name, upstreamRepository, upstreamReference, upstreamPath, downstreamReference, downstreamPath, true)
}

// repoRemovePropagationDirective removes a propagation directive.
func repoRemovePropagationDirective(ctx context.Context, o *options, name string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.RemovePropagationDirective(ctx, signer, name, true, opts...)
}

// repoAddGitHubApp adds a GitHub App to the root of trust.
func repoAddGitHubApp(ctx context.Context, o *options, appName, appKeyInput string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	appKey, err := gittuf.LoadPublicKey(appKeyInput)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddGitHubApp(ctx, signer, appName, appKey, true, opts...)
}

// repoRemoveGitHubApp removes a GitHub App from the root of trust.
func repoRemoveGitHubApp(ctx context.Context, o *options, appName string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.RemoveGitHubApp(ctx, signer, appName, true, opts...)
}

// repoTrustGitHubApp marks GitHub App approvals as trusted.
func repoTrustGitHubApp(ctx context.Context, o *options, appName string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.TrustGitHubApp(ctx, signer, appName, true, opts...)
}

// repoUntrustGitHubApp marks GitHub App approvals as untrusted.
func repoUntrustGitHubApp(ctx context.Context, o *options, appName string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.UntrustGitHubApp(ctx, signer, appName, true, opts...)
}

// repoAddControllerRepository adds a controller repository.
func repoAddControllerRepository(ctx context.Context, o *options, repositoryName, repositoryLocation string, principalRefs []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	initialRootPrincipals := make([]tuf.Principal, 0, len(principalRefs))
	for _, principalRef := range principalRefs {
		principal, err := gittuf.LoadPublicKey(principalRef)
		if err != nil {
			return err
		}
		initialRootPrincipals = append(initialRootPrincipals, principal)
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddControllerRepository(ctx, signer, repositoryName, repositoryLocation, initialRootPrincipals, true, opts...)
}

// repoAddNetworkRepository adds a network repository.
func repoAddNetworkRepository(ctx context.Context, o *options, repositoryName, repositoryLocation string, principalRefs []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	initialRootPrincipals := make([]tuf.Principal, 0, len(principalRefs))
	for _, principalRef := range principalRefs {
		principal, err := gittuf.LoadPublicKey(principalRef)
		if err != nil {
			return err
		}
		initialRootPrincipals = append(initialRootPrincipals, principal)
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddNetworkRepository(ctx, signer, repositoryName, repositoryLocation, initialRootPrincipals, true, opts...)
}

// repoSetRepositoryLocation sets the canonical repository location.
func repoSetRepositoryLocation(ctx context.Context, o *options, location string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.SetRepositoryLocation(ctx, signer, location, true, opts...)
}

// repoMakeController marks the current repository as a controller.
func repoMakeController(ctx context.Context, o *options) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.EnableController(ctx, signer, true, opts...)
}

// renderDirectives turns propagation directives into screen text.
func (s *trustPropagationScreen) renderDirectives() string {
	if len(s.directives) == 0 {
		return "No propagation directives configured."
	}

	var b strings.Builder
	for _, directive := range s.directives {
		b.WriteString("Directive: " + directive.name + "\n")
		b.WriteString("Upstream Repository: " + directive.upstreamRepository + "\n")
		b.WriteString("Upstream Reference: " + directive.upstreamReference + "\n")
		b.WriteString("Upstream Path: " + directive.upstreamPath + "\n")
		b.WriteString("Downstream Reference: " + directive.downstreamReference + "\n")
		b.WriteString("Downstream Path: " + directive.downstreamPath + "\n\n")
	}

	return strings.TrimSpace(b.String())
}
