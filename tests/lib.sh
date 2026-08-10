#!/usr/bin/env bash
# gittuf E2E Test: shared helpers for E2E tests

init_git_repo(){
    cd "$(mktemp -d)"

    mkdir {keys,repo}

    cd keys
    ssh-keygen -q -t ecdsa -N "" -f root
    ssh-keygen -q -t ecdsa -N "" -f targets
    ssh-keygen -q -t ecdsa -N "" -f unauthorized

    local NUM_USRS=${1:-1}
    for ((i=1; i<=NUM_USRS; i++)); do
        ssh-keygen -q -t ecdsa -N "" -f "authorized$i"
    done

    cd ../repo

    git init -b main
    git config --local gpg.format ssh
    git config --local commit.gpgsign true
    git config --local tag.gpgsign true
    git config --local user.signingkey ../keys/authorized1
    git config --local user.name gittuf-demo
    git config --local user.email gittuf.demo@example.com
}

setup_gittuf_basic() {
    $GITTUF_BIN trust init -k ../keys/root
    $GITTUF_BIN trust add-policy-key -k ../keys/root --policy-key ../keys/targets.pub
    $GITTUF_BIN policy init -k ../keys/targets
    $GITTUF_BIN policy add-person -k ../keys/targets --person-ID 'authorized-user' --public-key ../keys/authorized1.pub
    $GITTUF_BIN policy stage --local-only
    $GITTUF_BIN policy apply --local-only
}

rollback(){
    git reset --hard HEAD~$1
    git update-ref refs/gittuf/reference-state-log refs/gittuf/reference-state-log~$1
}

use_key(){
    git config --local user.signingkey "../keys/$1"
}

RAN=0
PASS=0
FAIL=0
SKIP=0

assert_passes(){
    RAN=$((RAN+1))
    if "$@"; then
        PASS=$((PASS+1))
        echo "PASS: $*"
    else
        FAIL=$((FAIL+1))
        echo "FAIL: $* (expected success)"
    fi
}

assert_fails(){
    local MSG=$1 WANT=$2
    shift 2
    RAN=$((RAN+1))
    if { "$@" 2>&1 || true; } | grep -Eq -- "$WANT"; then
        PASS=$((PASS+1))
        echo "PASS: Test passed $MSG"
    else
        FAIL=$((FAIL+1))
        echo "FAIL: Test failed $MSG (expected failure matching /$WANT/)"
    fi
}

assert_skip(){
    SKIP=$((SKIP+1))
    echo "SKIP: $*"
}

print_result(){
    echo "========================="
    echo "Result:"
    echo "[TOTAL_TESTS]: $((RAN+SKIP))"
    echo "[TESTS_RAN]: $RAN"
    echo "[TESTS_PASSED]: $PASS"
    echo "[TESTS_FAILED]: $FAIL"
    echo "[TESTS_SKIPPED]: $SKIP"
    echo "========================="
    [ "$FAIL" -eq 0 ]
}
