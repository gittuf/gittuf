# SPDX-License-Identifier: Apache-2.0

GIT_VERSION ?= $(shell git describe --tags --always --dirty)

LDFLAGS=-buildid= -X github.com/gittuf/gittuf/internal/version.gitVersion=$(GIT_VERSION)

.PHONY : build test install fmt

default : install

build : test
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"  -o dist/gittuf .

install : test
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf

test :
	go test -v ./...

fmt :
	@git diff --name-status $$(git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@') -- '*.go' | grep '.go$$' | grep -v '^D' | cut -f 2- | xargs -n 1 -P 4 goimports -l -e

generate :
	go generate ./...
