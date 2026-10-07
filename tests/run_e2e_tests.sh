#!/usr/bin/env bash
# gittuf E2E Test: RUN ALL
set -euo pipefail
shopt -s nullglob

PASS=0
FAIL=0

tests_dir="$(dirname "$0")"
tests=("$tests_dir"/gittuf*.sh)

if (( ${#tests[@]} == 0 )); then
    echo "No E2E tests found (expected: $tests_dir/gittuf*.sh)" >&2
    exit 1
fi

for file in "${tests[@]}"; do
    echo "========================="
    echo "Running: $file"
    echo "========================="
    if bash "$file"; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
done

echo "========================="
echo "Overall:"
echo "[PASSED]: $PASS"
echo "[FAILED]: $FAIL"
echo "========================="

[ "$FAIL" -eq 0 ]
