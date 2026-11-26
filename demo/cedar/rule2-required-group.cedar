// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// SKETCH (cedar-integration-sketch, not for merge).
//
// Rule 2: required group. An update to git refs/heads/main is rejected unless
// at least one principal credited toward the gittuf rule that accepted the
// change is a member of the "release-managers" group. creditedGroups is
// computed by gittuf from the signed policy (Person.Custom["cedar-groups"]).
//
// Use together with permit-all.cedar.
@id("rule2-required-group")
forbid (
  principal,
  action == Gittuf::Action::"updateRef",
  resource
)
when { resource.scheme == "git" && resource.name == "refs/heads/main" }
unless { context.creditedGroups.contains(Gittuf::Group::"release-managers") };
