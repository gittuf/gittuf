# gittuf E2E Test: Policy Rollback via RSL

. "$(dirname "$0")/lib.sh" # use the lib.sh functions


# Part 1: Test on a single repository

init_git_repo

CONTROLLER_REPOSITORY="$(pwd)"
CONTROLLER_ROOT_KEY="$CONTROLLER_REPOSITORY/../keys/root"

gittuf trust init -k ../keys/root
gittuf trust make-controller -k ../keys/root
gittuf trust add-policy-key -k ../keys/root --policy-key ../keys/targets.pub
gittuf policy init -k ../keys/targets
append_policy "add-person" "authorized-user" "../keys/authorized.pub" "" "local" "apply" 

# Check no violation with "unauthorized" key due to no branch protection rule active
use_key unauthorized

echo 'Hello, world!' > README.md
git add README.md
git commit -m 'Initial commit'
gittuf rsl record main --local-only

# This will succeed, this is OK.
assert_passes gittuf verify-ref main

# Add branch protection rule; stage and apply policy
use_key authorized
append_policy "add-rule" "protect-main" "git:refs/heads/main" "authorized-user" "local" "apply"

# Simulate violation by using unauthorized key
use_key unauthorized

echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Another commit'
gittuf rsl record main --local-only

# This will fail as branch protection rule is violated, this is OK.
assert_fails "Test failed on branch protection rule check" gittuf verify-ref main 

# Rewind main branch and RSL to known good state
rollback 1

# Switch to unauthorized key
use_key authorized

# Dump current policy commit hash
POLICY_HEAD="$(git show -s --format='%H' refs/gittuf/policy)"

# Rewind policy ref temporarily to use gittuf to record the previous hash
git update-ref refs/gittuf/policy refs/gittuf/policy~1
git show -s --format='%H' refs/gittuf/policy

# Record RSL entry with this previous policy
gittuf rsl record refs/gittuf/policy --local-only

# Restore policy back to previous tip
git update-ref refs/gittuf/policy $POLICY_HEAD

echo 'Hello, world!!!' > README.md
git add README.md
git commit -m 'Evil commit'

# Record commit to RSL
gittuf rsl record main --local-only

# This should NOT succeed
assert_fails "branch protection rule should block unauthorized commit" gittuf verify-ref main 

# Part 2: Test with a downstream repository

init_git_repo

DOWNSTREAM_REPOSITORY="$(pwd)"

# Set up repo and add first repo as controller
gittuf trust init -k ../keys/root
gittuf trust add-policy-key -k ../keys/root --policy-key ../keys/targets.pub
gittuf policy init -k ../keys/targets
append_policy "add-person" "authorized-user" "../keys/authorized.pub" "" "local" "no_apply"
gittuf trust -k ../keys/root add-controller-repository --location $CONTROLLER_REPOSITORY --name controller-repo --initial-root-principal $CONTROLLER_ROOT_KEY

gittuf policy stage --local-only
gittuf policy apply --local-only

# This should NOT succeed
assert_fails "Test failed on controller repository check" gittuf rsl propagate 

print_result
