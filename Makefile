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

env:
	env

shell-test: build shell-test-alpine shell-test-wrong-image shell-test-pull-public shell-test-pull-private

shell-test-alpine:
	./lstags alpine | egrep "\salpine:latest"

shell-test-wrong-image:
	./lstags nobody/nothing &>/dev/null && exit 1 || true

shell-test-pull-public: DOCKERHUB_PUBLIC_REPO?=ivanilves/dummy
shell-test-pull-public:
	./lstags --pull ${DOCKERHUB_PUBLIC_REPO}~/latest/

shell-test-pull-private: DOCKER_JSON:=tmp/docker.json.private-repo
shell-test-pull-private:
	mkdir -p tmp
	if [[ -n "${DOCKERHUB_PRIVATE_REPO}" && -n "${DOCKERHUB_AUTH}" ]]; then\
		touch "${DOCKER_JSON}" && chmod 0600 "${DOCKER_JSON}" \
		&& echo "{ \"auths\": { \"registry.hub.docker.com\": { \"auth\": \"${DOCKERHUB_AUTH}\" } } }" >"${DOCKER_JSON}"\
		&& ./lstags -j "${DOCKER_JSON}" --pull ${DOCKERHUB_PRIVATE_REPO}~/latest/; else echo "DOCKERHUB_PRIVATE_REPO or DOCKERHUB_AUTH not set!";\
	fi

lint: ERRORS:=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {})
lint: fail-on-errors

vet: ERRORS:=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i go tool vet {})
vet: fail-on-errors

fail-on-errors:
	@echo "${ERRORS}" | grep . || echo "OK"
	@test `echo "${ERRORS}" | grep . | wc -l` -eq 0

build:
	@if [[ -z "${GOOS}" ]]; then go build; fi
	@if [[ -n "${GOOS}" ]]; then mkdir -p dist/assets/lstags-${GOOS}; GOOS=${GOOS} go build -o dist/assets/lstags-${GOOS}/lstags; fi

xbuild:
	${MAKE} --no-print-directory build GOOS=linux
	${MAKE} --no-print-directory build GOOS=darwin

clean:
	rm -rf ./lstags ./dist/

changelog: LAST_RELEASE?=$(shell git tag | sed 's/^v//' | sort -n | tail -n1)
changelog: GITHUB_COMMIT_URL:=https://github.com/ivanilves/lstags/commit
changelog:
	@echo "## Changelog"
	@git log --oneline v${LAST_RELEASE}..HEAD | egrep -iv "^[0-9a-f]{7,} (Merge pull request |Merge branch |Ignore:)" | \
		sed -r "s|^([0-9a-f]{7,}) (.*)|* [\`\1\`](${GITHUB_COMMIT_URL}/\1) \2|"

release: clean
release: LAST_RELEASE:=$(shell git tag | sed 's/^v//' | sort -n | tail -n1)
release: THIS_RELEASE:=$(shell expr ${LAST_RELEASE} + 1)
release: RELEASE_CSUM:=$(shell git rev-parse --short HEAD)
release: RELEASE_NAME:=v${THIS_RELEASE}-${RELEASE_CSUM}
release:
	mkdir -p ./dist/release ./dist/assets
	sed -i "s/CURRENT/${RELEASE_NAME}/" ./version.go && ${MAKE} xbuild && git checkout ./version.go
	echo ${RELEASE_NAME} > ./dist/release/NAME && echo v${THIS_RELEASE} > ./dist/release/TAG
	${MAKE} --no-print-directory changelog > ./dist/release/CHANGELOG.md
	cp README.md ./dist/assets/

validate-release:
	test -s ./dist/release/TAG && test -s ./dist/release/NAME
	egrep "^\* " ./dist/release/CHANGELOG.md
	[[ `find dist/assets -mindepth 2 -type f | wc -l` -ge 2 ]]

deploy: validate-release
deploy: TAG=$(shell cat ./dist/release/TAG)
deploy:
	test -n "${GITHUB_TOKEN}" && git tag ${TAG} && git push --tags
	GITHUB_TOKEN=${GITHUB_TOKEN} ./scripts/github-create-release.sh ./dist/release
	GITHUB_TOKEN=${GITHUB_TOKEN} ./scripts/github-upload-assets.sh ${TAG} ./dist/assets
