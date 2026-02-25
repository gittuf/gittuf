# gittuf-git

gittuf also comes with a drop-in replacement for the `git` binary that aims to
be command-compatible with Git, while also performing some gittuf operations for
you. We call this the **command compatibility layer**, named `gittuf-git`.

`gittuf-git` supports performing the following common operations for you:

- Creating RSL entries upon pushing your changes
- Fetching gittuf metadata when pulling changes

> [!NOTE] The command compatibility layer does not perform the steps needed to
> *initialize* a gittuf repository (i.e. setting up root of trust, policy,
> etc.). These steps must be done manually for new repositories (see the
> [getting started guide](/docs/get-started.md)).

## How to Install

At the moment, the command compatibility layer must be built from source, from
the `experimental` branch of the repository. Running `make` will compile the
command compatibility layer and place it in your `GOBIN`.

## How to Use

Once it's installed, using the command compatibility layer is easy. Simply
invoke `gittuf-git` instead of `git` for any git operation, e.g.

```bash
gittuf-git clone git@github.com:gittuf/gittuf
gittuf-git pull
gittuf-git push
```

## How it Works

The command compatibility layer has two modes of operation, depending on whether
the repository it is being used on is gittuf-enabled:

### A repository without gittuf

All operations will be passed through directly to the `git` binary on the
system. The behavior in this case is identical to running `git`.

### A repository with gittuf

For repositories with gittuf enabled, `gittuf-git` will check to see if the Git
operation being performed necessitates adding to or validating gittuf metadata.
The following is the list of Git operations that are intercepted and the changes
made:

- `clone`
    - In addition to cloning the repository as Git would do, gittuf-specific
      metadata, such as the RSL and policy are also fetched.

- `commit`
    - Before invoking `git commit`, gittuf verification is run. The result is
     printed out to `stdout`, but the process continues regardless of whether
     verification was successful or not.

- `push`
    - Before pushing, the changes are recorded to the RSL. After the
     user-specified changes are pushed to the remote, the RSL and policy are
     also pushed to the remote.

- `pull` and `fetch`
    - After the user-specified refs are pulled or fetched, the RSL and policy
     are also fetched from the remote.
