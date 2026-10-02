#!/usr/bin/env bash
# Sets up a throwaway sshd instance for transport integration tests.
# Prints the port it's listening on and the path to the client SSH key to use.
set -euo pipefail

SSHD_DIR="$1"   # working directory for keys/config/pidfile
REPO_ROOT="$2"  # directory to expose as the git repo root over SSH

mkdir -p "$SSHD_DIR"

# host key
ssh-keygen -t ed25519 -f "$SSHD_DIR/host_key" -N "" -q

# client key + authorize it
ssh-keygen -t ed25519 -f "$SSHD_DIR/client_key" -N "" -q
cat "$SSHD_DIR/client_key.pub" > "$SSHD_DIR/authorized_keys"
chmod 600 "$SSHD_DIR/authorized_keys"

PORT=2222

cat > "$SSHD_DIR/sshd_config" << CONF
Port ${PORT}
ListenAddress 127.0.0.1
HostKey ${SSHD_DIR}/host_key
AuthorizedKeysFile ${SSHD_DIR}/authorized_keys
PubkeyAuthentication yes
PasswordAuthentication no
StrictModes no
UsePAM no
PidFile ${SSHD_DIR}/sshd.pid
AcceptEnv GIT_PROTOCOL
LogLevel VERBOSE
Subsystem sftp internal-sftp
CONF

/usr/sbin/sshd -f "$SSHD_DIR/sshd_config" -D -e > "$SSHD_DIR/sshd.log" 2>&1 < /dev/null &
SSHD_PID=$!
disown
echo "$SSHD_PID" > "$SSHD_DIR/sshd.pid"

sleep 1

echo "$PORT"
