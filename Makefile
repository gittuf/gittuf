# SPDX-License-Identifier: Apache-2.0

GO ?= go

GIT_VERSION ?= $(shell git describe --tags --always --dirty)

LDFLAGS=-buildid= -X github.com/gittuf/gittuf/internal/version.gitVersion=$(GIT_VERSION)

.PHONY : build test test-compat install fmt

default : install

build : test
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/gittuf .
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/git-remote-gittuf ./internal/git-remote-gittuf
	set CGO_ENABLED=
else
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/gittuf .
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/git-remote-gittuf ./internal/git-remote-gittuf
endif

install : test just-install

just-install :
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf/internal/git-remote-gittuf
	set CGO_ENABLED=
else
	CGO_ENABLED=0 $(GO) install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf
	CGO_ENABLED=0 $(GO) install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf/internal/git-remote-gittuf
endif

test :
	$(GO) test -race -timeout 20m -v ./...

test-compat :
	GO=$(GO) ./scripts/test-compat.sh

fmt :
	$(GO) fmt ./...

generate :
	$(GO) generate ./...
