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

lint: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {})
lint: fail-on-errors

vet: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i go tool vet {})
vet: fail-on-errors

fail-on-errors:
	@echo "${ERRORS}" | grep . || echo "OK"
	@test `echo "${ERRORS}" | grep . | wc -l` -eq 0

build:
	go build
