# Debug Helpers

This directory contains tools or artifacts used by the gittuf developers during
debugging.

## Debugging gittuf with Git 2.34.1

Build and run the container from the root of the gittuf repository.

```bash
docker build -t debug-gittuf-2-34-1 -f debug/Dockerfile.Git_2_34_1 .
docker run -it --rm -v $PWD:/gittuf -w /gittuf debug-gittuf-2-34-1
```

As the gittuf repository is mounted as a volume, you can make changes on the
host and iteratively debug within the container.
