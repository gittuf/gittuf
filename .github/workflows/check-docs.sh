#!/bin/bash

set -euo pipefail

make generate
if [[ $(git --no-pager diff) ]] ; then
    echo "Please re-generate CLI docs"
    exit 1
fi
