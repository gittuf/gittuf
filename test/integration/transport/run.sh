#!/usr/bin/env bash
# End-to-end integration tests for the gittuf transport (curl + SSH).
set -uo pipefail

REPO_ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
WORKDIR="$(mktemp -d)"
FAILURES=0

# Isolate git config from the developer's environment so a global
# user.signingkey / commit.gpgsign can't interfere with the test repos.
export GIT_CONFIG_GLOBAL="$WORKDIR/gitconfig-empty"
export GIT_CONFIG_SYSTEM=/dev/null
touch "$GIT_CONFIG_GLOBAL"

cleanup() {
  [[ -n "${HTTP_PID:-}" ]] && kill "$HTTP_PID" 2>/dev/null
  [[ -n "${SSHD_PID:-}" ]] && kill "$SSHD_PID" 2>/dev/null
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; FAILURES=$((FAILURES+1)); }

init_local_repo() {
  local dir="$1"
  git init --initial-branch=main "$dir" >/dev/null
  pushd "$dir" >/dev/null
  git config user.name "Integration Test"
  git config user.email "test@example.com"
  git config commit.gpgsign false
  git config tag.gpgsign false
  git config gpg.format ssh
  git commit --allow-empty -m "initial commit" >/dev/null

  git config user.signingkey "$WORKDIR/root"
  gittuf trust init -k "$WORKDIR/root" --create-rsl-entry >/dev/null
  gittuf trust add-policy-key -k "$WORKDIR/root" --policy-key "$WORKDIR/targets.pub" --create-rsl-entry >/dev/null

  git config user.signingkey "$WORKDIR/targets"
  gittuf policy init -k "$WORKDIR/targets" --create-rsl-entry >/dev/null
  gittuf policy apply -k "$WORKDIR/targets" --local-only --create-rsl-entry >/dev/null
  popd >/dev/null
}

ssh-keygen -t ed25519 -f "$WORKDIR/root" -N "" -q
ssh-keygen -t ed25519 -f "$WORKDIR/targets" -N "" -q

HTTPBACKEND_BIN="$WORKDIR/httpbackend"
go build -o "$HTTPBACKEND_BIN" "$REPO_ROOT_DIR/test/integration/transport/httpbackend"

# HTTP (curl handler) scenarios
HTTP_REMOTE_ROOT="$WORKDIR/http-remote"
mkdir -p "$HTTP_REMOTE_ROOT"
git init --bare "$HTTP_REMOTE_ROOT/repo.git" >/dev/null
git -C "$HTTP_REMOTE_ROOT/repo.git" config http.receivepack true

"$HTTPBACKEND_BIN" "$HTTP_REMOTE_ROOT" "127.0.0.1:8090" &
HTTP_PID=$!

for i in $(seq 1 20); do
  if curl -s -o /dev/null "http://127.0.0.1:8090/"; then
    break
  fi
  sleep 0.5
done

HTTP_LOCAL="$WORKDIR/http-local"
init_local_repo "$HTTP_LOCAL"

pushd "$HTTP_LOCAL" >/dev/null
git remote add origin "gittuf::http://127.0.0.1:8090/repo.git"

if git push origin main "refs/gittuf/*:refs/gittuf/*" >/tmp/http-push.log 2>&1; then
  pass "[http] push main + all gittuf refs together"
else
  fail "[http] push main + all gittuf refs together"; cat /tmp/http-push.log
fi
popd >/dev/null

HTTP_CLONE="$WORKDIR/http-clone"
if git clone "gittuf::http://127.0.0.1:8090/repo.git" "$HTTP_CLONE" >/tmp/http-clone.log 2>&1; then
  if git -C "$HTTP_CLONE" show-ref | grep -q "refs/gittuf/policy"; then
    pass "[http] clone syncs remote gittuf refs"
  else
    fail "[http] clone did not sync refs/gittuf/policy"
  fi
else
  fail "[http] clone from remote failed"; cat /tmp/http-clone.log
fi

# Push the RSL ref standalone, before it exists on the remote.
pushd "$HTTP_LOCAL" >/dev/null
if git push origin refs/gittuf/reference-state-log:refs/gittuf/reference-state-log >/tmp/http-rsl.log 2>&1; then
  pass "[http] standalone RSL ref push"
else
  fail "[http] standalone RSL ref push"; cat /tmp/http-rsl.log
fi
popd >/dev/null

kill "$HTTP_PID" 2>/dev/null; wait "$HTTP_PID" 2>/dev/null; HTTP_PID=""

# SSH scenarios
SSHD_DIR="$WORKDIR/sshd"
SSH_REMOTE_ROOT="$WORKDIR/ssh-remote"
mkdir -p "$SSH_REMOTE_ROOT"
git init --bare "$SSH_REMOTE_ROOT/repo.git" >/dev/null
git -C "$SSH_REMOTE_ROOT/repo.git" config http.receivepack true

SSH_PORT="$("$REPO_ROOT_DIR/test/integration/transport/setup-sshd.sh" "$SSHD_DIR" "$SSH_REMOTE_ROOT" | tail -1)"
SSHD_PID="$(cat "$SSHD_DIR/sshd.pid")"

export GIT_SSH_COMMAND="ssh -p ${SSH_PORT} -i ${SSHD_DIR}/client_key -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o SendEnv=GIT_PROTOCOL"

SSH_LOCAL="$WORKDIR/ssh-local"
init_local_repo "$SSH_LOCAL"

pushd "$SSH_LOCAL" >/dev/null
git remote add origin "gittuf::ssh://127.0.0.1:${SSH_REMOTE_ROOT}/repo.git"

if timeout 30 git push origin main "refs/gittuf/*:refs/gittuf/*" >/tmp/ssh-push.log 2>&1; then
  pass "[ssh] push main + all gittuf refs together"
else
  fail "[ssh] push main + all gittuf refs together"; cat /tmp/ssh-push.log
fi
popd >/dev/null

SSH_CLONE="$WORKDIR/ssh-clone"
if timeout 30 git clone "gittuf::ssh://127.0.0.1:${SSH_REMOTE_ROOT}/repo.git" "$SSH_CLONE" >/tmp/ssh-clone.log 2>&1; then
  if git -C "$SSH_CLONE" show-ref | grep -q "refs/gittuf/policy"; then
    pass "[ssh] clone syncs remote gittuf refs"
  else
    fail "[ssh] clone did not sync refs/gittuf/policy"
  fi
else
  fail "[ssh] clone from remote failed"; cat /tmp/ssh-clone.log
fi

pushd "$SSH_LOCAL" >/dev/null
if timeout 30 git push origin refs/gittuf/reference-state-log:refs/gittuf/reference-state-log >/tmp/ssh-rsl.log 2>&1; then
  pass "[ssh] standalone RSL ref push"
else
  fail "[ssh] standalone RSL ref push"; cat /tmp/ssh-rsl.log
fi
popd >/dev/null

kill "$SSHD_PID" 2>/dev/null; SSHD_PID=""

if [[ "$FAILURES" -gt 0 ]]; then
  echo "$FAILURES scenario(s) failed"
  exit 1
fi
echo "all scenarios passed"
