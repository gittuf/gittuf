#!/usr/bin/env python3
# Copyright The gittuf Authors
# SPDX-License-Identifier: Apache-2.0
#
# SKETCH (cedar-integration-sketch): convert a gittuf primary rule file
# (metadata/targets.json, a DSSE envelope as stored in refs/gittuf/policy) to
# the old POC's Doc format (gcedar compile -mode gittuf). Only git: rules are
# kept; a rule with several patterns becomes one POC rule per pattern, which is
# equivalent because gittuf treats every matching rule as an alternative.
import base64
import json
import sys


def main(path):
    with open(path) as f:
        envelope = json.load(f)
    targets = json.loads(base64.b64decode(envelope["payload"]))
    delegations = targets.get("delegations", {})
    principals = {pid: pid for pid in delegations.get("principals", {})}
    rules = []
    for role in delegations.get("roles", []):
        if role["name"] == "gittuf-allow-rule":
            continue
        if role.get("terminating"):
            sys.exit(f"rule {role['name']}: terminating rules are not supported by the converter")
        members = sorted(role.get("principalIDs") or [])
        git_paths = [p for p in role["paths"] if p.startswith("git:")]
        for i, path in enumerate(git_paths):
            name = role["name"] if len(git_paths) == 1 else f"{role['name']}-{i}"
            rules.append({"name": name, "pattern": path[len("git:"):], "threshold": role["threshold"], "members": members})
    json.dump({"principals": principals, "rules": rules}, sys.stdout, indent=1, sort_keys=True)
    print()


if __name__ == "__main__":
    main(sys.argv[1])
