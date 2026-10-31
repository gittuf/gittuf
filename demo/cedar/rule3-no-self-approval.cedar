// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// SKETCH (cedar-integration-sketch, not for merge).
//
// Rule 3: no self-approval. A change is rejected when the pull request author
// is among its approvers. Applies to every target and action. Two forms:
//
// 3a (principal match): the PR author maps to a gittuf principal (through a
// policy person's AssociatedIdentities) and that principal is among the
// approvers. Catches an author who approved from a second linked account.
//
// 3b (identity match): the raw "<login>+<id>" author identity appears among
// the raw approver identities. Works even when the author is not a policy
// person.
//
// pr and pr.author are optional in the schema, so both are guarded with `has`.
//
// Use together with permit-all.cedar.
@id("rule3a-no-self-approval-principal")
forbid (principal, action, resource)
when {
  context has pr &&
  context.pr has author &&
  context.approvers.contains(context.pr.author)
};

@id("rule3b-no-self-approval-identity")
forbid (principal, action, resource)
when {
  context has pr &&
  context.approverIdentities.contains(context.pr.authorIdentity)
};
