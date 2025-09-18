# Lua Sandbox APIs

## gitGetAbsoluteReference

**Signature:** `gitGetAbsoluteReference(ref) -> absoluteRef`

Retried the fully qualified reference path for the specified Git reference.

### Example 1

```
gitGetAbsoluteReference("main") -> "refs/heads/main"
```

## gitGetBlobID

**Signature:** `gitGetBlobID(ref, path) -> blobID`

Retrieve the blob ID of the file at the given path from a specific reference (commit or staged index).

### Example 1

```
gitGetBlobID(":", "s.txt") -> "abc123..." (staged)
```

### Example 2

```
gitGetBlobID("HEAD", "s.txt") -> "def456..." (current commit)
```

### Example 3

```
gitGetBlobID("HEAD~1", "s.txt") -> "ghi789..." (previous commit)
```

## gitGetCommitMessage

**Signature:** `gitGetCommitMessage(commitID) -> message`

Retrieve the message for the specified Git commit.

### Example 1

```
gitGetCommitMessage("e7fca95377c9bad2418c5df7ab3bab5d652a5309") -> "Commit message."
```

## gitGetFilePathsChangedByCommit

**Signature:** `gitGetFilePathsChangedByCommit(commitID) -> paths`

Retrieve a Lua table of file paths changed by the specified Git commit.

### Example 1

```
gitGetFilePathsChangedByCommit("e7fca95377c9bad2418c5df7ab3bab5d652a5309") -> 2, "foo/bar", "foo/baz"
```

## gitGetObjectSize

**Signature:** `gitGetObjectSize(objectID) -> size`

Retrieve the size of the Git object specified using its ID from the repository.

### Example 1

```
gitGetObjectSize("e7fca95377c9bad2418c5df7ab3bab5d652a5309") -> 13
```

## gitGetReference

**Signature:** `gitGetReference(ref) -> hash`

Retrieve the tip of the specified Git reference.

### Example 1

```
gitGetReference("main") -> "e7fca95377c9bad2418c5df7ab3bab5d652a5309"
```

### Example 2

```
gitGetReference("refs/heads/main") -> "e7fca95377c9bad2418c5df7ab3bab5d652a5309"
```

### Example 3

```
gitGetReference("refs/gittuf/reference-state-log") -> "c70885ffc33866dbdfe95d0e10efa6d77c77a43b"
```

## gitGetRemoteURL

**Signature:** `gitGetRemoteURL(remote) -> remoteURL`

Retrieve the remote URL for the specified Git remote.

### Example 1

```
gitGetRemoteURL("origin") -> "example.com/example/example"
```

## gitGetStagedFilePaths

**Signature:** `gitGetStagedFilePaths() -> paths`

Retrieve a Lua table of file paths that have staged changes (changes in the index).

### Example 1

```
gitGetStagedFilePaths() -> ["foo/bar.txt", "baz/qux.py"]
```

## gitGetSymbolicReferenceTarget

**Signature:** `gitGetSymbolicReferenceTarget(ref) -> ref`

Retrieve the name of the Git reference the specified symbolic Git reference is pointing to.

### Example 1

```
gitGetSymbolicReferenceTarget("HEAD") -> "refs/heads/main"
```

## gitGetTagTarget

**Signature:** `gitGetTagTarget(tagID) -> targetID`

Retrieve the ID of the Git object that the tag with the specified ID points to.

### Example 1

```
gitGetTagTarget("f38f261f5df1d393a97aec3a5463017da6c22934") ->  "e7fca95377c9bad2418c5df7ab3bab5d652a5309"
```

## gitReadBlob

**Signature:** `gitReadBlob(blobID) -> blob`

Retrieve the bytes of the Git blob specified using its ID from the repository.

### Example 1

```
gitReadBlob("e7fca95377c9bad2418c5df7ab3bab5d652a5309") -> "Hello, world!"
```

## matchRegex

**Signature:** `matchRegex(pattern, text) -> matched`

Check if the regular expression pattern matches the provided text.

## strSplit

**Signature:** `strSplit(str, sep) -> components`

Split string using provided separator. If a separator is not provided, then "\n" is used by default.

### Example 1

```
strSplit("hello\nworld") -> ["hello", "world"]
```

### Example 2

```
strSplit("hello\nworld", "\n") -> ["hello", "world"]
```
