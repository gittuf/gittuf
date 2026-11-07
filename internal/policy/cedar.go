// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cedar-policy/cedar-go"
	cedarast "github.com/cedar-policy/cedar-go/x/exp/ast"
	cedarschema "github.com/cedar-policy/cedar-go/x/exp/schema"
	cedarresolved "github.com/cedar-policy/cedar-go/x/exp/schema/resolved"
	cedarvalidate "github.com/cedar-policy/cedar-go/x/exp/schema/validate"
	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/rsl"
)

// SKETCH (cedar-integration-sketch, not for merge): an experimental Cedar
// overlay on gittuf's native decision. The overlay can only restrict: the final
// verdict is the native verdict AND the Cedar verdict. It runs only for git:
// targets, only after the native decision succeeds, and only when
// GITTUF_DEV=1 and GITTUF_DECIDER=cedar. Any Cedar error is a denial.

const (
	// DeciderKey selects an experimental decision layer in developer mode.
	DeciderKey = "GITTUF_DECIDER"
	// DeciderCedar enables the Cedar overlay.
	DeciderCedar = "cedar"

	// CedarPolicyKey is the path to the Cedar policy file.
	// SKETCH: the policy is an unsigned local file, not gittuf metadata.
	CedarPolicyKey = "GITTUF_CEDAR_POLICY"

	// CedarPullRequestAttestationKey is the path to a GitHub pull request
	// attestation statement (in-toto, as created by
	// attestations.NewGitHubPullRequestAttestation).
	// SKETCH: the statement is hand-recorded and unsigned; see the sketch log
	// for how it would be retrieved from refs/gittuf/attestations and trusted.
	CedarPullRequestAttestationKey = "GITTUF_CEDAR_PR_ATTESTATION"

	// cedarDecisionLogKey is the path to a file that receives one line per
	// Cedar decision.
	// SKETCH: evidence for test runs only.
	cedarDecisionLogKey = "GITTUF_CEDAR_LOG"

	// cedarSignersKey selects how credited signers are computed: "reuse"
	// (default) reuses the native decision's accepted principals, "exhaustive"
	// verifies signatures again against every policy principal.
	// SKETCH: a benchmarking knob.
	cedarSignersKey        = "GITTUF_CEDAR_SIGNERS"
	cedarSignersExhaustive = "exhaustive"

	// cedarGroupsCustomKey is the tufv02.Person custom metadata key listing
	// the Cedar groups a person belongs to, comma separated.
	// SKETCH: a convention, not part of the gittuf policy schema.
	cedarGroupsCustomKey = "cedar-groups"
)

var (
	ErrCedarPolicyNotSpecified            = fmt.Errorf("cedar decider is enabled but %s is not set", CedarPolicyKey)
	ErrInvalidCedarPolicy                 = errors.New("invalid cedar policy")
	ErrCedarDenied                        = errors.New("cedar policy denied verification")
	ErrInvalidCedarPullRequestAttestation = errors.New("invalid GitHub pull request attestation for cedar")
)

//go:embed cedar.cedarschema
var cedarSchemaBytes []byte

// nativeDecisionFacts holds facts computed by
// verifyGitObjectAndAttestationsNative that its return values don't carry.
type nativeDecisionFacts struct {
	// acceptedPrincipalIDs are the trusted principals counted toward the rule
	// that verified the target. It is nil if no rule protects the target.
	acceptedPrincipalIDs *set.Set[string]
}

// withNativeFacts sets a sink for facts computed by the native decision.
func withNativeFacts(facts *nativeDecisionFacts) verifyGitObjectAndAttestationsOption {
	return func(o *verifyGitObjectAndAttestationsOptions) {
		o.nativeFacts = facts
	}
}

// cedarExhaustiveVerifier returns a verifier that trusts every policy
// principal and verifies exhaustively.
func cedarExhaustiveVerifier(policy *State) *SignatureVerifier {
	verifier := &SignatureVerifier{
		repository:         policy.repository,
		name:               tuf.ExhaustiveVerifierName,
		principals:         make([]tuf.Principal, 0, len(policy.allPrincipals)),
		threshold:          1,
		verifyExhaustively: true,
	}
	for _, principal := range policy.allPrincipals {
		verifier.principals = append(verifier.principals, principal)
	}
	return verifier
}

// cedarDeciderEnabled returns true when the Cedar overlay is enabled.
func cedarDeciderEnabled() bool {
	return dev.InDevMode() && os.Getenv(DeciderKey) == DeciderCedar
}

