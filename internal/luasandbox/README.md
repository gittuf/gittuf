# luasandbox

## Overview
The `luasandbox` package provides a Lua sandbox environment for executing Lua
scripts with controlled access to the file system and Git repository. 

## Structure
- **main.go**: Entry point for the Lua sandbox, initializes the environment and
  configures security settings.
- **api.go**: Defines and registers all API functions exposed to Lua, including
  predefined Lua functions and those mapped to Go functions.
- **util.go**: Utility functions and helpers for api functions and sandbox
  settings.
  
## API Functions
### 1. `readFile`

#### Signature
    readFile(filePath: string) -> string

#### Description
Reads the content of a file and returns it as a string. If the file path is not
permitted or there is an error reading the file, returns an error message.


### 2. `getDiff`

#### Signature

    getDiff() -> string


#### Description
Retrieves the raw Git diff output of current changes in the repository.

---

### 3. `getWorkTree`

#### Signature
    getWorkTree() -> string

#### Description
Lists the files in the Git working tree (one file per line).

---

### 4. `checkAddedLargeFile`

#### Signature

    checkAddedLargeFile(maxSizeKB: number, enforceAll: 
    ean) -> table[string]

#### Description
Checks added or modified files (or all files in the work tree if `enforceAll` is
`true`) to see if any exceed a specified size in kilobytes. Returns a Lua table
of file paths.


---

### 5. `checkMergeConflict`

#### Signature

    checkMergeConflict() -> table[string]

#### Description
Scans files listed by `git diff` for merge conflict markers (`<<<<<<<`,
`=======`, `>>>>>>>`). Returns a table of file names where conflicts are found.


---

### 6. `checkJSON`

#### Signature

    checkJSON() -> table[string]

#### Description
Inspects all files in the work tree to see if they contain valid JSON. Returns a
table of files that successfully parse as JSON.

---

### 7. `checkNoCommitOnBranch`

#### Signature

    checkNoCommitOnBranch(protectedBranches: table[string], patterns: table[string]) -> boolean

#### Description
Checks the current Git branch name against a list of protected branch names and
regex patterns. Returns `true` if the branch is protected, `false` otherwise.

---

### 8. `getGitObject`

#### Signature

    getGitObject(object: string) -> string


#### Description
Retrieves and returns the raw content of a Git object (commit, tree, blob, etc.)
via `git cat-file -p`.

---

### 9. `getGitObjectSize`

#### Signature

    getGitObjectSize(object: string) -> string


#### Description
Gets the size (in bytes) of the specified Git object by running `git cat-file
-s`.


---

### 10. `getGitObjectHash`

#### Signature

    getGitObjectHash(object: string) -> string


#### Description
Computes the Git hash of the given file or object using `git hash-object -w`.

---

### 11. `getGitObjectPath`

#### Signature

    getGitObjectPath(object: string) -> string


#### Description
Resolves the path or reference of a Git object via `git rev-parse`. This returns
a string describing the resolved commit or reference.

---
### 12. `regexMatch`
#### Signature

    regexMatch(pattern:string, text:string) -> boolean

#### Description
Searches for a regex pattern in a given text. Returns `true` if the pattern is
found, `false` otherwise.

---
### 13. `regexMatchGitDiff`

#### Signature

    regexMatchGitDiff(patternTable: table[{ key: string, regex: string }]) -> table[ { type: string, line_num: string, content: string } ]

#### Description
Processes the Git diff output line by line and searches for user-defined regex
patterns in added lines (lines prefixed with `+`). Returns a table keyed by
filename, each containing a list of matches (with type, line number, and
content).
