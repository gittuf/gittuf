# Providing SHA-256 identifiers alongside existing SHA-1 identifiers

Last Modified: January 21, 2023

By default, Git uses the SHA-1 hash algorithm to calculate unique identifiers.
There is experimental support for SHA-256 identifiers, but:
1. repositories can't currently use both SHA-1 and SHA-256 identifiers, so
   converting existing repositories means the loss of development history.
1. most Git servers or forges don't support SHA-256 identifiers.

Since gittuf maintains a separate set of metadata about the Git objects in a
repository, it can also provide a mapping to SHA-256 identifiers. This requires
gittuf to maintain a SHA-256 reference to every SHA-1 identifier that exists in
a repository.

## Background: SHA-1

Git stores all its objects in a content addressed store located under
`.git/objects`. This directory contains subdirectories that act as an index to
the hashes themselves. For example, the Git object for commit
`4dcd174e182cedf597b8a84f24ea5a53dae7e1e7` is stored as
`.git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7`. The hash is
calculated across the corresponding object prior to compressing it, and it can
be recalculated as follows:

```
$ cat .git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7 | zlib-flate -uncompress | sha1sum
4dcd174e182cedf597b8a84f24ea5a53dae7e1e7  -
```

## Supporting SHA-256

There are several types of Git objects: commits, blobs, and trees. Commits
record changes made in the repository. Blobs are files in the repository while
trees map to the directory structure of the repository. Trees contain a record
of blobs and subtrees.

Git commits store a record of their one or more parent commits (creating a
Merkle DAG). Each commit also points to the specific tree object that
represents the root of the repository.

```
$ git cat-file -p db1c7b0210513a452b0b971e1912d5eb2e3ffcd0
tree 7b968da28453b323a0d3333e3be4030b870d26e4
parent 4dcd174e182cedf597b8a84f24ea5a53dae7e1e7
...
```

### Approach 1

Now, there are several ways to calculate SHA-256 identifiers. The simplest way
is to calculate the SHA-256 hash of the commit object itself.

```
$ cat .git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7 | zlib-flate -uncompress | sha256sum
c9262d30f2dd6e50088acbfb67fa49bb3e80c30e57779551727bc89fcf02e21b  -
```

However, if a SHA-1 collision is successfully performed within the repo, this
technique has some blind spots. A collision with a commit object will be
detected as two distinct commit objects may collide in SHA-1 but almost
overwhelmingly won't in SHA-256. However, a collision in the tree object is
more dangerous. In this situation, the commit object can remain the same but
point to a malicious version of the tree. The SHA-256 identifier will not
detect this change.

### Approach 2

A more involved way of calculating SHA-256 identifiers requires every object in
the repository with a SHA-1 object to have a SHA-256 identifier. In this
method, gittuf maintains a SHA-1 to SHA-256 mapping for every object in Git's
content addressed store. This mapping can be a simple key value dictionary.
When gittuf is invoked to calculate new identifiers, say when creating a new
commit, it must use Git's default semantics to create the object with SHA-1
identifiers. For each new object created, it must replace SHA-1 identifiers with
their SHA-256 equivalents, calculating them recursively if necessary, and then
finally calculate the SHA-256 hash. For every new object encountered, a SHA-1 to
SHA-256 entry must be added to the key value record.

Note that in this method, the new objects are not written to `.git`. Instead,
the objects continue to be stored with their SHA-1 identifiers. The only change
is the addition of the file with the key value mapping.

However, a parallel set of objects could be maintained with SHA-256 identifiers
that are symbolic links to their SHA-1 counterparts. Note that this will
probably not play well with Git's packfiles while maintaining a separate mapping
will.

**Q:** How much extra space does it take to store both versions of the objects?

An extra reason to use this technique is forward compatibility. As noted
before, Git includes experimental support for SHA-256. Here, a repository must
be initialized with the object format set to SHA-256. From then on, all object
identifiers are calculated using SHA-256 and stored in `.git/objects`. The same
data structures are maintained, except all SHA-1 identifiers are replaced with
SHA-256 identifiers. This is similar to the technique described here, meaning
that SHA-256 identifiers calculated by gittuf are the same as Git's SHA-256
identifiers. This will play well with any transition techniques provided by Git
for SHA-1 repositories to SHA-256 in future.

## Commit / Tag signing

By default, Git signs commits using a SHA-256 representation of the commit
objects. However, these commit objects contain SHA-1 references. A collision of
the tree object referenced in the commit wouldn't be caught.

As such, the verification workflow for a commit must also validate that the
objects referenced by SHA-1 hashes also have the correct SHA-256 hashes. After
they are validated, the signature can be verified using the relevant public key
to check the identify of the committer.

**Q:** Verification of SHA-256 hashes requires that the object be present as
well. How does this work when fetching new objects? Only a malicious object
that has a SHA-1 collision may be presented, meaning we don't have a reference
of the correct SHA-256 hash.

**T:** We'd have to pass around a prior calculated SHA-256 hash via the
translation mapping. However, if that must be trusted, we'd also have to ensure
it wasn't tampered with. TUF semantics can help here.