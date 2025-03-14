# gittuf User Programmable Policy Extensions (GUPPIEs)

## Metadata

* **Number:**
* **Title:** gittuf User Programmable Policy Integration Extensions (GUPPIEs)
* **Implemented:** No
* **Withdrawn/Rejected:** No
* **Sponsors:** Aditya Sirish A Yelgundhalli (adityasaky), Patrick Zielinski (patzielinski)
* **Related GAPs:** [GAP-4](/docs/gaps/4/README.md)
* **Last Modified:** March 14, 2025

## Abstract

gittuf can currently be used to declare and verify compliance with write access
control policies. For cases where users want to define their own types of checks
to ensure compliance with, this GAP introduces gittuf User Programmable Policy
Integration Extensions (GUPPIEs) also known as gittuf hooks, declared in a
repository's policy metadata.

## Specification

This GAP introduces the notion of GUPPIEs, which enable users to extend gittuf
policy enforcement with custom programmable check programs that run at
predetermined times in the Git repository lifecycle.

## What is a GUPPIE?

GUPPIEs are custom checks that gittuf is able to run in addition to the existing
policy verification flow. GUPPIEs are inspired by the Git hooks that are used
today, but have several improvements compared to them. GUPPIEs
are an open-ended type of check, which means that a they may be used for a
variety of different check scenarios, within the capabilities of the environment
they are run in.

### Declaring GUPPIEs

GUPPIEs are declared by the repository's owners in the repository's policy
metadata, much like regular gittuf rules. A GUPPIE may be defined to apply for
any of the actors defined in a repository's gittuf policy, or any user who runs
gittuf verification on the repository. As with any changes to policy metadata,
adding, removing, or updating a GUPPIE requires approval from a threshold of
users authorized to set gittuf policy for the repository.

## Writing GUPPIEs

As of writing for this GAP, GUPPIEs can be written in the lightweight scripting
language Lua. While the implementation largely dictates the methods available to
GUPPIEs, a certain baseline is mandated by this GAP.

TODO: Extend

## Running GUPPIEs

Due to their customizable nature, GUPPIEs are run in an isolated environment. As
of writing, GUPPIEs may be run in an isolated Lua environment provided by the
implementation. GUPPIEs share the same invocation mechanism as Git hooks:
multiple "stages" are defined for various user interactions with the repository
(e.g. `pre-commit`, `pre-push`, `post-receive`, etc.). Whenever a stage is
invoked by the Git binary, the gittuf implementation is provided with the stage
name through an API call. The implementation identifies the appropriate GUPPIEs
for the user invoking the Git binary and executes each one of them. The result
of execution (e.g. return code, GUPPIE name, user running the GUPPIE, etc.) are
then recorded in an attestation to provide evidence of running the GUPPIE.

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
history to add the appropriate signoffs, etc.). GUPPIEs written in a lightweight
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

TODO...

## Backwards Compatibility

This GAP impacts backwards compatibility in certain cases. Should a repository's
metadata not declare any GUPPIEs, then any version of gittuf (irrespective of
whether it supports GUPPIEs) is able to properly interpret and verify compliance
with the policy for a repository. Should a repository declare GUPPIEs however,
versions of gittuf released prior to the addition of this feature will ignore
the declared GUPPIEs altogether. In such scenarios, the client must abort
verification with a message to the user to update their gittuf client.

## Security

The addition of GUPPIEs to gittuf raises two primary concerns. The first concern
is of the incompatibility of older clients with policy metadata that has GUPPIEs
declared. This is addressed in the backwards compatibility section.

The second is a concern about the execution of untrusted code on users'
machines. As with any system that allows running user-generated code inside it
as an extension, there is the possibility for unauthorized code execution.
GUPPIEs are however to be run in isolated environments, such as the Lua sandbox
for GUPPIEs written in Lua. It is up to the implementation of gittuf and the
appropriate isolation technology to ensure that GUPPIEs do not have unrestricted
access to users' computers.

## Prototype Implementation

There is a prototype implementation of this GAP in the `experimental` branch of
the gittuf repository.

A partial implementation is in the `main` branch of the gittuf repository, with
development underway.

## References

* [Git Hooks](https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks)
