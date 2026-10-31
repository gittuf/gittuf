// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// SKETCH (cedar-integration-sketch, not for merge).
//
// Baseline policy. gittuf's final verdict is native AND Cedar, so Cedar only
// ever narrows what gittuf's native rules already accepted. This file permits
// everything; each ruleN-*.cedar file adds a forbid on top of it. Deploy this
// file together with one or more rule files (they can all be concatenated
// into a single policy set). Without it, Cedar's default deny would reject
// every change.
@id("permit-all")
permit (principal, action, resource);
