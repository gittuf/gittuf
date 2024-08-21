# git-remote-gittuf

Alongside the `gittuf` binary, gittuf ships with a custom remote transfer
protocol binary, implementing Git's [remote-helper
interface](https://git-scm.com/docs/gitremote-helpers). We call this the
**transport** binary, named `git-remote-gittuf`.

It's an easy way to get started with using gittuf on your repository, as it
takes care of the following common operations for you:

- Creating RSL entries upon pushing your changes
- Fetching gittuf metadata when pulling changes

> [!NOTE] The transport does not perform the steps needed to *initialize* a
> gittuf repository (i.e. setting up root of trust, policy, etc.). These steps
> must be done manually for new repositories (see the [getting started
> guide](/docs/get-started.md)).

The gittuf transport supports both HTTPS and SSH remotes.

## How to Obtain

Currently, the transport must be built from source. Running `go install` will
compile the transport and place it in your `GOBIN`, which is the same location
that the `gittuf` binary is installed into. As it matures, we will update our
release configuration and the packaging of gittuf (e.g., on Homebrew) to include
this binary.

## How to Use

Once it's installed, using the custom transport is simple; you'll need to add
the `gittuf::` prefix to the repository URL. How to do this depends on the
repository you'd like to use it for.

### Using with a fresh `git clone`

When running `git clone`, add `gittuf::` to the beginning of the URL of the
repository. For example,

- `gittuf::git@github.com:gittuf/gittuf`, if you're using SSH
- `gittuf::https://github.com/gittuf/gittuf`, if you're using HTTPS

### Using with an existing repository

In this case you'll need to set the remote for your repository (most likely
`origin`):

```bash
# For SSH
git remote set-url origin gittuf::git@github.com:gittuf/gittuf

# For HTTPS
git remote set-url origin gittuf::https://github.com/gittuf/gittuf
```
