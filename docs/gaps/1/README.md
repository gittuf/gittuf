# Providing SHA-256 Identifiers Alongside Existing SHA-1 Identifiers

## Metadata

* **Number:** 1
* **Title:** Providing SHA-256 Identifiers Alongside Existing SHA-1 Identifiers
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky)
* **Last Modified:** January 20, 2025

## Abstract

Git stores all its objects in a content addressed store located under
`.git/objects`. The content address for each object is calculated using SHA-1.
Due to known weaknesses with SHA-1, this GAP explores using gittuf to provide
cryptoagility for the hash algorithm used in Git, whether SHA-256 or other
algorithms that may be desirable to adopt in future.

## Specification

Git stores all its objects in a content addressed store located under
`.git/objects`. This directory contains subdirectories that act as an index to
the hashes themselves. For example, the Git object for commit
`4dcd174e182cedf597b8a84f24ea5a53dae7e1e7` is stored as
`.git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7`. The hash is calculated
across the corresponding object prior to compressing it, and it can be
recalculated as follows:

```
cat .git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7 | zlib-flate -uncompress | sha1sum
4dcd174e182cedf597b8a84f24ea5a53dae7e1e7  -
```

There are several types of Git objects: commits, blobs, trees, and tags. Commits
record changes made in the repository. Blobs are files in the repository while
trees map to the directory structure of the repository. Trees contain a record
of blobs and subtrees.

Git commits store a record of their one or more parent commits (creating a
Merkle DAG). Each commit also points to the specific tree object that
represents the root of the repository.

```
git cat-file -p db1c7b0210513a452b0b971e1912d5eb2e3ffcd0
tree 7b968da28453b323a0d3333e3be4030b870d26e4
parent 4dcd174e182cedf597b8a84f24ea5a53dae7e1e7
...
```

Finally, tag objects serve as static pointers to other Git objects (frequently
commits). As with Git commits and trees, the tag object also identifies the
target Git object using its identifier.

This GAP proposes recomputing SHA-256 identifiers for every object in the
repository. In this method, gittuf would maintain a SHA-1 to SHA-256 mapping for
every object in Git's content addressed store. This mapping can be a simple key
value dictionary.  When gittuf is invoked to calculate new identifiers, say when
creating a new commit, it must use Git's default semantics to create the object
with SHA-1 identifiers. For each new object created, it must replace SHA-1
identifiers with their SHA-256 equivalents, calculating them recursively if
necessary, and then finally calculate the SHA-256 hash. For every new object
encountered, a SHA-1 to SHA-256 entry must be added to the key value record.

Note that in this method, the new objects are not written to `.git/objects`.
Instead, the objects continue to be stored with their SHA-1 identifiers. The
only change is the addition of the file with the key value mapping.

TODO: Should a parallel set of objects be maintained with SHA-256 identifiers
that are symbolic links to their SHA-1 counterparts? This will probably not play
well with Git's packfiles while only maintaining a separate mapping will.

TODO: How much extra space does it take to store both versions of the objects?

TODO: How must this support arbitrary hash algorithms, beyond SHA-256?

TODO: How must the mapping of SHA-1 to SHA-256 (or other) hashes be stored and
synchronized in gittuf workflows?

### Impact on Commit / Tag signing

By default, Git signs commits using a SHA-256 representation of the commit
objects. However, these commit objects contain SHA-1 references. A collision of
the tree object referenced in the commit wouldn't be caught.

As such, the verification workflow for a commit must also validate that the
objects referenced by SHA-1 hashes also have the correct SHA-256 hashes. After
they are validated, the signature can be verified using the relevant public key
to check the identify of the committer.

TODO: Verification of SHA-256 hashes requires that the object be present as
well. How does this work when fetching new objects? Only a malicious object
that has a SHA-1 collision may be presented, meaning we don't have a reference
of the correct SHA-256 hash.

## Motivation

By default, Git uses the SHA-1 hash algorithm to calculate unique identifiers.
Due to known weaknesses with SHA-1, the Git community has proposed moving to
SHA-256. There is experimental support for SHA-256 identifiers, but:
1. repositories can't currently use both SHA-1 and SHA-256 identifiers, so
   converting existing repositories means the loss of development history.
1. most Git servers or forges don't support SHA-256 identifiers.

Since gittuf maintains a separate set of metadata about the Git objects in a
repository, it can also provide a mapping to SHA-256 identifiers. This requires
gittuf to maintain a SHA-256 reference to every SHA-1 identifier that exists in
a repository.

## Reasoning

### Forward Compatibility with Git's SHA-256 Support

One reason for recomputing SHA-256 hashes for all objects is forward
compatibility. As noted before, Git includes experimental support for SHA-256.
Here, a repository must be initialized with the object format set to SHA-256.
From then on, all object identifiers are calculated using SHA-256 and stored in
`.git/objects`. The same data structures are maintained, except all SHA-1
identifiers are replaced with SHA-256 identifiers. This is similar to the
technique described here, meaning that SHA-256 identifiers calculated by gittuf
are the same as Git's SHA-256 identifiers. This will play well with any
transition techniques provided by Git for SHA-1 repositories to SHA-256.

### Alternate Solution

A simpler solution is to calculate the SHA-256 hash of commit objects, rather
than recompute hashes for all objects. This would look similar to:

```
cat .git/objects/4d/cd174e182cedf597b8a84f24ea5a53dae7e1e7 | zlib-flate -uncompress | sha256sum
c9262d30f2dd6e50088acbfb67fa49bb3e80c30e57779551727bc89fcf02e21b  -
```

However, if a SHA-1 collision is successfully performed within the repository,
this technique has some blind spots. A collision with a commit object will be
detected as two distinct commit objects may collide in SHA-1 but overwhelmingly
won't in SHA-256. However, a collision in the tree object is more dangerous. In
this situation, the commit object can remain the same but point to a malicious
version of the tree. The SHA-256 identifier will not detect this change.

## Backwards Compatibility

This GAP does not impact the backwards compatibility of gittuf as it only
suggests recording additional information in gittuf metadata.

## Security

TODO: A detailed security analysis is necessary before this GAP can be implemented.

## Prototype Implementation

A prototype implementation was proposed in https://github.com/gittuf/gittuf/pull/105.

## Changelog

* January 20th, 2025: moved from `/docs/extensions` to `/docs/gaps` as GAP-1

## References

* [Git Hash Function Transition](https://git-scm.com/docs/hash-function-transition/2.48.0)
* [SHAttered](https://shattered.io/)
