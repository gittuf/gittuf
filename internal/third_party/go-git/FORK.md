# Fork of github.com/go-git/go-git/v5

This package is a fork of go-git v5.9.0 with a patch submitted to change
behavior in `fetch`. Specifically, one of the checks performed to determine if
the fetch needs a `force` directive is if the destination Git ref is a branch.
go-git v5.9.0 assumes the destination is a branch only if it's in
`refs/heads`. This isn't true for custom refs such as the ones we use for gittuf
metadata. A patch has been submitted to go-git to handle custom refs, and when
it's merged and released, this fork will be removed.

Upstream patch: https://github.com/go-git/go-git/pull/875

Issue tracking future removal: https://github.com/gittuf/gittuf/issues/145

## Forking Steps

1. go mod vendor -> vendors all dependencies
2. Copy go-git to the internal namespace
3. Run go mod init with go-git's module name and go mod tidy in the
   internal copy of go-git
4. Add FORK.md file
5. Add replace directive to gittuf's go.mod followed by go mod tidy
6. Apply patch to internal go-git
7. Remove vendor directory
