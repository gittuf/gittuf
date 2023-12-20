package addhooks

var prePushScript = []byte(`#!/bin/sh
set -e

remote="$1"
url="$2"

if ! command -v gittuf > /dev/null
then
    echo "gittuf could not be found"
    echo "Download from: https://github.com/gittuf/gittuf/releases/latest"
    exit 1
fi

echo "Pulling RSL from ${remote}."
gittuf rsl remote pull ${remote}
echo "Creating new RSL record for HEAD."
gittuf rsl record HEAD
echo "Pushing RSL to ${remote}."
gittuf rsl remote push ${remote}
`)
