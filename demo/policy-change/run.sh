#!/usr/bin/env bash
# Copyright The gittuf Authors
# SPDX-License-Identifier: Apache-2.0
#
# SKETCH (cedar-integration-sketch): milestone 7, policy-change analysis.
# Builds real gittuf rule files with the CLI, converts each version to the old
# POC's Doc format, compiles them with the POC's gittuf mode, and asks
# cedar-policy-symcc + cvc5 whether the change weakens refs/heads/main.
# Then replays the solver's finding in gittuf itself.
#
# Needs: Go, Rust >= 1.89, python3, and a scratch copy of the POC with the
# rulefile_diff binary (see below). Usage: run.sh <scratch-poc-dir>
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
POC=$1
SKETCH=${SKETCH:-$HOME/gittuf-sketch}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/gittuf-m7.XXXXXX")
GITTUF="$WORK/gittuf"
unset GITHUB_TOKEN GITTUF_DEV GITTUF_DECIDER GITTUF_CEDAR_POLICY
export CVC5=${CVC5:-$POC/smt/.tools/cvc5-1.2.1/bin/cvc5}

(cd "$SKETCH" && CGO_ENABLED=0 go build -trimpath -o "$GITTUF" .)
(cd "$POC/smt" && cargo build --quiet --release --bin rulefile_diff)
DIFF="$POC/smt/target/release/rulefile_diff"

cd "$WORK" && mkdir keys repo
for k in root targets alice bob carol; do ssh-keygen -q -t ecdsa -N "" -f "keys/$k"; done
cd repo
git init -q -b main
git config --local gpg.format ssh
git config --local commit.gpgsign true
git config --local user.name gittuf-demo
git config --local user.email gittuf.demo@example.com
git config --local user.signingkey ../keys/alice
q() { "$@" > /dev/null; }
apply() { q "$GITTUF" policy stage --local-only; q "$GITTUF" policy apply --local-only; }
snapshot() { git show refs/gittuf/policy:metadata/targets.json > "$WORK/$1.targets.json"; }

q "$GITTUF" trust init -k ../keys/root
q "$GITTUF" trust add-policy-key -k ../keys/root --policy-key ../keys/targets.pub
q "$GITTUF" policy init -k ../keys/targets
for p in alice bob carol; do q "$GITTUF" policy add-person -k ../keys/targets --person-ID "$p" --public-key "../keys/$p.pub"; done
q "$GITTUF" policy add-rule -k ../keys/targets --rule-name protect-main --rule-pattern git:refs/heads/main --threshold 2 --authorize alice --authorize bob
apply
snapshot v1-before
echo init > README.md && git add README.md && git commit -q -m init && q "$GITTUF" rsl record main --local-only

# Benign change: a rule for refs/heads/dev only.
q "$GITTUF" policy add-rule -k ../keys/targets --rule-name protect-dev --rule-pattern git:refs/heads/dev --threshold 1 --authorize carol
apply
snapshot v2-benign

# Replay, part 1: under v2, carol alone pushes to main.
carol_push() {
	git config --local user.signingkey ../keys/carol
	echo "carol $1" >> README.md && git commit -q -am "carol $1" && q "$GITTUF" rsl record main --local-only
	if "$GITTUF" verify-ref --latest-only main > /dev/null 2>&1; then echo PASS; else echo FAIL; fi
}
REPLAY_V2=$(carol_push v2)

# Weakening change: a broad rule for all branches that trusts carol alone.
q "$GITTUF" policy add-rule -k ../keys/targets --rule-name protect-branches --rule-pattern 'git:refs/heads/*' --threshold 1 --authorize carol
apply
snapshot v3-weakening

for v in v1-before v2-benign v3-weakening; do
	python3 -I "$HERE/gittuf_to_doc.py" "$WORK/$v.targets.json" > "$WORK/$v.doc.json"
	(cd "$POC/go" && go run ./cmd/gcedar compile -mode gittuf -out "$WORK/$v" "$WORK/$v.doc.json" > /dev/null)
done

check() {
	echo
	echo "== $1: $2 -> $3"
	echo "   before rules: $(python3 -c 'import json,sys; print([(r["name"],r["pattern"],r["threshold"],r["members"]) for r in json.load(open(sys.argv[1]))["rules"]])' "$WORK/$2.doc.json")"
	echo "   after rules:  $(python3 -c 'import json,sys; print([(r["name"],r["pattern"],r["threshold"],r["members"]) for r in json.load(open(sys.argv[1]))["rules"]])' "$WORK/$3.doc.json")"
	local status=0
	"$DIFF" "$WORK/$2" "$WORK/$3" | sed 's/^/   /' || status=$?
	echo "   solver verdict: $([ "$status" -eq 0 ] && echo 'no weakening on main' || echo 'WEAKENS main')"
}
time check "benign change (add protect-dev)" v1-before v2-benign
time check "weakening change (add refs/heads/* 1-of-{carol})" v2-benign v3-weakening

echo
echo "== replay in gittuf (plain, no Cedar): carol alone pushes to main"
echo "   under v2 (before the weakening change): $REPLAY_V2 (expected FAIL)"
echo "   under v3 (after the weakening change):  $(carol_push v3) (expected PASS: the solver's counterexample is real)"
echo "   work dir: $WORK"
