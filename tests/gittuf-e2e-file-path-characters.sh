#!/usr/bin/env bash
# gittuf E2E Test: Incorrect Parsing of Filenames
set -euo pipefail

. "$(dirname "$0")/lib.sh"

init_git_repo
# Exercise Git's default quoting behavior regardless of inherited settings.
git config --local core.quotePath true
setup_gittuf_basic
use_key authorized1

echo 'Hello, world!!' > README.md
git add README.md
git commit -m 'Initial commit'
$GITTUF_BIN rsl record main --local-only

# This will succeed, this is OK.
assert_passes $GITTUF_BIN verify-ref main

# A prefix pattern must match the actual filename, but cannot match Git's
# incorrectly quoted pathname representation as a catch-all pattern would.
$GITTUF_BIN policy add-rule -k ../keys/targets --rule-name 'protect-readme' --rule-pattern 'file:READM*' --authorize authorized-user
$GITTUF_BIN policy stage --local-only
$GITTUF_BIN policy apply --local-only

# Simulate violation by using unauthorized key
use_key unauthorized

# Add the usual file
echo 'Hello, world!!!' > README.md
git add README.md
git commit -m 'Add README.md'
$GITTUF_BIN rsl record main --local-only

# This will fail as file protection rule is violated
assert_fails "file protection rule check" "verifying file namespace policies failed" $GITTUF_BIN verify-ref main

# Rewind main branch and RSL to known good state
rollback 1

# Add READM€.md instead
echo 'Hello, world!!' > 'READM€.md'
git add 'READM€.md'
git commit -m 'Add READM€.md'
$GITTUF_BIN rsl record main --local-only

# Git's quoted output must not prevent matching the file protection rule.
assert_fails "file protection check should still fail even with special character" "verifying file namespace policies failed" $GITTUF_BIN verify-ref main

print_result
