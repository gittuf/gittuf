# Programmable Policy Extensions

## Metadata

* **Number:**
* **Title:** Programmable Policy Extensions
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski)
* **Related GAPs:** [GAP-4](/docs/gaps/4/README.md)
* **Last Modified:** October 16, 2025

## Abstract

gittuf can currently be used to declare and verify compliance with write access
control policies. For cases where users want to define their own types of checks
to ensure compliance with, this GAP introduces Programmable Policy Extensions
(PPEs), also known as gittuf hooks, declared in a repository's policy metadata.

## Specification

This GAP introduces the notion of Programmable Policy Extensions, which enable
users to extend gittuf policy enforcement with custom programmable checks
programs that run at predetermined times in the Git repository workflow.

## What is a PPE?

PPEs are custom checks that gittuf is able to run in addition to the existing
policy verification flow. PPEs are inspired by the Git hooks that are used
today, but have several improvements (discussed below) compared to them. PPEs
are an open-ended type of check, which means that they may be used for a variety
of different check scenarios, within the capabilities of the environment they
are run in.

### Declaring PPEs

PPEs are declared by the repository's owners in the repository's root metadata,
similar to global rules. PPEs are bound to a stage, which defines the time in a
Git developer workflow that the PPE will run. PPEs may use any stage defined for
[Git hooks](https://git-scm.com/docs/githooks#_hooks), such as `pre-commit,
pre-push, etc.`. In addition to the stages offered by Git, gittuf also adds a
stage, called `post-verify`, which runs after gittuf verification of write
access control rules has completed successfully.

A PPE has the following attributes:
* ID: A unique identifier for the PPE
* Principals: A set of principals which must run this PPE
* Hashes: A listing of hashes computed for the script file which the PPE 
  represents
* Environment: The environment which the PPE must run in
* Modules: The Lua modules which are to be exposed to the PPE in addition to 
  the standard set for an implementation, only applicable for the Lua 
  environment

A declaration of PPE stages and their corresponding PPEs has the following
schema:

```json
{
  "hooks": {
    "<stage>" : [
      {
        "name": "<name>",
        "principals": [
          "<principalID>"
        ],
        "hashes": {
          "gitBlob": "<SHA-1 hash>",
          "sha256": "<SHA-256 hash>"
        },
        "environment": "<environment>",
        "modules": [
          "<module>"
        ]
      }
    ]
  }
}
```

Here, `name` can be set to any unique (within the same stage) string.
`principals` defines the set of principals, identified by their IDs, which will
be required to run the PPE. A star `*` in this field indicates that this PPE
must be run for any user who invokes this stage, regardless of whether they are
listed as a principal in gittuf metadata. `hashes` lists the hashes of the 
file underlying the PPE (e.g. the script file that is run). This is able to 
accept hashes other than SHA-1 and SHA-256, for cryptographic agility 
purposes. `environment` defines the environment that the PPE will run in, 
such as `lua`. Finally, `modules` (only applicable if the PPE will run in 
the `lua` environment) defines the Lua modules which should be exposed to 
the PPE in addition to the standard set mandated by this GAP.

Here is an example PPE defined for the `pre-commit` stage using this schema:

```json
{
  "hooks": {
    "pre-commit" : [
      {
        "name": "sample-hook",
        "principals": [
          "aditya@example.com"
        ],
        "hashes": {
          "gitBlob": "c632704e1916ebbe1febaa0b1c0ba310db363d39",
          "sha256": "5ecf36d2beb4f3aaabd6a6323469bef1131309cfa4b566a09248ea4fb1791c11"
        },
        "environment": "lua",
        "modules": [
          ""
        ]
      }
    ]
  }
}
```

A PPE may be defined for any stage, as well as any principal in either a
repository's gittuf root or targets metadata, or any user who runs gittuf
verification on the repository. As with any changes to root metadata, adding,
removing, or updating a PPE requires approval from a threshold of repository 
owners.

## Writing PPEs

As of writing for this GAP, PPEs can be written in the lightweight scripting
language Lua. While the implementation largely dictates the methods available to
PPEs in any environment, a certain baseline of API properties is mandated by 
this GAP. Any API available to a PPE must not allow write access to any 
resource on the host machine, and must restrict any read access to inside 
the Git repository which declares the PPE.

### Lua

The following Lua modules are trusted by default:
```
basic, table, string, math, coroutine
```

In addition to these modules, an API which provides standard read-only
operations on a Git repository is required. The following properties are 
required of any API implementing this GAP:
* PPEs must only be able to read data from the Git repository they are declared
  in. Read access to all other system resources must be prohibited.
* PPEs must not be able to write data to disk.
* PPEs must not be able to communicate via the network.
* PPEs must not be able to execute arbitrary functions on the system outside
  the Lua sandbox.
* APIs must be well-documented and tested.

These operations as implemented in gittuf are as follows, followed by their
name, as well as reasoning for their inclusion:

* **Reading a Git blob (GitReadBlob)**
  * PPEs that operate on a blob's contents, e.g. secrets scanners must be
    able to read the contents of said blob.
* **Retrieving a Git object's size (GitGetObjectSize)**
  * PPEs may need to check object sizes, for example where large objects
    must not be accidentally checked into the repository to prevent the
    performance of the repository degrading.
* **Retrieving the target of a Git tag (GitGetTagTarget)**
  * PPEs may need to determine the commit a tag points to in order to
    perform operations on the commit's data.
* **Reading the tip of a Git reference (GitGetReference)**
  * PPEs may need to determine the latest commit on a certain branch to
    perform operations on it such comparing changes to the content to be
    committed.
* **Determining the fully qualified path of a Git reference (GitGetAbsoluteReference)**
  * PPEs may need to resolve the fully qualified path of a Git reference to
    determine mode of operation, e.g. behave differently based on the
    reference's path.
* **Determining the target of a specified Git symbolic reference (GitGetSymbolicReferenceTarget)**
  * PPEs may need to operate on the commit specified by a symbolic reference,
    e.g. `HEAD`, and may need to determine the reference the symbolic reference
    points to.
* **Retrieving the parents of a Git commit (TBD)**
  * PPEs may need to walk back on a branch's commits to perform operations
    such as determining whether a file was present in a previous commit.
* **Reading the message of the specified Git commit (GitGetCommitMessage)**
  * PPEs may need to check the contents of a commit's message, e.g. secrets
    scanners and DCO signoff checkers.
* **Retrieving the file paths changed by a Git commit (GitGetFilePathsChangedByCommit)**
  * PPEs may need to identify the file paths changed by a commit, e.g.
    secrets scanners only scanning files that have changed.
* **Retrieving the remote URL for the specified remote (GitGetRemoteURL)**
  * PPEs may operate differently based on the remote URL specified in a
    user's Git configuration.

In addition to the above API providing access to the Git repository, the
implementation may choose to expose additional operations intended to ease the
writing of PPEs. These operations SHOULD only be added as injected Lua methods.
That is, they should only be written as Lua functions that are executed inside
the sandbox by the PPE, unless prohibitive to do so. The gittuf implementation
provides the following additional operations:

* **Matching text against a given regular expression (MatchRegex)**
    * Lua does not provide a native regular expression library, which is needed
      for scripts that must pattern match, such as secrets scanners. This is
      implemented using the official regular expression library in Go.
* **Splitting a string using the provided separator (StrSplit)**
    * Provided as a convenience for users given Lua's lack of a native
      string splitter function.

To prevent unbounded usage of computer resources, Lua PPEs are to be 
restricted to no more than 100 seconds of computation time. Should a PPE 
exceed this duration, it is terminated and gittuf will return an error that 
the timeout was exceeded for the PPE.

The result of the PPE (e.g. error or successful completion) is indicated by the
value returned. A PPE must follow the same return code standard as defined 
by POSIX, e.g. a value of `0` signifies success, while any other value 
signifies an error.

## Running PPEs

PPEs share the same invocation mechanism as Git hooks: multiple "stages" are
defined for various user interactions with the repository (e.g. `pre-commit`,
`pre-push`, `post-receive`, etc.). Whenever a stage is invoked by the Git
binary, the gittuf implementation is provided with the stage name through an API
call. The implementation identifies the appropriate PPEs for the user invoking
the Git binary and executes each one of them in an isolated environment. The
result of execution (e.g. return code, PPE name, user running the PPE, etc.) are
then recorded in an attestation to provide evidence of running the PPE.

TODO: Extend

## Motivation

gittuf currently only supports validation of properties that have been
programmed into it, such as write access control, global constraints, etc. Many
scenarios may call for additional checks to be run as a part of a repository's
policy.

### Lightweight Developer Checks

Developer workflows often consist of running lightweight checks for mundane
processes, such as linting code, adding a Developer Certificate of Origin (DCO)
signoff to Git commits, etc. These checks, while simple, are important to run,
as correcting issues down the line may prove to be difficult (e.g. rewriting Git
history to add the appropriate signoffs, etc.). PPEs written in a lightweight
scripting language fit well here, as they allow developers to catch mistakes
early on in the process, before pushing code up to the forge.

### Continuous Integration

Many user-programmable checks are declared as workflows for CI solutions to run
in the cloud. This has the benefit of running all checks on a controlled and
consistent environment, at the expense of trusting the provider executing the
check. When these checks are more heavyweight (e.g. running build pipelines,
tests,), developers are less likely to run them often, given the whole process
of committing, pushing, and waiting for CI to finish. Enabling these checks to
run on developer machines allows for more runs of CI without the need to push
changes and allows for a wider possible testbed of devices.

TODO: Rewrite?

## Reasoning

This section describes how gittuf's PPE workflow compares to existing solutions.

### PPEs vs CI in Forges

Continuous integration workflows for use in forges are often defined by YAML
files in specific directories based on the forge in use. Should a repository useFirst
gittuf, these workflow files can be protected against unauthorized
modifications. However, [forges do not support definining the users that must
execute certain CI
workflows](https://github.com/orgs/community/discussions/27947), instead
defining them based on events (e.g. pushes or pull requests to certain Git
branches).

TODO: Expand?

## Backwards Compatibility

This GAP impacts backwards compatibility in certain cases. Should a repository's
metadata not declare any PPEs, then any version of gittuf (irrespective of
whether it supports PPEs) is able to properly interpret and verify compliance
with the policy for a repository. If a repository's gittuf metadata _does_
declare PPEs however, versions of gittuf released prior to the addition of this
feature will ignore the declared PPEs altogether. In such scenarios, the client
must abort verification with a message to the user to update their gittuf
client.

## Security

The addition of PPEs to gittuf raises two primary concerns. The first concern
is of the incompatibility of older clients with policy metadata that has PPEs
declared. This is addressed in the backwards compatibility section.

The second is a concern about the execution of untrusted code on users'
machines. As with any system that allows running user-generated code inside it
as an extension, there is the possibility for unauthorized code execution.
PPEs are however to be run in isolated environments, such as the Lua sandbox
for PPEs written in Lua. It is up to the implementation of gittuf and the
appropriate isolation technology to ensure that PPEs do not have unrestricted
access to users' computers.

## Prototype Implementation

There is a prototype implementation of this GAP in the `main` branch of the
gittuf repository, with development underway.

## References

* [Git Hooks](https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks)
