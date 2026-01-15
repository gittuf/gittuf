# gittuf's `gitinterface` Package

gittuf's `gitinterface` package is a lightweight Go API for interacting with Git
repositories. It is similar to [`go-git`](https://github.com/go-git/go-git) in
its goal, but differs as, unlike `go-git`, `gitinterface` uses the Git binary
for its operations.

To operate correctly, `gitinterface` requires a Git binary version of 2.34 or
higher. 
