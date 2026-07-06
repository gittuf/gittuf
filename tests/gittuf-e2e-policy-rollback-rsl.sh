# gittuf E2E Test: Policy Rollback via RSL

. "$(dirname "$0")/lib.sh"

# Part 1: Test on a single repository

init_git_repo

CONTROLLER_REPOSITORY="$(pwd)"
CONTROLLER_ROOT_KEY="$CONTROLLER_REPOSITORY/../keys/root"

setup_basic_repo

# Check no violation with "unauthorized" key due to no branch protection rule active
use_key unauthorized

echo 'Hello, world!' > README.md
git add README.md
git commit -m 'Initial commit'
gittuf rsl record main --local-only

# This will succeed, this is OK.
assert_passes gittuf verify-ref main

# Add branch protection rule; stage and apply policy
use_key authorized1
gittuf policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user
gittuf policy stage --local-only
gittuf policy apply --local-only

# Simulate violation by using unauthorized key
use_key unauthorized

echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Another commit'
gittuf rsl record main --local-only

# This will fail as branch protection rule is violated
assert_fails "branch protection rule check" gittuf verify-ref main

# Rewind main branch and RSL to known good state
rollback 1
use_key authorized1

# Dump current policy commit hash
POLICY_HEAD="$(git show -s --format='%H' refs/gittuf/policy)"

# Rewind policy ref temporarily to record the previous hash
git update-ref refs/gittuf/policy refs/gittuf/policy~1

# Record RSL entry with this previous policy
gittuf rsl record refs/gittuf/policy --local-only

# Restore policy back to previous tip
git update-ref refs/gittuf/policy $POLICY_HEAD

echo 'Hello, world!!!' > README.md
git add README.md
git commit -m 'Evil commit'
gittuf rsl record main --local-only

# This should NOT succeed
assert_fails "policy rollback should be detected" gittuf verify-ref main

# Part 2: Test with a downstream repository

init_git_repo

DOWNSTREAM_REPOSITORY="$(pwd)"

# Set up repo and add first repo as controller
setup_basic_repo

gittuf trust -k ../keys/root add-controller-repository --location $CONTROLLER_REPOSITORY --name controller-repo --initial-root-principal $CONTROLLER_ROOT_KEY

gittuf policy stage --local-only
gittuf policy apply --local-only

# This should NOT succeed
assert_fails "controller repository check should fail without valid policy" gittuf rsl propagate

print_result