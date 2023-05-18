.PHONY : install

default : install

install : test
	go install

test :
	go test ./...
