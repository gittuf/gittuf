#!/usr/bin/env bash
# gittuf E2E Test: Basic Functionality
set -euo pipefail

. "$(dirname "$0")/lib.sh"

init_git_repo

setup_gittuf_basic

# Add branch protection rule
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

echo 'Hello, world!' > README.md
git add README.md
git commit -m 'Initial commit'
$GITTUF_BIN rsl record main --local-only

# This will succeed!
assert_passes $GITTUF_BIN verify-ref main

# Simulate violation by using unauthorized key
use_key unauthorized
echo 'This is not allowed!' >> README.md
git add README.md
git commit -m 'Update README.md'
$GITTUF_BIN rsl record main --local-only

# This will fail as branch protection rule is violated!
assert_fails "branch protection rule should block unauthorized commit" "verifying Git namespace policies failed" $GITTUF_BIN verify-ref main

# Rewind to known good state
rollback 1
use_key authorized1

# Add file protection rule
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-readme' --rule-pattern file:README.md --authorize authorized-user
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# Make change to README.md using unauthorized key
use_key unauthorized
echo 'This is not allowed!' >> README.md
git add README.md
git commit -m 'Update README.md'

# But record RSL entry using authorized key to meet branch protection rule
use_key authorized1
$GITTUF_BIN rsl record main --local-only

# This will fail as file protection rule is violated!
assert_fails "file protection rule should block unauthorized commit" "verifying file namespace policies failed" $GITTUF_BIN verify-ref main

# Rewind to known good state
rollback 1
use_key authorized1

# Add tag protection rule
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-releases' --rule-pattern "git:refs/tags/v*" --authorize authorized-user
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# Tag v1 using unauthorized key
use_key unauthorized
git tag v1 -m "Unauthorized release"
$GITTUF_BIN rsl record v1 --local-only

# This will fail as tag protection rule is violated!
assert_fails "tag protection rule should block unauthorized tag" "verifying tag entry failed" $GITTUF_BIN verify-ref refs/tags/v1

print_result
