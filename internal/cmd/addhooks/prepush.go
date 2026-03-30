// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

var prePushScript = []byte(`#!/bin/sh
set -e

remote="$1"
url="$2"

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

# Read stdin to get the refs being pushed
while read local_ref local_sha remote_ref remote_sha
do
    # Skip if no refs are being pushed
    if [ "$local_sha" = "0000000000000000000000000000000000000000" ]; then
        continue
    fi
    
    echo "Processing ref: $local_ref -> $remote_ref"
    
    # Create RSL entry for the ref being pushed
    echo "Creating RSL record for ${local_ref}..."
    if ! gittuf rsl record "${local_ref}" --local-only; then
        handle_error "Failed to create RSL record for ${local_ref}"
    fi
    
    # Sync gittuf metadata with remote
    echo "Syncing gittuf metadata with ${remote}..."
    if ! gittuf sync "${remote}" 2>/dev/null; then
        echo "Warning: Could not sync with remote, this may be expected for new repositories"
    fi
done

echo "gittuf pre-push hook completed successfully"
`)
