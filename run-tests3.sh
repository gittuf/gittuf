#!/bin/bash

# set -e  # Exit immediately if a command exits with a non-zero status
set -o pipefail  # Ensure pipeline errors propagate
set -u  # Treat unset variables as an error

# Constants
GITTUF_BINARY="git-remote-gittuf"
TEST_REPO_URL_SSH="gittuf::git@github.com:gittuf/zsun6"
TEST_REPO_URL_HTTPS="gittuf::https://github.com/zsun6/gittuf-remote-test_repo"
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

REPO_URL="$TEST_REPO_URL_HTTPS"

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

# Test first fetch
echo "Testing fetch operation..."
git fetch "$TEST_REMOTE_NAME"
if [ $? -ne 0 ]; then
    echo "Error: Fetch operation failed."
    exit 1
fi
# Navigate to the folder where the repo is cloned
REPO_DIR="repo2"

# Check if the directory exists
if [ -d "$REPO_DIR" ]; then
    echo "Repository folder '$REPO_DIR' found."
    # Check for the README file inside the repo directory
    README_FILE="$REPO_DIR/README.md"
    if [ -f "$README_FILE" ]; then
        echo "README file found. Printing contents:"
        cat "$README_FILE"
    else
        echo "README file not found in '$REPO_DIR'."
    fi
else
    echo "Repository folder '$REPO_DIR' does not exist."
    exit 1
fi
echo "Fetch operation completed successfully."

# Test creating a new branch and pushing changes
echo "Testing push operation..."
git checkout -b "$GITTUF_TEST_BRANCH"
echo "This is auto test" > written_by_autotest.txt
git add written_by_autotest.txt
git commit -m "Add test file for $GITTUF_TEST_BRANCH"
git push -u "$TEST_REMOTE_NAME" "$GITTUF_TEST_BRANCH" --force

echo "Push operation completed successfully."

README_FILE_2="written_by_autotest.txt"
# Test fetching updates
echo "Testing fetch operation..."
git fetch "$TEST_REMOTE_NAME"
if [ $? -ne 0 ]; then
    echo "Error: Fetch operation failed."
    exit 1
fi
if [ -f "$README_FILE_2" ]; then
    echo "pushed file found. Printing contents:"
    cat "$README_FILE_2"
else
    echo "README file not found in '$REPO_DIR'."
fi
echo "Fetch operation completed successfully."

# Clean up
echo "Cleaning up test environment..."
cd ..
rm -rf "$TEST_CLONE_DIR"

echo "All tests completed successfully."
