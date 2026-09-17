#!/usr/bin/env bash
# Copyright The gittuf Authors
# SPDX-License-Identifier: Apache-2.0
#
# Pins how previously released gittuf clients behave on repositories written
# by the current tree. Scenario 1: a repository whose entries are all plain
# reference entries is fully readable by every listed release. Scenario 2: a
# repository containing one bulk reference entry makes every listed release
# fail closed, and the oldest release must not silently read it as a single
# update. Scenario 3: entries written by each release are readable by the
# current tree. Scenario 4: the optional ref qualifier on an annotation is
# ignored by every listed release rather than breaking them.
set -euo pipefail

# Run from the repository root so `go install .` builds the current tree
# regardless of the caller's working directory.
cd "$(dirname "$0")/.."

fail() { echo "FAIL: $*" >&2; exit 1; }

command -v ssh-keygen >/dev/null || fail "ssh-keygen is required"

VERSIONS=${COMPAT_VERSIONS:-"v0.8.1 v0.14.1 v0.16.0"}
GO=${GO:-go}
WORK=$(mktemp -d)
# Set KEEP_WORK=1 to inspect the scratch repositories after a failure.
if [ "${KEEP_WORK:-}" = "1" ]; then
  echo "keeping work directory $WORK"
else
  trap 'rm -rf "$WORK"' EXIT
fi
BIN="$WORK/bin"
mkdir -p "$BIN"

echo "building current tree"
GOBIN="$BIN" $GO install . >/dev/null
mv "$BIN/gittuf" "$BIN/gittuf-current"

for v in $VERSIONS; do
  echo "installing gittuf@$v"
  GOBIN="$BIN" $GO install "github.com/gittuf/gittuf@$v" >/dev/null
  mv "$BIN/gittuf" "$BIN/gittuf-$v"
done

ZERO=0000000000000000000000000000000000000000

# Every listed release signs RSL entries and v0.8.1 has no unsigned mode, so
# each scratch repository gets an SSH signing key.
ssh-keygen -q -t ed25519 -N "" -f "$WORK/signing-key"

new_repo() {
  local dir="$1"
  # The listed releases predate gittuf's SHA-256 support, so the scratch
  # repositories are pinned to SHA-1.
  git init -q -b main --object-format=sha1 "$dir"
  git -C "$dir" config user.name compat
  git -C "$dir" config user.email compat@example.com
  git -C "$dir" config gpg.format ssh
  git -C "$dir" config user.signingkey "$WORK/signing-key"
  git -C "$dir" config commit.gpgsign false
  git -C "$dir" commit -q --allow-empty -m init
  git -C "$dir" branch feature
}

# record_with runs `rsl record` for a release. Releases before v0.9.0 have no
# --local-only flag and never contact a remote, so the flag is dropped when
# the release rejects it. On failure the second attempt's combined output is
# printed so the caller can report why the release refused to record.
record_with() {
  local bin="$1" dir="$2" ref="$3"
  if (cd "$dir" && "$bin" rsl record "$ref" --local-only >/dev/null 2>&1); then
    return 0
  fi
  (cd "$dir" && "$bin" rsl record "$ref" 2>&1)
}

# write_rsl_entry appends a commit carrying the given message to the RSL and
# echoes its ID. Entries that no gittuf command can produce are written this
# way, in the exact wire format the current writer produces.
write_rsl_entry() {
  local dir="$1" msgfile="$2"
  local tree parent id
  tree=$(git -C "$dir" hash-object -t tree /dev/null)
  parent=$(git -C "$dir" rev-parse refs/gittuf/reference-state-log)
  id=$(git -C "$dir" commit-tree "$tree" -p "$parent" -F "$msgfile")
  git -C "$dir" update-ref refs/gittuf/reference-state-log "$id"
  echo "$id"
}

# pem_message renders an annotation message the way the writer's PEM encoder
# does. Messages are kept short enough to fit the encoder's 64 column line.
pem_message() {
  printf -- '-----BEGIN MESSAGE-----\n%s\n-----END MESSAGE-----' "$(printf '%s' "$1" | base64 | tr -d '\n')"
}

echo "scenario 1: current tree writes reference entries, every release reads them"
R1="$WORK/s1"; new_repo "$R1"
(cd "$R1" && "$BIN/gittuf-current" rsl record refs/heads/main --local-only >/dev/null)
(cd "$R1" && "$BIN/gittuf-current" rsl record refs/heads/feature --local-only >/dev/null)
for v in $VERSIONS; do
  out=$(cd "$R1" && "$BIN/gittuf-$v" rsl log 2>&1) || fail "$v cannot read per-ref entries: $out"
  grep -q "refs/heads/main" <<<"$out" || fail "$v log missing main"
  grep -q "refs/heads/feature" <<<"$out" || fail "$v log missing feature"
done

