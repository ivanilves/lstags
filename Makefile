all: prepare dep test lint vet build

prepare:
	go get -u \
		github.com/golang/dep/cmd/dep \
		github.com/golang/lint/golint

dep:
	dep ensure

test:
	go test

lint:
	find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {}

vet:
	find . -name "*.go" ! -path "./vendor/*" | xargs -i go vet {}

build:
	go build
