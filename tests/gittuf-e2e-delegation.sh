#!/usr/bin/env bash
# gittuf E2E Test: Delegation chain
set -euo pipefail

. "$(dirname "$0")/lib.sh"

# Part 1: Basic delegation chain
# auth1 delegates to auth2, auth2 delegates to auth3
# removing auth2 should revoke auth3's access

init_git_repo 3

setup_gittuf_basic

# auth1 adds auth2 as a person and creates a delegation rule named 'protect-main'
# the policy file name must match the rule name
$GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'auth2' --public-key ../keys/authorized2.pub
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize auth2
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# auth2 initializes policy file named 'protect-main' (must match rule name above)
$GITTUF_BIN policy init -k ../keys/authorized2 --policy-name protect-main
$GITTUF_BIN policy add-person -k ../keys/authorized2 --person-ID 'auth3' --public-key ../keys/authorized3.pub --policy-name protect-main
$GITTUF_BIN policy add-rule -k ../keys/authorized2 --rule-name 'auth3-can-commit' --rule-pattern git:refs/heads/main --authorize auth3 --policy-name protect-main
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# auth3 makes a commit — should pass
use_key authorized3
echo 'Hello from auth3!' > README.md
git add README.md
git commit -m 'auth3 commit'
$GITTUF_BIN rsl record main --local-only
assert_passes $GITTUF_BIN verify-ref main


## commented out as we don't have a solution yet for deleting unused policy files, needs update
# # auth1 tears down delegation chain: clear delegated policy first, then remove from targets
# $GITTUF_BIN policy remove-rule -k ../keys/authorized2 --rule-name 'auth3-can-commit' --policy-name protect-main
# $GITTUF_BIN policy remove-person -k ../keys/authorized2 --person-ID 'auth3' --policy-name protect-main
# use_key authorized1
# $GITTUF_BIN policy remove-rule -k ../keys/targets --rule-name 'protect-main'
# $GITTUF_BIN policy remove-person -k ../keys/targets --person-ID 'auth2'
# $GITTUF_BIN policy stage --local-only
# $GITTUF_BIN policy apply --local-only

# # auth3 tries to make another commit — should fail
# use_key authorized3
# echo 'Hello again from auth3!' >> README.md
# git add README.md
# git commit -m 'auth3 commit after auth2 removed'
# $GITTUF_BIN rsl record main --local-only
# assert_fails "auth3 access should be revoked when auth2 is removed" $GITTUF_BIN verify-ref main

# Part 2: Threshold cannot be overwhelmed by a single delegated user
init_git_repo 4

setup_gittuf_basic

# auth1 sets up a rule requiring 3 of auth2, auth3, auth4 to approve changes
$GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'auth2' --public-key ../keys/authorized2.pub
$GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'auth3' --public-key ../keys/authorized3.pub
$GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'auth4' --public-key ../keys/authorized4.pub
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize auth2 --authorize auth3 --authorize auth4 --threshold 3
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# auth2 alone tries to make a commit — should fail, threshold not met
use_key authorized2
echo 'Hello from auth2!' > README.md
git add README.md
git commit -m 'auth2 unilateral commit'
$GITTUF_BIN rsl record main --local-only
assert_fails "single user should not meet threshold of 3" "verifying Git namespace policies failed" $GITTUF_BIN verify-ref main

print_result
