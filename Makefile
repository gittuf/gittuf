# SPDX-License-Identifier: Apache-2.0

GIT_VERSION ?= $(shell git describe --tags --always --dirty)

LDFLAGS=-buildid= -X github.com/gittuf/gittuf/internal/version.gitVersion=$(GIT_VERSION)

.PHONY : build test install fmt

default : install

build : test
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0
	go build -trimpath -ldflags "$(LDFLAGS)" -o dist/gittuf .
	go build -trimpath -ldflags "$(LDFLAGS)" -o dist/git-remote-gittuf ./internal/git-remote-gittuf
	set CGO_ENABLED=
else
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/gittuf .
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/git-remote-gittuf ./internal/git-remote-gittuf
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/gittuf-git ./internal/gittuf-git
endif

install : test just-install

just-install :
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0
	go install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf
	go install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf/internal/git-remote-gittuf
	set CGO_ENABLED=
else
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf/internal/git-remote-gittuf
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf/internal/gittuf-git
endif

test :
	go test -timeout 20m -v ./...

fmt :
	go fmt ./...

generate :
	go generate ./...
