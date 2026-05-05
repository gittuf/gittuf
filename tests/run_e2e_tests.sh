# gittuf E2E Test: RUN ALL

PASS=0
FAIL=0

for file in "$(dirname "$0")"/gittuf*.sh; do
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