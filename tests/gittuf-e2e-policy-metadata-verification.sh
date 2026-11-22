#!/usr/bin/env bash
# gittuf E2E Test: Policy Metadata Verification
set -euo pipefail

. "$(dirname "$0")/lib.sh"

init_git_repo

setup_gittuf_basic

# Record a valid entry before the invalid policy transition.
echo 'Initial content' > README.md
git add README.md
git commit -m 'Initial commit'
$GITTUF_BIN rsl record main --local-only
FIRST_ENTRY="$(git rev-parse refs/gittuf/reference-state-log)"
assert_passes $GITTUF_BIN verify-ref main

# Create a policy update whose targets metadata is signed by an unauthorized
# key. Policy editing permits collecting signatures incrementally, so this
# succeeds while policy apply correctly rejects the incomplete metadata.
$GITTUF_BIN policy add-rule -k ../keys/unauthorized --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user
$GITTUF_BIN policy stage --local-only

# Bypass policy apply to model an invalid policy state received through the
# repository. Record the update in the RSL so verification encounters it.
git update-ref refs/gittuf/policy refs/gittuf/policy-staging
$GITTUF_BIN rsl record refs/gittuf/policy --local-only

# This entry satisfies the rule in the invalid metadata. Verification must
# still reject the policy transition because the metadata signature is invalid.
echo 'Updated content' >> README.md
git add README.md
git commit -m 'Update README.md'
$GITTUF_BIN rsl record main --local-only

assert_fails "invalidly signed policy metadata should be rejected" "threshold constraints not met" env GITTUF_DEV=1 $GITTUF_BIN verify-ref main --from-entry "$FIRST_ENTRY"

print_result
