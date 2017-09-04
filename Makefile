all: prepare dep unit-test lint vet build

prepare:
	go get -u -v \
		github.com/golang/dep/cmd/dep \
		github.com/golang/lint/golint

dep:
	dep ensure -v

test: unit-test

unit-test:
	go test -v

integration-test:
	go test -integration -v

lint:
	@find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {}

vet:
	@find . -name "*.go" ! -path "./vendor/*" | xargs -i go tool vet {}

build:
	go build