// verifyGitObjectAndAttestations decides whether the target's rules and the
// global rules are met. It runs gittuf's native decision and, when the Cedar
// overlay is enabled and the target is a Git reference, additionally requires
// the Cedar policy to allow the change.
func verifyGitObjectAndAttestations(ctx context.Context, policy *State, target string, gitID githash.Hash, authorizationAttestation *sslibdsse.Envelope, opts ...verifyGitObjectAndAttestationsOption) (string, bool, error) {
	if !cedarDeciderEnabled() || !strings.HasPrefix(target, gitReferenceRuleScheme+":") {
		return verifyGitObjectAndAttestationsNative(ctx, policy, target, gitID, authorizationAttestation, opts...)
	}

	facts := &nativeDecisionFacts{}
	// Don't append into the caller's backing array.
	nativeOpts := make([]verifyGitObjectAndAttestationsOption, 0, len(opts)+1)
	nativeOpts = append(nativeOpts, opts...)
	nativeOpts = append(nativeOpts, withNativeFacts(facts))
	verifiedUsing, rslEntrySignatureNeededForThreshold, err := verifyGitObjectAndAttestationsNative(ctx, policy, target, gitID, authorizationAttestation, nativeOpts...)
	if err != nil {
		return "", false, err
	}

	if os.Getenv(cedarSignersKey) == cedarSignersExhaustive {
		// SKETCH: verify signatures a second time against every principal.
		signers, err := cedarExhaustiveVerifier(policy).Verify(ctx, gitID, authorizationAttestation)
		if err != nil {
			return "", false, err
		}
		facts.acceptedPrincipalIDs = signers
	}

	options := &verifyGitObjectAndAttestationsOptions{}
	for _, fn := range opts {
		fn(options)
	}
	if err := verifyCedarOverlay(policy, target, gitID, verifiedUsing, facts, options); err != nil {
		return "", false, err
	}

	return verifiedUsing, rslEntrySignatureNeededForThreshold, nil
}

// verifyCedarOverlay evaluates the Cedar policy for a change gittuf has
// already accepted. It returns nil only if Cedar allows the change without
// any evaluation errors.
func verifyCedarOverlay(policy *State, target string, gitID githash.Hash, verifiedUsing string, facts *nativeDecisionFacts, options *verifyGitObjectAndAttestationsOptions) error {
	policySet, err := loadCedarPolicySet()
	if err != nil {
		logCedarDecision(target, "error", err.Error())
		return err
	}

	request, entities, err := buildCedarRequest(policy, target, gitID, verifiedUsing, facts, options)
	if err != nil {
		logCedarDecision(target, "error", err.Error())
		return err
	}

	decision, diagnostic := cedar.Authorize(policySet, entities, request)
	reasons := cedarDiagnosticSummary(diagnostic)
	// cedar-go skips policies whose evaluation errors, so an erroring forbid
	// would not deny. Treat any evaluation error as a denial.
	if decision != cedar.Allow || len(diagnostic.Errors) != 0 {
		slog.Debug(fmt.Sprintf("Cedar policy denied '%s': %s", target, reasons))
		logCedarDecision(target, "deny", reasons)
		return fmt.Errorf("%w: %w (%s)", ErrVerifierConditionsUnmet, ErrCedarDenied, reasons)
	}

	slog.Debug(fmt.Sprintf("Cedar policy allowed '%s': %s", target, reasons))
	logCedarDecision(target, "allow", reasons)
	return nil
}

