#!/usr/bin/env bash
# gittuf E2E Test: Policy Rollback via RSL
set -euo pipefail

. "$(dirname "$0")/lib.sh"

# Part 1: Test on a single repository

init_git_repo

CONTROLLER_REPOSITORY="$(pwd)"
CONTROLLER_ROOT_KEY="$CONTROLLER_REPOSITORY/../keys/root"

setup_gittuf_basic

# Check no violation with "unauthorized" key due to no branch protection rule active
use_key unauthorized

echo 'Hello, world!' > README.md
git add README.md
git commit -m 'Initial commit'
$GITTUF_BIN rsl record main --local-only

# This will succeed, this is OK.
assert_passes $GITTUF_BIN verify-ref main

# Add branch protection rule; stage and apply policy
use_key authorized1
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# Simulate violation by using unauthorized key
use_key unauthorized

echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Another commit'
$GITTUF_BIN rsl record main --local-only

# This will fail as branch protection rule is violated
assert_fails "branch protection rule check" "verifying Git namespace policies failed" $GITTUF_BIN verify-ref main

# Rewind main branch and RSL to known good state
rollback 1
use_key authorized1

# Dump current policy commit hash
POLICY_HEAD="$(git show -s --format='%H' refs/gittuf/policy)"

# Rewind policy ref temporarily to record the previous hash
git update-ref refs/gittuf/policy refs/gittuf/policy~1

# Record RSL entry with this previous policy
$GITTUF_BIN rsl record refs/gittuf/policy --local-only

# Restore policy back to previous tip
git update-ref refs/gittuf/policy "$POLICY_HEAD"

echo 'Hello, world!!!' > README.md
git add README.md
git commit -m 'Evil commit'
$GITTUF_BIN rsl record main --local-only

# This should NOT succeed
assert_fails "policy rollback should be detected" "gittuf policy metadata rollback detected" $GITTUF_BIN verify-ref main

# Part 2: Test with a downstream repository

init_git_repo

DOWNSTREAM_REPOSITORY="$(pwd)"

# Set up repo and add first repo as controller
setup_gittuf_basic

$GITTUF_BIN trust -k ../keys/root add-controller-repository --location "$CONTROLLER_REPOSITORY" --name controller-repo --initial-root-principal "$CONTROLLER_ROOT_KEY"

$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# This should NOT succeed
assert_fails "controller repository check should fail without valid policy" "gittuf policy metadata rollback detected" $GITTUF_BIN rsl propagate

print_result
