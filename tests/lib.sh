# gittuf E2E Test: shared helpers for E2E tests

# Exit on nonzero return code
set -e


# Intializes git repo with tuf setups
init_git_repo(){
    # make a tmp dir with a random name 
    cd "$(mktemp -d)"

    mkdir {keys,repo}
    
    cd keys
    ssh-keygen -q -t ecdsa -N "" -f root
    ssh-keygen -q -t ecdsa -N "" -f targets
    ssh-keygen -q -t ecdsa -N "" -f authorized
    ssh-keygen -q -t ecdsa -N "" -f unauthorized

    cd ../repo

    git init -b main
    git config --local gpg.format ssh
    git config --local commit.gpgsign true
    git config --local tag.gpgsign true
    git config --local user.signingkey ../keys/authorized
    git config --local user.name gittuf-demo
    git config --local user.email gittuf.demo@example.com
}

# Counters and assertions
RAN=0 #tests that ran
PASS=0  #tests that passes
FAIL=0  #tests that failed
SKIP=0  #tests that skipped

assert_passes(){ # expected sucess
    RAN=$((RAN+1)) # append run by 1

    if "$@"; then
        PASS=$((PASS+1))
        echo "PASS: $*"
    else 
        FAIL=$((FAIL+1))
        echo "FAIL: $* (expected success)"
    fi
}

assert_fails(){ #expects fail
    local MSG=$1
    shift #shifts so $@ doesn't include $1
    RAN=$((RAN+1)) # append run by 1
    

    if ! "$@"; then
        PASS=$((PASS+1))
        echo "PASS: $MSG"
    else 
        FAIL=$((FAIL+1))
        echo "FAIL: $MSG (expected failure)"
    fi
}

assert_skip(){ ##skipped test
    SKIP=$((SKIP+1))
    echo "SKIP: $*"
}

rollback() { # rollback <iteration>
    local ITERATION=$1
    for ((i=0; i<ITERATION; i++)); do
        git reset --hard HEAD~1
        git update-ref refs/gittuf/reference-state-log refs/gittuf/reference-state-log~1
    done
}

append_policy(){
    local ADD=$1
    local NAME=$2
    local PATTERN=$3
    local AUTHORIZE=$4
    local MODE=$5
    local APPLY=$6  

    if [ "$ADD" == "add-rule" ];then
        gittuf policy add-rule -k ../keys/targets \
            --rule-name "$NAME" \
            --rule-pattern "$PATTERN" \
            --authorize "$AUTHORIZE"
    elif [ "$ADD" == "add-person" ];then
        gittuf policy add-person -k ../keys/targets \
        --person-ID "$NAME" \
        --public-key "$PATTERN"
    fi

    # stage and apply
    if [ "$APPLY" = "apply" ]; then
        if [ "$MODE" = "local" ]; then
            gittuf policy stage --local-only
            gittuf policy apply --local-only
        else
            gittuf policy stage
            gittuf policy apply
        fi
    fi
}

use_key() {
    git config --local user.signingkey "../keys/$1"
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
    [ "$FAIL" -eq 0 ] # if there are any failed cases the script returns fail
}