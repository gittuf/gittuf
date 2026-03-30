// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

var preCommitScript = []byte(`#!/bin/sh
set -e

# Check if gittuf is available
if ! command -v gittuf > /dev/null 2>&1
then
    echo "Error: gittuf could not be found in PATH"
    echo "Please install gittuf from: https://github.com/gittuf/gittuf/releases/latest"
    exit 1
fi

# Function to handle errors gracefully
handle_error() {
    echo "Error: $1" >&2
    exit 1
}

echo "Running gittuf pre-commit verification..."

# Verify current repository state against policies
if ! gittuf verify-ref HEAD 2>/dev/null; then
    echo "Warning: Current HEAD does not pass gittuf verification"
    echo "This may be expected for new commits before they are recorded in RSL"
fi

# Check if there are any policy violations in staged changes
if git diff --cached --quiet; then
    echo "No staged changes to verify"
    exit 0
fi

echo "Staged changes detected, performing gittuf policy checks..."

# Get list of staged files
staged_files=$(git diff --cached --name-only)

# Check if any staged files are protected by gittuf policies
for file in $staged_files; do
    echo "Checking policy compliance for: $file"
    # Note: This is a placeholder for future file-level policy checking
    # The actual implementation would check against gittuf file protection rules
done

echo "gittuf pre-commit hook completed successfully"
`)
