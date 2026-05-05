# gittuf E2E Test: Policy Rollback via RSL

. "$(dirname "$0")/lib.sh" # use the lib.sh functions


init_git_repo

gittuf trust init -k ../keys/root
gittuf trust add-policy-key -k ../keys/root --policy-key ../keys/targets.pub
gittuf policy init -k ../keys/targets

# Add trusted person to gittuf policy file
append_policy "add-person" "authorized-user" "../keys/authorized.pub" "" "local" "no_apply"

# Add branch protection rule
# stage and apply
append_policy "add-rule" "protect-main" "git:refs/heads/main" "authorized-user" "local" "apply"

echo 'Hello, world!' > README.md
git add README.md
git commit -m 'Initial commit'

gittuf rsl record main --local-only

# This will succeed!
assert_passes gittuf verify-ref main

# Simulate violation by using unauthorized key
use_key unauthorized

echo 'This is not allowed!' >> README.md
git add README.md
git commit -m 'Update README.md'

gittuf rsl record main --local-only


# This will fail as branch protection rule is violated!
assert_fails "" gittuf verify-ref main 


# Rewind to known good state
rollback 1
use_key authorized

# Add file protection rule
# Stage and apply policy
append_policy "add-rule" "protect-readme" "file:README.md" "authorized-user" "local" "apply"

# Make change to README.md using unauthorized key
use_key unauthorized

echo 'This is not allowed!' >> README.md
git add README.md
git commit -m 'Update README.md'

# But record RSL entry using authorized key to meet branch protection rule
use_key authorized
gittuf rsl record main --local-only

# This will fail as file protection rule is violated!
assert_fails "" gittuf verify-ref main

# Rewind to known good state
rollback 1
use_key authorized

# Add tag protection rule
# Stage and apply policy
append_policy "add-rule" "protect-releases" "git:refs/tags/v*" "authorized-user" "local" "apply"

# Tag v1 using unauthorized key
use_key unauthorized
git tag v1 -m "Unauthorized release"

# Record to RSL and verify tag
gittuf rsl record v1 --local-only

# This will fail as tag protection rule is violated!
assert_fails "" gittuf verify-ref --verbose refs/tags/v1 

print_result
