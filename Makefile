all: prepare dep test lint vet build

prepare:
	go get -u -v \
		github.com/golang/dep/cmd/dep \
		github.com/golang/lint/golint

dep:
	dep ensure -v

test: unit-test package-test

unit-test:
	go test -v

package-test:
	@find \
		-mindepth 2 -type f ! -path "./vendor/*" -name "*_test.go" \
		| xargs dirname \
		| xargs -i sh -c "pushd {}; go test -v || exit 1; popd"

integration-test:
	go test -integration -v

shell-test: build
	./lstags alpine | egrep "\salpine:latest"
	./lstags nobody/nothing &>/dev/null && exit 1 || true

lint: ERRORS:=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {})
lint: fail-on-errors

vet: ERRORS:=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i go tool vet {})
vet: fail-on-errors

fail-on-errors:
	@echo "${ERRORS}" | grep . || echo "OK"
	@test `echo "${ERRORS}" | grep . | wc -l` -eq 0

build:
	go build

build-linux: GOOS:=linux
build-linux:
	GOOS=${GOOS} go build -o lstags.${GOOS}

build-darwin: GOOS:=darwin
build-darwin:
	GOOS=${GOOS} go build -o lstags.${GOOS}

xbuild: build-linux build-darwin
