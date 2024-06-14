# SPDX-License-Identifier: Apache-2.0

GIT_VERSION ?= $(shell git describe --tags --always --dirty)

LDFLAGS=-buildid= -X github.com/gittuf/gittuf/internal/version.gitVersion=$(GIT_VERSION)

.PHONY : build test install fmt

default : install

build : test
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"  -o dist/gittuf .

install : test just-install

just-install :
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" github.com/gittuf/gittuf

test :
	go test -v ./...

fmt :
	go fmt ./...

generate :
	go generate ./...