echo "scenario 2: a bulk entry makes every release fail closed"
R2="$WORK/s2"; new_repo "$R2"
(cd "$R2" && "$BIN/gittuf-current" rsl record refs/heads/main --local-only >/dev/null)
# No gittuf command records two references in one call: `rsl record` takes a
# single reference, and the only caller of the bulk writer is the
# git-remote-gittuf transport during a push, which needs an SSH or HTTP
# server. The entry is therefore written by hand, in the exact wire format
# the current writer produces.
MSG="$WORK/bulk.msg"
printf 'RSL Bulk Reference Entry\n\nrefs/heads/aaa: %s\nrefs/heads/zzz: %s\n\nnumber: 2' "$ZERO" "$ZERO" > "$MSG"
BULK=$(write_rsl_entry "$R2" "$MSG")
for v in $VERSIONS; do
  if out=$(cd "$R2" && "$BIN/gittuf-$v" rsl log 2>&1); then
    fail "$v accepted a bulk entry: $out"
  fi
  grep -q "invalid format or is of unexpected type" <<<"$out" || fail "$v failed for an unexpected reason: $out"
  if grep -q "refs/heads/zzz" <<<"$out"; then
    fail "$v silently read the bulk entry as a single update"
  fi
done
out=$(cd "$R2" && "$BIN/gittuf-current" rsl log 2>&1) || fail "current tree cannot read its own bulk entry: $out"
grep -q "bulk entry $BULK" <<<"$out" || fail "current tree did not render the bulk entry"

echo "scenario 3: each release writes, current tree reads"
for v in $VERSIONS; do
  R3="$WORK/s3-$v"; new_repo "$R3"
  out=$(record_with "$BIN/gittuf-$v" "$R3" refs/heads/main) || fail "$v could not record an entry: $out"
  out=$(cd "$R3" && "$BIN/gittuf-current" rsl log 2>&1) || fail "current tree cannot read entries from $v: $out"
  grep -q "refs/heads/main" <<<"$out" || fail "current tree log missing main for $v"
done

echo "scenario 4: an annotation's ref qualifier does not break any release"
# A qualifier is only legal on a bulk entry, so a release that meets one also
# meets the bulk entry it qualifies and fails closed there: it can never act
# on the annotation's narrowed skip. 4a pins that. 4b isolates the ref key
# itself by putting it on an annotation for a plain reference entry, the shape
# a future writer would produce if qualifiers were extended to those entries,
# and pins that releases parse it and silently drop the qualifier, widening
# the skip to the whole entry.
R4A="$WORK/s4a"; new_repo "$R4A"
(cd "$R4A" && "$BIN/gittuf-current" rsl record refs/heads/main --local-only >/dev/null)
MSG="$WORK/bulk-4a.msg"
printf 'RSL Bulk Reference Entry\n\nrefs/heads/aaa: %s\nrefs/heads/zzz: %s\n\nnumber: 2' "$ZERO" "$ZERO" > "$MSG"
BULK4A=$(write_rsl_entry "$R4A" "$MSG")
MSG="$WORK/annotation-4a.msg"
printf 'RSL Annotation Entry\n\nentryID: %s\nref: refs/heads/aaa\nskip: true\nnumber: 3\n%s' \
  "$BULK4A" "$(pem_message 'skip aaa only')" > "$MSG"
ANN4A=$(write_rsl_entry "$R4A" "$MSG")
for v in $VERSIONS; do
  if out=$(cd "$R4A" && "$BIN/gittuf-$v" rsl log 2>&1); then
    fail "$v accepted a qualified annotation's bulk entry: $out"
  fi
  grep -q "invalid format or is of unexpected type" <<<"$out" || fail "$v failed for an unexpected reason: $out"
  if grep -q "refs/heads/aaa" <<<"$out"; then
    fail "$v acted on a qualified annotation instead of failing closed"
  fi
done
out=$(cd "$R4A" && "$BIN/gittuf-current" rsl log 2>&1) || fail "current tree cannot read its own qualified annotation: $out"
grep -q "bulk entry $BULK4A" <<<"$out" || fail "current tree did not render the bulk entry"
grep -q "Annotation ID: $ANN4A" <<<"$out" || fail "current tree did not render the annotation"
grep -q "Refs:          refs/heads/aaa" <<<"$out" || fail "current tree did not render the annotation's qualifier"

R4B="$WORK/s4b"; new_repo "$R4B"
(cd "$R4B" && "$BIN/gittuf-current" rsl record refs/heads/main --local-only >/dev/null)
ENTRY4B=$(git -C "$R4B" rev-parse refs/gittuf/reference-state-log)
MSG="$WORK/annotation-4b.msg"
printf 'RSL Annotation Entry\n\nentryID: %s\nref: refs/heads/main\nskip: true\nnumber: 2\n%s' \
  "$ENTRY4B" "$(pem_message 'skip main')" > "$MSG"
write_rsl_entry "$R4B" "$MSG" >/dev/null
for v in $VERSIONS; do
  out=$(cd "$R4B" && "$BIN/gittuf-$v" rsl log 2>&1) || fail "$v cannot read an annotation carrying a ref: $out"
  grep -q "entry $ENTRY4B (skipped)" <<<"$out" || fail "$v did not apply the annotation's skip: $out"
  grep -q "skip main" <<<"$out" || fail "$v did not decode the annotation message: $out"
done
out=$(cd "$R4B" && "$BIN/gittuf-current" rsl log 2>&1) || fail "current tree cannot read the annotation: $out"
grep -q "entry $ENTRY4B (skipped)" <<<"$out" || fail "current tree did not render the entry as skipped: $out"

echo "compat matrix passed for: $VERSIONS"
