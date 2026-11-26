#!/usr/bin/env bash
# Copyright The gittuf Authors
# SPDX-License-Identifier: Apache-2.0

# SKETCH (cedar-integration-sketch, not for merge): demo of the experimental
# Cedar AND-overlay. Part A shows the integration is gated and fail-closed.
# Part B shows rules plain gittuf cannot express, each run three ways:
#   1. plain gittuf (gate off)
#   2. Cedar with a permits-only policy (must match plain gittuf)
#   3. Cedar with the rule (must deny the violating change)
#
# Usage: demo/cedar-demo.sh [A] [2] [3] [4] [5]   (default: all sections)

set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
POLICIES="$ROOT/demo/cedar"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/gittuf-cedar-demo.XXXXXX")
GITTUF="$WORK/gittuf"
DECISIONS="$WORK/cedar-decisions.tsv"
unset GITHUB_TOKEN GITTUF_DEV GITTUF_DECIDER GITTUF_CEDAR_POLICY GITTUF_CEDAR_PR_ATTESTATION GITTUF_CEDAR_SIGNERS

RAN=0
MISMATCHES=0

section() { printf '\n\033[1m== %s ==\033[0m\n' "$*"; }
note() { printf '   %s\n' "$*"; }

echo "Building gittuf into $GITTUF ..."
(cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -o "$GITTUF" .)

# policy_file <name> <cedar files...>: concatenate policy files into one.
policy_file() {
	local out="$WORK/$1.cedar"
	shift
	cat "$@" > "$out"
	echo "$out"
}

PERMIT_ALL="$POLICIES/permit-all.cedar"
DENY_ALL=$(policy_file deny-all "$PERMIT_ALL" <(echo '@id("deny-all") forbid (principal, action, resource);'))
GARBAGE=$(policy_file garbage <(echo 'this is not a cedar policy'))
UNGUARDED=$(policy_file unguarded "$PERMIT_ALL" <(echo '@id("unguarded-pr") forbid (principal, action, resource) when { context.pr.number > 0 };'))

# verify <mode> [policy]: runs `gittuf verify-ref --latest-only main` in the
# requested mode and prints PASS or FAIL.
verify() {
	local mode=$1 policy=${2:-}
	local -a env_args=(env -u GITTUF_DEV -u GITTUF_DECIDER -u GITTUF_CEDAR_POLICY)
	case "$mode" in
	plain) ;;
	gate-half) env_args+=(GITTUF_DECIDER=cedar "GITTUF_CEDAR_POLICY=$policy") ;;
	cedar) env_args+=(GITTUF_DEV=1 GITTUF_DECIDER=cedar "GITTUF_CEDAR_POLICY=$policy" "GITTUF_CEDAR_LOG=$DECISIONS") ;;
	esac
	: > "$DECISIONS"
	if "${env_args[@]}" "$GITTUF" verify-ref --latest-only main > "$WORK/verify.out" 2>&1; then
		echo PASS
	else
		echo FAIL
	fi
}

# expect <description> <expected PASS|FAIL> <mode> [policy]
expect() {
	local desc=$1 want=$2
	shift 2
	local got
	got=$(verify "$@")
	RAN=$((RAN + 1))
	local mark="ok"
	if [ "$got" != "$want" ]; then
		mark="MISMATCH"
		MISMATCHES=$((MISMATCHES + 1))
	fi
	printf '   %-58s %-4s (expected %s) %s\n' "$desc" "$got" "$want" "$mark"
	if [ "$got" = FAIL ]; then
		if [ -s "$DECISIONS" ] && grep -q -v '^allow' "$DECISIONS"; then
			grep -v '^allow' "$DECISIONS" | tail -1 | awk -F'\t' '{printf "      cedar: %s %s: %s\n", $1, $2, $3}'
		else
			sed -n '$p' "$WORK/verify.out" | sed 's/^/      gittuf: /'
		fi
	fi
}

# three_way <rule policy file> <expect plain> <expect permits-only> <expect with rule>
three_way() {
	local rule=$1
	local with_rule
	with_rule=$(policy_file "with-$(basename "$rule" .cedar)" "$PERMIT_ALL" "$rule")
	expect "1. plain gittuf" "$2" plain
	expect "2. Cedar, permits-only policy" "$3" cedar "$PERMIT_ALL"
	expect "3. Cedar, permit-all + $(basename "$rule")" "$4" cedar "$with_rule"
}

