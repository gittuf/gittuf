# gittuf E2E Test: Basic Functionality

. "$(dirname "$0")/lib.sh"

init_git_repo

setup_basic_repo

# Add branch protection rule
gittuf policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user
gittuf policy stage --local-only
gittuf policy apply --local-only

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
assert_fails "branch protection rule should block unauthorized commit" gittuf verify-ref main

# Rewind to known good state
rollback 1
use_key authorized1

# Add file protection rule
gittuf policy add-rule -k ../keys/targets --rule-name 'protect-readme' --rule-pattern file:README.md --authorize authorized-user
gittuf policy stage --local-only
gittuf policy apply --local-only

# Make change to README.md using unauthorized key
use_key unauthorized
echo 'This is not allowed!' >> README.md
git add README.md
git commit -m 'Update README.md'

# But record RSL entry using authorized key to meet branch protection rule
use_key authorized1
gittuf rsl record main --local-only

# This will fail as file protection rule is violated!
assert_fails "file protection rule should block unauthorized commit" gittuf verify-ref main

# Rewind to known good state
rollback 1
use_key authorized1

# Add tag protection rule
gittuf policy add-rule -k ../keys/targets --rule-name 'protect-releases' --rule-pattern "git:refs/tags/v*" --authorize authorized-user
gittuf policy stage --local-only
gittuf policy apply --local-only

# Tag v1 using unauthorized key
use_key unauthorized
git tag v1 -m "Unauthorized release"
gittuf rsl record v1 --local-only

# This will fail as tag protection rule is violated!
assert_fails "tag protection rule should block unauthorized tag" gittuf verify-ref --verbose refs/tags/v1

print_result