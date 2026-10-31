// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// SKETCH (cedar-integration-sketch, not for merge).
//
// Rule 5: forbidden principal. An update to git refs/heads/main is rejected
// when principal "mallory" is credited toward the accepting rule, even if the
// native threshold was met without her. Useful to block a principal quickly
// without rewriting the signed rule's principal list.
//
// Use together with permit-all.cedar.
@id("rule5-forbid-principal")
forbid (
  principal,
  action == Gittuf::Action::"updateRef",
  resource
)
when {
  resource.scheme == "git" &&
  resource.name == "refs/heads/main" &&
  context.credited.contains(Gittuf::Principal::"mallory")
};