// buildCedarRequest builds the Cedar request and entities for a decision.
func buildCedarRequest(policy *State, target string, gitID githash.Hash, verifiedUsing string, facts *nativeDecisionFacts, options *verifyGitObjectAndAttestationsOptions) (cedar.Request, cedar.EntityMap, error) {
	scheme, name, _ := strings.Cut(target, ":")

	action := "updateRef"
	if !options.tagObjectID.IsZero() {
		action = "updateTag"
	}

	approverIdentities := []string{}
	if options.approverPrincipalIDs != nil {
		approverIdentities = options.approverPrincipalIDs.Contents()
	}
	sort.Strings(approverIdentities)
	approvers := principalsForIdentities(policy, approverIdentities)

	// Credited principals: those counted toward the rule that verified the
	// target, plus approvers that rule trusts. The native decision only merges
	// approvals when signatures alone don't meet the threshold.
	credited := set.NewSet[string]()
	if facts.acceptedPrincipalIDs != nil {
		credited.Extend(facts.acceptedPrincipalIDs)
	}
	if verifiedUsing != "" && approvers.Len() != 0 {
		verifiers, err := policy.FindVerifiersForPath(target)
		if err != nil {
			return cedar.Request{}, nil, err
		}
		for _, verifier := range verifiers {
			if verifier.Name() == verifiedUsing {
				credited.Extend(verifier.TrustedPrincipalIDs().Intersection(approvers))
				break
			}
		}
	}

	entities := cedar.EntityMap{}
	addPrincipal := func(principalID string) cedar.EntityUID {
		uid := cedar.NewEntityUID("Gittuf::Principal", cedar.String(principalID))
		if _, has := entities[uid]; has {
			return uid
		}
		parents := []cedar.EntityUID{}
		for _, group := range cedarGroupsForPrincipal(policy, principalID) {
			groupUID := cedar.NewEntityUID("Gittuf::Group", cedar.String(group))
			entities[groupUID] = cedar.Entity{UID: groupUID}
			parents = append(parents, groupUID)
		}
		entities[uid] = cedar.Entity{UID: uid, Parents: cedar.NewEntityUIDSet(parents...)}
		return uid
	}
	principalSet := func(principalIDs []string) cedar.Set {
		values := make([]cedar.Value, 0, len(principalIDs))
		for _, principalID := range principalIDs {
			values = append(values, addPrincipal(principalID))
		}
		return cedar.NewSet(values...)
	}

	creditedIDs := credited.Contents()
	sort.Strings(creditedIDs)
	creditedGroups := set.NewSet[string]()
	for _, principalID := range creditedIDs {
		for _, group := range cedarGroupsForPrincipal(policy, principalID) {
			creditedGroups.Add(group)
		}
	}
	groupIDs := creditedGroups.Contents()
	sort.Strings(groupIDs)
	groupValues := make([]cedar.Value, 0, len(groupIDs))
	for _, group := range groupIDs {
		groupValues = append(groupValues, cedar.NewEntityUID("Gittuf::Group", cedar.String(group)))
	}

	approverIDs := approvers.Contents()
	sort.Strings(approverIDs)
	identityValues := make([]cedar.Value, 0, len(approverIdentities))
	for _, identity := range approverIdentities {
		identityValues = append(identityValues, cedar.String(identity))
	}

	context := cedar.RecordMap{
		"mergeable":          cedar.Boolean(options.verifyMergeable),
		"credited":           principalSet(creditedIDs),
		"creditedGroups":     cedar.NewSet(groupValues...),
		"approvers":          principalSet(approverIDs),
		"approverIdentities": cedar.NewSet(identityValues...),
		"verifiedUsing":      cedar.String(verifiedUsing),
	}

	pullRequest, err := loadCedarPullRequestFacts(policy, gitID)
	if err != nil {
		return cedar.Request{}, nil, err
	}
	if pullRequest != nil {
		record := cedar.RecordMap{
			"number":         cedar.Long(pullRequest.number),
			"authorIdentity": cedar.String(pullRequest.authorIdentity),
			"labels":         cedarStringSet(pullRequest.labels),
		}
		if authors := principalsForIdentities(policy, []string{pullRequest.authorIdentity}).Contents(); len(authors) == 1 {
			record["author"] = addPrincipal(authors[0])
		}
		context["pr"] = cedar.NewRecord(record)
	}

	targetUID := cedar.NewEntityUID("Gittuf::Target", cedar.String(target))
	entities[targetUID] = cedar.Entity{UID: targetUID, Attributes: cedar.NewRecord(cedar.RecordMap{
		"scheme": cedar.String(scheme),
		"name":   cedar.String(name),
	})}

	changeID := "mergeable"
	if !gitID.IsZero() {
		changeID = gitID.String()
	}
	changeUID := cedar.NewEntityUID("Gittuf::Change", cedar.String(changeID))
	entities[changeUID] = cedar.Entity{UID: changeUID}

	return cedar.Request{
		Principal: changeUID,
		Action:    cedar.NewEntityUID("Gittuf::Action", cedar.String(action)),
		Resource:  targetUID,
		Context:   cedar.NewRecord(context),
	}, entities, nil
}

// principalsForIdentities maps code review identities to policy principals
// the same way approvals are matched in
// verifyGitObjectAndAttestationsUsingVerifiers: a tufv02.Person matches if its
// associated identity for a trusted app equals the identity. Unlike that
// function, all policy principals are considered, not just a rule's.
func principalsForIdentities(policy *State, identities []string) *set.Set[string] {
	principalIDs := set.NewSet[string]()
	if len(identities) == 0 {
		return principalIDs
	}

	wanted := set.NewSetFromItems(identities...)
	for _, principal := range policy.allPrincipals {
		person, isV02 := principal.(*tufv02.Person)
		if !isV02 {
			continue
		}
		for appName, appEntry := range policy.GitHubApps {
			if !appEntry.IsTrusted() {
				continue
			}
			if identity, has := person.AssociatedIdentities[appName]; has && wanted.Has(identity) {
				principalIDs.Add(person.ID())
				break
			}
		}
	}
	return principalIDs
}

