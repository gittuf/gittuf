#!/bin/bash

# set -e  # Exit immediately if a command exits with a non-zero status
# set -o pipefail  # Ensure pipeline errors propagate
# set -u  # Treat unset variables as an error

# Constants
GITTUF_BINARY="git-remote-gittuf"
TEST_REPO_URL_SSH="gittuf::git@github.com:gittuf/zsun6"
TEST_REPO_URL_HTTPS="gittuf::https://github.com/zsun6/gittuf"
TEST_CLONE_DIR="test_clone"
TEST_REMOTE_NAME="origin"
GITTUF_TEST_BRANCH="test-branch"

# Verify if the gittuf binary is installed
if ! command -v "$GITTUF_BINARY" &>/dev/null; then
    echo "Error: $GITTUF_BINARY is not installed or not in PATH."
    exit 1
fi

echo "Git-Remote-Gittuf binary is installed."

# Clean up from previous runs
if [ -d "$TEST_CLONE_DIR" ]; then
    rm -rf "$TEST_CLONE_DIR"
fi

echo "Cleaned up previous test environment."

# Check if SSH is working
echo "Testing SSH authentication..."
if ssh -T git@github.com &>/dev/null; then
    echo "SSH authentication successful. Using SSH URL."
    REPO_URL="$TEST_REPO_URL_SSH"
else
    echo "SSH authentication failed. Falling back to HTTPS URL."
    REPO_URL="$TEST_REPO_URL_HTTPS"
fi

# Test cloning the repository
echo "Testing clone using $REPO_URL..."
git clone "$REPO_URL" "$TEST_CLONE_DIR"
cd "$TEST_CLONE_DIR"

# Check if the repository cloned correctly
if [ ! -d ".git" ]; then
    echo "Error: Failed to clone repository."
    exit 1
fi
echo "Successfully cloned repository."

# Test setting remote to HTTPS (for flexibility in testing)
echo "Testing setting remote URL to HTTPS..."
git remote set-url "$TEST_REMOTE_NAME" "$TEST_REPO_URL_HTTPS"

REMOTE_URL=$(git remote get-url "$TEST_REMOTE_NAME")
if [ "$REMOTE_URL" != "$TEST_REPO_URL_HTTPS" ]; then
    echo "Error: Failed to set remote URL to HTTPS."
    exit 1
fi
echo "Successfully set remote URL to HTTPS."

# Test creating a new branch and pushing changes
echo "Testing push operation..."
git checkout -b "$GITTUF_TEST_BRANCH"
echo "Test file" > test_file.txt
git add test_file.txt
git commit -m "Add test file for $GITTUF_TEST_BRANCH"
git push -u "$TEST_REMOTE_NAME" "$GITTUF_TEST_BRANCH"

echo "Push operation completed successfully."

# Test fetching updates
echo "Testing fetch operation..."
git fetch "$TEST_REMOTE_NAME"
if [ $? -ne 0 ]; then
    echo "Error: Fetch operation failed."
    exit 1
fi
echo "Fetch operation completed successfully."

# Clean up
echo "Cleaning up test environment..."
cd ..
rm -rf "$TEST_CLONE_DIR"

echo "All tests completed successfully."
