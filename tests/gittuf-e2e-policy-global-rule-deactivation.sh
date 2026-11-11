#!/usr/bin/env bash
# gittuf E2E Test: Global Rules Deactivate Policy Rules
set -euo pipefail

. "$(dirname "$0")/lib.sh"

# Part one: Global rule with T=1, and branch protection rule with T=2
init_git_repo 2

setup_gittuf_basic

$GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'authorized-user2' --public-key ../keys/authorized2.pub

# Add branch protection rule
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user --authorize authorized-user2 --threshold 2

# Add global rule for main requiring threshold of 1
$GITTUF_BIN trust add-global-rule -k ../keys/root --threshold 1 --rule-name 'test-global-threshold' --type threshold --rule-pattern 'git:refs/heads/main'

# Stage and apply policy
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

use_key authorized1

# Commit to main
echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Commit two!'

$GITTUF_BIN rsl record main --local-only

# This should NOT pass.
assert_fails "branch protection check with global rule" "verifying Git namespace policies failed" $GITTUF_BIN verify-ref --verbose main


# Part two: Global rule with T=1, and branch protection rule with T=2, person not on BPR signs for said namespace
init_git_repo 2

setup_gittuf_basic

$GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'authorized-user2' --public-key ../keys/authorized2.pub

# Add branch protection rule
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user --threshold 1

# Add global rule for main requiring threshold of 1
$GITTUF_BIN trust add-global-rule -k ../keys/root --threshold 1 --rule-name 'test-global-threshold' --type threshold --rule-pattern 'git:refs/heads/main'

# Stage and apply policy
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# Simulate violation by using unauthorized key
use_key authorized2

echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Commit!'

$GITTUF_BIN rsl record main --local-only

# This should NOT pass.
assert_fails "branch protection check with global rule" "verifying Git namespace policies failed" $GITTUF_BIN verify-ref --verbose main


# Part three: Global rule allows user added to *different* policy file to sign for namespace that already has BPR
init_git_repo 2

setup_gittuf_basic

# Add branch protection rule
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern git:refs/heads/main --authorize authorized-user --threshold 1

# Add global rule for main requiring threshold of 1
$GITTUF_BIN trust add-global-rule -k ../keys/root --threshold 1 --rule-name 'test-global-threshold' --type threshold --rule-pattern 'git:refs/heads/main'

# Add delegated policy file
$GITTUF_BIN policy init -k ../keys/authorized1.pub --policy-name 'protect-main'

# Add person to delegated file
$GITTUF_BIN policy add-person -k ../keys/authorized1.pub --policy-name 'protect-main' --person-ID 'authorized-user2' --public-key ../keys/authorized2.pub

# Stage and apply policy
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# Simulate violation by using unauthorized key
use_key authorized2

echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Commit!'

$GITTUF_BIN rsl record main --local-only

# This should NOT pass.
assert_fails "branch protection check with global rule" "verifying Git namespace policies failed" $GITTUF_BIN verify-ref --verbose main


# Part four: Use file protection rule instead of branch protection rule
init_git_repo 2

setup_gittuf_basic

$GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'authorized-user2' --public-key ../keys/authorized2.pub

# Add branch protection rule
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-main' --rule-pattern file:README.md --authorize authorized-user --authorize authorized-user2 --threshold 2

# Add global rule for main requiring threshold of 1
$GITTUF_BIN trust add-global-rule -k ../keys/root --threshold 1 --rule-name 'test-global-threshold' --type threshold --rule-pattern 'git:refs/heads/main'

# Stage and apply policy
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# Commit to main
echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Commit two!'

$GITTUF_BIN rsl record main --local-only

# This should NOT pass.
assert_fails "file protection check with global rule" "verifying file namespace policies failed" $GITTUF_BIN verify-ref --verbose main

print_result