// cedarGroupsForPrincipal returns the Cedar groups a principal belongs to.
func cedarGroupsForPrincipal(policy *State, principalID string) []string {
	person, isV02 := policy.allPrincipals[principalID].(*tufv02.Person)
	if !isV02 {
		return nil
	}
	groups := []string{}
	for _, group := range strings.Split(person.Custom[cedarGroupsCustomKey], ",") {
		if group = strings.TrimSpace(group); group != "" {
			groups = append(groups, group)
		}
	}
	return groups
}

func cedarStringSet(items []string) cedar.Set {
	values := make([]cedar.Value, 0, len(items))
	for _, item := range items {
		values = append(values, cedar.String(item))
	}
	return cedar.NewSet(values...)
}

func cedarDiagnosticSummary(diagnostic cedar.Diagnostic) string {
	parts := []string{}
	for _, reason := range diagnostic.Reasons {
		parts = append(parts, fmt.Sprintf("policy %s", reason.PolicyID))
	}
	for _, evalErr := range diagnostic.Errors {
		parts = append(parts, fmt.Sprintf("error in policy %s: %s", evalErr.PolicyID, evalErr.Message))
	}
	if len(parts) == 0 {
		return "no policy applied"
	}
	return strings.Join(parts, "; ")
}

// cedarPullRequestFacts are the pull request facts passed to Cedar.
type cedarPullRequestFacts struct {
	number         int64
	authorIdentity string
	labels         []string
}

// loadCedarPullRequestFacts reads the hand-recorded pull request attestation,
// if one is configured, and returns its facts when it is bound to the RSL
// entry being verified (same commit and base branch). It returns nil when no
// attestation is configured or it is for a different change.
//
// SKETCH: the attestation is neither read from refs/gittuf/attestations nor
// signature-checked.
func loadCedarPullRequestFacts(policy *State, gitID githash.Hash) (*cedarPullRequestFacts, error) {
	path := os.Getenv(CedarPullRequestAttestationKey)
	if path == "" || gitID.IsZero() {
		return nil, nil
	}

	statement, err := loadCedarPullRequestStatement(path)
	if err != nil {
		return nil, err
	}

	entry, err := rsl.GetEntry(policy.repository, gitID)
	if err != nil {
		return nil, err
	}
	referenceEntry, isReferenceEntry := entry.(*rsl.ReferenceEntry)
	if !isReferenceEntry {
		return nil, nil
	}
	if statement.Subject[0].Digest["gitCommit"] != referenceEntry.TargetID.String() || "refs/heads/"+statement.Predicate.Base.Ref != referenceEntry.RefName {
		slog.Debug(fmt.Sprintf("Pull request attestation is not for RSL entry '%s', ignoring...", gitID.String()))
		return nil, nil
	}

	facts := &cedarPullRequestFacts{
		number:         int64(statement.Predicate.Number),
		authorIdentity: statement.Predicate.User.Login,
		labels:         []string{},
	}
	if statement.Predicate.User.ID != 0 {
		// Match the identity format the gittuf GitHub app records for
		// approvers: "<login>+<id>".
		facts.authorIdentity = fmt.Sprintf("%s+%d", statement.Predicate.User.Login, int64(statement.Predicate.User.ID))
	}
	for _, label := range statement.Predicate.Labels {
		facts.labels = append(facts.labels, label.Name)
	}
	return facts, nil
}

