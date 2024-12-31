
#!/bin/bash

set -euo pipefail

# Paths
SERVER_DIR=/tmp/git-server
CLIENT_DIR=/tmp/git-client
LOG_FILE=/tmp/git-remote-gittuf.log
SSH_SERVER_DIR=/tmp/ssh-server
SSH_KEY=/tmp/test_ssh_key
SSH_PORT=2222

# Cleanup
echo "Cleaning up old test directories..."
rm -rf "$SERVER_DIR" "$CLIENT_DIR" "$SSH_SERVER_DIR" "$LOG_FILE"

# Set up logging
export GITTUF_LOG_FILE="$LOG_FILE"
echo "Logs will be written to $GITTUF_LOG_FILE"

# Initialize mock HTTP Git server
echo "Initializing HTTP Git server..."
mkdir -p "$SERVER_DIR"
git init --bare "$SERVER_DIR/test-repo.git"

# Initialize local client
echo "Setting up local client repository..."
mkdir -p "$CLIENT_DIR"
cd "$CLIENT_DIR"
git init
git remote add origin gittuf::file://$SERVER_DIR/test-repo.git

# Test Fetch Workflow
echo "Testing Fetch Workflow..."
touch initial-file
git add initial-file
git commit -m "Initial commit"
git push origin main
git fetch origin
git branch -r | grep origin/main
echo "Fetch Workflow Passed!"

# Test Push Workflow
echo "Testing Push Workflow..."
echo "Adding a new file for push test..." > new-file
git add new-file
git commit -m "Add new file"
git push origin main
echo "Push Workflow Passed!"

# Set up SSH server
echo "Setting up SSH server..."
mkdir -p "$SSH_SERVER_DIR"
git init --bare "$SSH_SERVER_DIR/test-repo.git"
ssh-keygen -q -N "" -f "$SSH_KEY" <<<y >/dev/null
eval "$(ssh-agent -s)"
ssh-add "$SSH_KEY"

# Start SSH server in Docker
echo "Starting SSH server container..."
docker run --rm -d \
  --name ssh-server \
  -v "$SSH_SERVER_DIR:/home/git/repo" \
  -v "$SSH_KEY.pub:/home/git/.ssh/authorized_keys:ro" \
  -p $SSH_PORT:22 \
  rastasheep/ubuntu-sshd

# Configure SSH remote
echo "Adding SSH remote..."
git remote add ssh-origin gittuf::ssh://git@localhost:$SSH_PORT/repo/test-repo.git

# Test SSH Fetch Workflow
echo "Testing SSH Fetch Workflow..."
git fetch ssh-origin
git branch -r | grep ssh-origin/main
echo "SSH Fetch Workflow Passed!"

# Test SSH Push Workflow
echo "Testing SSH Push Workflow..."
echo "Adding another new file for SSH push test..." > ssh-new-file
git add ssh-new-file
git commit -m "Add new file via SSH"
git push ssh-origin main
echo "SSH Push Workflow Passed!"

# Cleanup Docker container
echo "Stopping SSH server container..."
docker stop ssh-server

# Log Validation
echo "Validating logs for refs/gittuf/..."
grep "refs/gittuf/" "$LOG_FILE" || echo "No refs/gittuf/ found in logs. Check your implementation."

# Final output
echo "All tests passed successfully!"
echo "Log file content:"
cat "$LOG_FILE"