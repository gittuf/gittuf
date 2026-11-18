#!/usr/bin/env bash
# gittuf E2E Test: Intermediate Controller Policy Propagation
set -euo pipefail

. "$(dirname "$0")/lib.sh"

export GITTUF_DEV=1

# Set up a controller with no global rules initially.
init_git_repo
setup_gittuf_basic

"$GITTUF_BIN" trust -k ../keys/root make-controller
"$GITTUF_BIN" policy stage --local-only
"$GITTUF_BIN" policy apply --local-only

CONTROLLER_REPOSITORY="$(pwd)"
CONTROLLER_ROOT_KEY="$CONTROLLER_REPOSITORY/../keys/root.pub"

# Protect main locally and inherit the controller's policy.
init_git_repo
DOWNSTREAM_REPOSITORY="$(pwd)"
setup_gittuf_basic

"$GITTUF_BIN" policy add-rule -k ../keys/targets --rule-name protect-main --rule-pattern git:refs/heads/main --authorize authorized-user
"$GITTUF_BIN" trust -k ../keys/root add-controller-repository --location "$CONTROLLER_REPOSITORY" --name controller-repo --initial-root-principal "$CONTROLLER_ROOT_KEY"
"$GITTUF_BIN" policy stage --local-only
"$GITTUF_BIN" policy apply --local-only
"$GITTUF_BIN" rsl propagate

echo 'Initial content' > README.md
git add README.md
git commit -m 'Initial commit'
"$GITTUF_BIN" rsl record main --local-only
assert_passes "$GITTUF_BIN" verify-ref main

# A valid update must still pass after an intermediate propagation.
cd "$CONTROLLER_REPOSITORY"
"$GITTUF_BIN" trust -k ../keys/root add-global-rule --rule-name main-approvals --type threshold --rule-pattern git:refs/heads/main --threshold 1
"$GITTUF_BIN" policy stage --local-only
"$GITTUF_BIN" policy apply --local-only

cd "$DOWNSTREAM_REPOSITORY"
"$GITTUF_BIN" rsl propagate

echo 'Approved content' >> README.md
git add README.md
git commit -m 'Update with one approval'
"$GITTUF_BIN" rsl record main --local-only
assert_passes "$GITTUF_BIN" verify-ref main

# Raise the controller's threshold without changing the downstream's local
# policy. Full verification must load this propagation entry during its walk.
cd "$CONTROLLER_REPOSITORY"
"$GITTUF_BIN" trust -k ../keys/root update-global-rule --rule-name main-approvals --type threshold --rule-pattern git:refs/heads/main --threshold 2
"$GITTUF_BIN" policy stage --local-only
"$GITTUF_BIN" policy apply --local-only

cd "$DOWNSTREAM_REPOSITORY"
"$GITTUF_BIN" rsl propagate

echo 'Insufficiently approved content' >> README.md
git add README.md
git commit -m 'Update with insufficient approvals'
"$GITTUF_BIN" rsl record main --local-only

# Checking the latest entry confirms the stricter policy was propagated.
assert_fails "latest policy should require two approvals" "verifying Git namespace policies failed" "$GITTUF_BIN" verify-ref main --latest-only

# This is the regression guard: the old verifier ignored intermediate
# propagation entries and incorrectly accepted this history.
assert_fails "full verification should enforce the propagated approval threshold" "verifying Git namespace policies failed" "$GITTUF_BIN" verify-ref main

print_result