// cedarPullRequestStatement is the subset of a GitHub pull request
// attestation statement used by the Cedar overlay.
type cedarPullRequestStatement struct {
	Subject []struct {
		Digest map[string]string `json:"digest"`
	} `json:"subject"`
	PredicateType string `json:"predicate_type"`
	Predicate     struct {
		Number float64 `json:"number"`
		User   struct {
			Login string  `json:"login"`
			ID    float64 `json:"id"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"predicate"`
}

type cedarPullRequestCacheEntry struct {
	modTime   time.Time
	size      int64
	statement *cedarPullRequestStatement
}

var (
	cedarPullRequestCacheMu sync.Mutex
	cedarPullRequestCache   = map[string]cedarPullRequestCacheEntry{}
)

// loadCedarPullRequestStatement reads and checks a pull request attestation
// statement. Statements are cached per path and reloaded when the file
// changes.
func loadCedarPullRequestStatement(path string) (*cedarPullRequestStatement, error) {
	info, err := os.Stat(path) //nolint:gosec // SKETCH: path comes from a developer-mode env var by design
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCedarPullRequestAttestation, err)
	}

	cedarPullRequestCacheMu.Lock()
	defer cedarPullRequestCacheMu.Unlock()

	if entry, has := cedarPullRequestCache[path]; has && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.statement, nil
	}

	statementBytes, err := os.ReadFile(path) //nolint:gosec // SKETCH: path comes from a developer-mode env var by design
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCedarPullRequestAttestation, err)
	}

	statement := &cedarPullRequestStatement{}
	if err := json.Unmarshal(statementBytes, statement); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCedarPullRequestAttestation, err)
	}
	if statement.PredicateType != githubv01.PullRequestPredicateType || len(statement.Subject) != 1 || statement.Predicate.User.Login == "" {
		return nil, ErrInvalidCedarPullRequestAttestation
	}

	cedarPullRequestCache[path] = cedarPullRequestCacheEntry{modTime: info.ModTime(), size: info.Size(), statement: statement}
	return statement, nil
}

type cedarPolicyCacheEntry struct {
	modTime   time.Time
	size      int64
	policySet *cedar.PolicySet
}

var (
	cedarPolicyCacheMu sync.Mutex
	cedarPolicyCache   = map[string]cedarPolicyCacheEntry{}

	cedarSchemaOnce = sync.OnceValues(func() (*cedarresolved.Schema, error) {
		var schema cedarschema.Schema
		schema.SetFilename("cedar.cedarschema")
		if err := schema.UnmarshalCedar(cedarSchemaBytes); err != nil {
			return nil, err
		}
		return schema.Resolve()
	})
)

// loadCedarPolicySet loads and strictly validates the configured Cedar policy.
// Policies are cached per path and reloaded when the file changes.
func loadCedarPolicySet() (*cedar.PolicySet, error) {
	path := os.Getenv(CedarPolicyKey)
	if path == "" {
		return nil, ErrCedarPolicyNotSpecified
	}

	info, err := os.Stat(path) //nolint:gosec // SKETCH: path comes from a developer-mode env var by design
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCedarPolicy, err)
	}

	cedarPolicyCacheMu.Lock()
	defer cedarPolicyCacheMu.Unlock()

	if entry, has := cedarPolicyCache[path]; has && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.policySet, nil
	}

	slog.Debug(fmt.Sprintf("Loading Cedar policy from '%s'...", path)) //nolint:gosec // SKETCH: developer-mode env var
	policyBytes, err := os.ReadFile(path)                              //nolint:gosec // SKETCH: path comes from a developer-mode env var by design
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCedarPolicy, err)
	}

	policyList, err := cedar.NewPolicyListFromBytes(path, policyBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidCedarPolicy, err)
	}
	if len(policyList) == 0 {
		return nil, fmt.Errorf("%w: no policies in '%s'", ErrInvalidCedarPolicy, path)
	}

	schema, err := cedarSchemaOnce()
	if err != nil {
		return nil, fmt.Errorf("%w: unable to load schema: %w", ErrInvalidCedarPolicy, err)
	}
	validator := cedarvalidate.New(schema, cedarvalidate.WithStrict())

	// Name policies by their @id annotation so decisions report stable,
	// readable IDs; NewPolicySetFromBytes would name them by position.
	policySet := cedar.NewPolicySet()
	for i, policy := range policyList {
		policyID := cedar.PolicyID(fmt.Sprintf("policy%d", i))
		if id, has := policy.Annotations()["id"]; has {
			policyID = cedar.PolicyID(id)
		}
		if err := validator.Policy(string(policyID), (*cedarast.Policy)(policy.AST())); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidCedarPolicy, err)
		}
		if !policySet.Add(policyID, policy) {
			return nil, fmt.Errorf("%w: duplicate policy ID '%s'", ErrInvalidCedarPolicy, policyID)
		}
	}

	cedarPolicyCache[path] = cedarPolicyCacheEntry{modTime: info.ModTime(), size: info.Size(), policySet: policySet}
	return policySet, nil
}

// logCedarDecision appends a decision to the file named by GITTUF_CEDAR_LOG.
//
// SKETCH: evidence that the overlay ran during test runs.
func logCedarDecision(target, outcome, detail string) {
	path := os.Getenv(cedarDecisionLogKey)
	if path == "" {
		return
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // SKETCH: path comes from a developer-mode env var by design
	if err != nil {
		return
	}
	defer file.Close() //nolint:errcheck
	_, _ = fmt.Fprintf(file, "%s\t%s\t%s\n", outcome, target, detail)
}
