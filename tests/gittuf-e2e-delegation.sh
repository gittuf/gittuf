# gittuf E2E Test: Delegation chain

. "$(dirname "$0")/lib.sh"

# Part 1: Basic delegation chain
# auth1 delegates to auth2, auth2 delegates to auth3
# removing auth2 should revoke auth3's access

init_git_repo 3

setup_basic_repo

# auth1 adds auth2 as a person and creates a delegation rule named 'protect-main'
# the policy file name must match the rule name
gittuf policy add-person -k ../keys/targets --person-ID 'auth2' --public-key ../keys/authorized2.pub
gittuf policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize auth2
gittuf policy stage --local-only
gittuf policy apply --local-only

# auth2 initializes policy file named 'protect-main' (must match rule name above)
gittuf policy init -k ../keys/authorized2 --policy-name protect-main
gittuf policy add-person -k ../keys/authorized2 --person-ID 'auth3' --public-key ../keys/authorized3.pub --policy-name protect-main
gittuf policy add-rule -k ../keys/authorized2 --rule-name 'auth3-can-commit' --rule-pattern git:refs/heads/main --authorize auth3 --policy-name protect-main
gittuf policy stage --local-only
gittuf policy apply --local-only

# auth3 makes a commit — should pass
use_key authorized3
echo 'Hello from auth3!' > README.md
git add README.md
git commit -m 'auth3 commit'
gittuf rsl record main --local-only
assert_passes gittuf verify-ref main

# auth1 tears down delegation chain: clear delegated policy first, then remove from targets
gittuf policy remove-rule -k ../keys/authorized2 --rule-name 'auth3-can-commit' --policy-name protect-main
gittuf policy remove-person -k ../keys/authorized2 --person-ID 'auth3' --policy-name protect-main
use_key authorized1
gittuf policy remove-rule -k ../keys/targets --rule-name 'protect-main'
gittuf policy remove-person -k ../keys/targets --person-ID 'auth2'
gittuf policy stage --local-only
gittuf policy apply --local-only

# auth3 tries to make another commit — should fail
use_key authorized3
echo 'Hello again from auth3!' >> README.md
git add README.md
git commit -m 'auth3 commit after auth2 removed'
gittuf rsl record main --local-only
assert_fails "auth3 access should be revoked when auth2 is removed" gittuf verify-ref main

# Part 2: Threshold cannot be overwhelmed by a single delegated user
init_git_repo 4

setup_basic_repo

# auth1 sets up a rule requiring 3 of auth2, auth3, auth4 to approve changes
gittuf policy add-person -k ../keys/targets --person-ID 'auth2' --public-key ../keys/authorized2.pub
gittuf policy add-person -k ../keys/targets --person-ID 'auth3' --public-key ../keys/authorized3.pub
gittuf policy add-person -k ../keys/targets --person-ID 'auth4' --public-key ../keys/authorized4.pub
gittuf policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize auth2 --authorize auth3 --authorize auth4 --threshold 3
gittuf policy stage --local-only
gittuf policy apply --local-only

# auth2 alone tries to make a commit — should fail, threshold not met
use_key authorized2
echo 'Hello from auth2!' > README.md
git add README.md
git commit -m 'auth2 unilateral commit'
gittuf rsl record main --local-only
assert_fails "single user should not meet threshold of 3" gittuf verify-ref main

print_result