#!/usr/bin/env bash
# gittuf E2E Test: RUN ALL
set -euo pipefail
shopt -s nullglob

PASS=0
FAIL=0

if [ -n "${GITTUF_BIN:-}" ]; then
    if [ ! -f "$GITTUF_BIN" ] && [ -f "${GITTUF_BIN}.exe" ]; then
        GITTUF_BIN="${GITTUF_BIN}.exe"
    elif [ ! -f "$GITTUF_BIN" ] && [ -f "${GITTUF_BIN%.exe}" ]; then
        GITTUF_BIN="${GITTUF_BIN%.exe}"
    fi
else
    if [ -f "./dist/gittuf" ]; then
        GITTUF_BIN="$(pwd)/dist/gittuf"
    elif [ -f "./dist/gittuf.exe" ]; then
        GITTUF_BIN="$(pwd)/dist/gittuf.exe"
    else
        GITTUF_BIN="gittuf"
    fi
fi
export GITTUF_BIN

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
