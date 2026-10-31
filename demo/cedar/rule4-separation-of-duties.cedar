// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// SKETCH (cedar-integration-sketch, not for merge).
//
// Rule 4: separation of duties. An update to git refs/heads/main is rejected
// unless the credited principals together cover all three groups: dev, qa and
// security. One principal in several groups covers each of them, so pair this
// with a native threshold if distinct people are required.
//
// Use together with permit-all.cedar.
@id("rule4-separation-of-duties")
forbid (
  principal,
  action == Gittuf::Action::"updateRef",
  resource
)
when { resource.scheme == "git" && resource.name == "refs/heads/main" }
unless {
  context.creditedGroups.containsAll([
    Gittuf::Group::"dev",
    Gittuf::Group::"qa",
    Gittuf::Group::"security"
  ])
};