# new_repo <person[:group,group]>...: creates a repository with a gittuf root,
# a primary rule file, and one person per argument. The first person makes the
# initial commit on main.
new_repo() {
	local dir
	dir=$(mktemp -d "$WORK/repo.XXXXXX")
	mkdir "$dir/keys" "$dir/repo"
	ssh-keygen -q -t ecdsa -N "" -f "$dir/keys/root"
	ssh-keygen -q -t ecdsa -N "" -f "$dir/keys/targets"
	local spec name
	for spec in "$@"; do
		name=${spec%%:*}
		ssh-keygen -q -t ecdsa -N "" -C "$name" -f "$dir/keys/$name"
	done
	cd "$dir/repo"
	git init -q -b main
	git config --local gpg.format ssh
	git config --local commit.gpgsign true
	git config --local user.name gittuf-demo
	git config --local user.email gittuf.demo@example.com
	use_key "${1%%:*}"

	"$GITTUF" trust init -k ../keys/root
	"$GITTUF" trust add-policy-key -k ../keys/root --policy-key ../keys/targets.pub
	"$GITTUF" policy init -k ../keys/targets
	local groups
	for spec in "$@"; do
		name=${spec%%:*}
		local -a args=(--person-ID "$name" --public-key "../keys/$name.pub")
		if [ "$spec" != "$name" ]; then
			groups=${spec#*:}
			args+=(--custom "cedar-groups=$groups")
		fi
		"$GITTUF" policy add-person -k ../keys/targets "${args[@]}"
	done
	apply_policy

	echo 'initial' > README.md
	git add README.md
	git commit -q -m 'Initial commit'
	"$GITTUF" rsl record main --local-only
}

use_key() { git config --local user.signingkey "../keys/$1"; }

apply_policy() {
	"$GITTUF" policy stage --local-only > /dev/null
	"$GITTUF" policy apply --local-only > /dev/null
}

# add_rule <name> <pattern> <threshold> <person>...
add_rule() {
	local name=$1 pattern=$2 threshold=$3
	shift 3
	local -a args=()
	local p
	for p in "$@"; do args+=(--authorize "$p"); done
	"$GITTUF" policy add-rule -k ../keys/targets --rule-name "$name" --rule-pattern "$pattern" --threshold "$threshold" "${args[@]}"
}

# change <rsl signer> [authorizer...]: commits on a feature branch, has each
# authorizer sign a reference authorization for merging it into main,
# fast-forwards main, and records main in the RSL signed by <rsl signer>.
# Changes stack: a compliant change is made on top of a violating one, and
# `verify-ref --latest-only` checks only the newest RSL entry for main.
change() {
	local signer=$1
	shift
	use_key "$signer"
	git checkout -q -B feature main
	echo "change by $signer, approved by $*" >> README.md
	git commit -q -am "Change by $signer"
	"$GITTUF" rsl record feature --local-only
	local a
	for a in "$@"; do
		"$GITTUF" attest authorize -k "../keys/$a" --create-rsl-entry --from-ref feature main
	done
	git checkout -q main
	git merge -q --ff-only feature
	"$GITTUF" rsl record main --local-only
}

part_a() {
	section "Part A: the integration is gated and fail-closed"
	new_repo alice bob > /dev/null 2>&1
	add_rule protect-main git:refs/heads/main 1 alice > /dev/null
	apply_policy
	change alice > /dev/null 2>&1
	note "repo: protect-main = 1-of-{alice}; alice pushed to main"
	expect "gate off (no GITTUF_DEV), deny-all policy set" PASS gate-half "$DENY_ALL"
	expect "plain gittuf" PASS plain
	expect "Cedar, permits-only" PASS cedar "$PERMIT_ALL"
	expect "Cedar, deny-all" FAIL cedar "$DENY_ALL"
	expect "Cedar, policy file missing" FAIL cedar "$WORK/does-not-exist.cedar"
	expect "Cedar, unparsable policy" FAIL cedar "$GARBAGE"
	expect "Cedar, policy failing strict validation" FAIL cedar "$UNGUARDED"
	change bob > /dev/null 2>&1
	note "bob (not authorized by protect-main) pushed to main"
	expect "plain gittuf" FAIL plain
	expect "Cedar, permits-only cannot allow what native denies" FAIL cedar "$PERMIT_ALL"
	note "full test suite under gate-off and permits-only: see ~/cedar-sketch-log.md (milestone 1)"
}

rule_2() {
	section "Part B, rule 2: a release manager must be among the signers (on top of 3-of-5)"
	new_repo R:release-managers a b c d > /dev/null 2>&1
	add_rule protect-main git:refs/heads/main 3 R a b c d > /dev/null
	apply_policy
	change a b c > /dev/null 2>&1
	note "violating change: signed by a, authorized by b and c (no release manager)"
	three_way "$POLICIES/rule2-required-group.cedar" PASS PASS FAIL
	change a R b > /dev/null 2>&1
	note "compliant change: signed by a, authorized by R and b"
	expect "Cedar, permit-all + rule2-required-group.cedar" PASS cedar "$(policy_file r2 "$PERMIT_ALL" "$POLICIES/rule2-required-group.cedar")"
	note "plain gittuf needs 6 rules for this (C(4,2)); see the log, milestone 2"
}

rule_3() {
	section "Part B, rule 3: the PR author must not approve their own PR"
	note "Runs as a Go test: approvals come from a GitHub app attestation that the CLI"
	note "can only record through the GitHub API. The PR attestation is hand-recorded (SKETCH)."
	local out status=0
	out=$(cd "$ROOT" && env -u GITHUB_TOKEN go test -count=1 -run 'TestCedarSketchDemoRule3' -v ./internal/policy/ 2>&1) || status=$?
	grep -E '^\s+zz_cedar_sketch_demo_test.go:[0-9]+: ' <<< "$out" | sed -E 's/^\s+zz_cedar_sketch_demo_test.go:[0-9]+: /   /'
	RAN=$((RAN + 1))
	if [ "$status" -ne 0 ]; then
		MISMATCHES=$((MISMATCHES + 1))
		note "MISMATCH: TestCedarSketchDemoRule3 failed"
	fi
}

rule_4() {
	section "Part B, rule 4: separation of duties, one signer from each of dev, qa, security"
	people=()
	for g in dev qa security; do
		for i in 1 2 3 4 5; do people+=("$g$i:$g"); done
	done
	new_repo "${people[@]}" > /dev/null 2>&1
	all=()
	for p in "${people[@]}"; do all+=("${p%%:*}"); done
	add_rule protect-main git:refs/heads/main 3 "${all[@]}" > /dev/null
	apply_policy
	change dev1 dev2 qa1 > /dev/null 2>&1
	note "violating change: dev1 signed, dev2 and qa1 authorized (no security)"
	three_way "$POLICIES/rule4-separation-of-duties.cedar" PASS PASS FAIL
	change dev1 qa1 security1 > /dev/null 2>&1
	note "compliant change: dev1 signed, qa1 and security1 authorized"
	expect "Cedar, permit-all + rule4-separation-of-duties.cedar" PASS cedar "$(policy_file r4 "$PERMIT_ALL" "$POLICIES/rule4-separation-of-duties.cedar")"
	note "plain gittuf needs 125 rules for this; see the log, milestone 4"
}

rule_5() {
	section "Part B (example), rule 5: forbid mallory on main although refs/heads/* admits her"
	new_repo alice bob mallory > /dev/null 2>&1
	add_rule protect-main git:refs/heads/main 1 alice bob > /dev/null
	add_rule protect-branches 'git:refs/heads/*' 1 alice bob mallory > /dev/null
	apply_policy
	change mallory > /dev/null 2>&1
	note "mallory pushed to main; protect-branches admits her"
	three_way "$POLICIES/rule5-forbid-principal.cedar" PASS PASS FAIL
	change alice > /dev/null 2>&1
	expect "alice: Cedar, permit-all + rule5-forbid-principal.cedar" PASS cedar "$(policy_file r5 "$PERMIT_ALL" "$POLICIES/rule5-forbid-principal.cedar")"

	note "plain-gittuf alternative 1 (CLI-settable): broad rule with main excluded by 8 negated patterns"
	new_repo alice bob mallory > /dev/null 2>&1
	add_rule protect-main git:refs/heads/main 1 alice bob > /dev/null
	local -a excl=()
	local pat
	for pat in '[!m]*' 'm[!a]*' 'ma[!i]*' 'mai[!n]*' 'main?*' 'm' 'ma' 'mai'; do excl+=(--rule-pattern "git:refs/heads/$pat"); done
	"$GITTUF" policy add-rule -k ../keys/targets --rule-name protect-branches --threshold 1 \
		--authorize alice --authorize bob --authorize mallory "${excl[@]}" > /dev/null
	apply_policy
	change mallory > /dev/null 2>&1
	expect "mallory on main, plain gittuf with exclusion patterns" FAIL plain
	change alice > /dev/null 2>&1
	expect "alice on main (control), same policy" PASS plain
	note "plain-gittuf alternative 2 (metadata hand-edit only; terminating is not CLI-settable):"
	(cd "$ROOT" && env -u GITHUB_TOKEN go test -count=1 -run 'TestCedarSketchRule5PlainWays/hand-edited' -v ./internal/policy/ 2>&1 |
		grep -E 'mallory on git:refs/heads/main' | sed -E 's/^.*zz_cedar_sketch_rule5_test.go:[0-9]+: /   /')
}

# Sections to run: A 2 3 4 5 (default: all).
sections=("$@")
[ ${#sections[@]} -eq 0 ] && sections=(A 2 3 4 5)
for s in "${sections[@]}"; do
	case "$s" in
	A) part_a ;;
	2 | 3 | 4 | 5) "rule_$s" ;;
	*) echo "unknown section '$s' (want A, 2, 3, 4 or 5)" >&2; exit 2 ;;
	esac
	cd "$WORK"
done

section "Result"
echo "   checks run: $RAN, mismatches: $MISMATCHES"
echo "   work dir: $WORK"
[ "$MISMATCHES" -eq 0 ]
