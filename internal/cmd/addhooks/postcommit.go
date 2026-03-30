// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

var postCommitScript = []byte(`#!/bin/sh
set -e

# Check if gittuf is available
if ! command -v gittuf > /dev/null 2>&1
then
    echo "Error: gittuf could not be found in PATH"
    echo "Please install gittuf from: https://github.com/gittuf/gittuf/releases/latest"
    exit 1
fi

echo "Running gittuf post-commit processing..."

# Get the current branch
current_branch=$(git symbolic-ref --short HEAD 2>/dev/null || echo "HEAD")

echo "Post-commit hook completed for branch: $current_branch"
echo "Remember to run 'gittuf rsl record $current_branch' before pushing"
echo "Or use 'gittuf add-hooks' to automate RSL management"
`)
