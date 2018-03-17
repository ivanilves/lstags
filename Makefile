API_VERSION:=$(shell cat API_VERSION)

.PHONY: default clean offline prepare dep test unit-test whitebox-integration-test coverage \
	blackbox-integration-test shell-test-alpine shell-test-wrong-image shell-test-docker-socket shell-test-docker-tcp \
	lint vet fail-on-errors docker-image build xbuild changelog release validate-release deploy deploy-github deploy-docker \
	wrapper install

default: prepare dep test lint vet build

clean:
	rm -rf ./lstags ./dist/ *.log *.pid

offline: unit-test lint vet build

prepare:
	go get -u -v \
		github.com/golang/dep/cmd/dep \
		github.com/golang/lint/golint \
		github.com/go-playground/overalls \
		github.com/mattn/goveralls

dep:
	dep ensure -v

test: unit-test whitebox-integration-test

unit-test:
	@find \
		-mindepth 2 -type f ! -path "./vendor/*" ! -path "./api/*" -name "*_test.go" \
		| xargs dirname \
		| xargs -i sh -c "pushd {}; go test -v -cover || exit 1; popd"

whitebox-integration-test:
	@find \
		-mindepth 2 -type f -path "./api/*" -name "*_test.go" \
		| xargs dirname \
		| xargs -i sh -c "pushd {}; go test -v -cover || exit 1; popd"

coverage: PROJECT:=github.com/ivanilves/lstags
coverage: SERVICE:=travis-ci
coverage:
	overalls -project=${PROJECT} -covermode=count \
		&& if [[ -n "${COVERALLS_TOKEN}" ]]; then goveralls -coverprofile=overalls.coverprofile -service ${SERVICE}; fi

blackbox-integration-test: build \
	shell-test-alpine \
	shell-test-wrong-image \
	shell-test-docker-socket \
	shell-test-docker-tcp

shell-test-alpine:
	./lstags alpine | egrep "\salpine:latest"

shell-test-wrong-image:
	./lstags nobody/nothing &>/dev/null && exit 1 || true

shell-test-docker-socket:
	unset DOCKER_HOST && ./lstags alpine~/latest/

shell-test-docker-tcp: DOCKER_HOST:=tcp://127.0.0.1:2375
shell-test-docker-tcp:
	./lstags nginx~/stable/

lint: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {} | tr '`' '|')
lint: fail-on-errors

vet: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i go tool vet {} | tr '`' '|')
vet: fail-on-errors

fail-on-errors:
	@echo "${ERRORS}" | grep . || echo "OK"
	@test `echo "${ERRORS}" | grep . | wc -l` -eq 0

docker-image: DOCKER_REPO:=ivanilves/lstags
docker-image: DOCKER_TAG:=latest
docker-image: GOOS=linux
docker-image: build
docker-image:
	@docker image build -t ${DOCKER_REPO}:${DOCKER_TAG} .

build: NAME=$(shell test "${GOOS}" = "windows" && echo 'lstags.exe' || echo 'lstags')
build:
	@if [[ -z "${GOOS}" ]]; then go build -ldflags '-d -s -w' -a -tags netgo -installsuffix netgo; fi
	@if [[ -n "${GOOS}" ]]; then mkdir -p dist/assets/lstags-${GOOS}; GOOS=${GOOS} go build -ldflags '-s -w' -a -tags netgo -installsuffix netgo -o dist/assets/lstags-${GOOS}/${NAME}; fi

xbuild:
	${MAKE} --no-print-directory build GOOS=linux
	${MAKE} --no-print-directory build GOOS=darwin
	${MAKE} --no-print-directory build GOOS=windows

changelog: LAST_RELEASED_TAG:=$(shell git tag --sort=creatordate | tail -n1)
changelog: GITHUB_COMMIT_URL:=https://github.com/ivanilves/lstags/commit
changelog:
	@echo "## Changelog" \
	&& git log --oneline --reverse ${LAST_RELEASED_TAG}..HEAD | egrep -iv "^[0-9a-f]{7,} (Merge pull request |Merge branch |NORELEASE)" | \
		sed -r "s|^([0-9a-f]{7,}) (.*)|* [\`\1\`](${GITHUB_COMMIT_URL}/\1) \2|"

release: clean
release: LAST_BUILD_NUMBER:=$(shell git tag --sort=creatordate | tail -n1 | sed 's/^v.*\.//')
release: THIS_BUILD_NUMBER:=$(shell expr ${LAST_BUILD_NUMBER} + 1)
release: THIS_RELEASE_NAME:=v${API_VERSION}.${THIS_BUILD_NUMBER}
release:
	mkdir -p ./dist/release ./dist/assets \
	&& sed -i "s/CURRENT/${THIS_RELEASE_NAME}/" ./version.go && ${MAKE} xbuild && git checkout ./version.go \
	&& echo ${THIS_RELEASE_NAME} > ./dist/release/NAME && echo ${THIS_RELEASE_NAME} > ./dist/release/TAG \
	&& ${MAKE} --no-print-directory changelog > ./dist/release/CHANGELOG.md \
	&& cp README.md ./dist/assets/

validate-release:
	test -s ./dist/release/TAG && test -s ./dist/release/NAME
	test -f ./dist/release/CHANGELOG.md && grep '^\* ' ./dist/release/CHANGELOG.md >/dev/null
	[[ `find dist/assets -mindepth 2 -type f | wc -l` -ge 3 ]]

deploy: DO_RELEASE:=$(shell git show | grep -i "Merge.*NORELEASE" >/dev/null && echo "false" || echo "true")
deploy: deploy-github deploy-docker

deploy-github: TAG=$(shell cat ./dist/release/TAG)
deploy-github:
	@if [[ "${DO_RELEASE}" == "true" ]]; then \
		${MAKE} --no-print-directory validate-release \
		&& test -n "${GITHUB_TOKEN}" && git tag ${TAG} && git push --tags \
		&& GITHUB_TOKEN=${GITHUB_TOKEN} ./scripts/github-create-release.sh ./dist/release \
		&& GITHUB_TOKEN=${GITHUB_TOKEN} ./scripts/github-upload-assets.sh ${TAG} ./dist/assets; \
	else \
		echo "NB! GitHub release skipped! (DO_RELEASE != true)"; \
	fi

deploy-docker: DOCKER_REPO:=ivanilves/lstags
deploy-docker: DOCKER_TAG=$(shell cat ./dist/release/TAG)
deploy-docker:
	@if [[ "${DO_RELEASE}" == "true" ]]; then \
		docker tag ${DOCKER_REPO}:release ${DOCKER_REPO}:${DOCKER_TAG} && docker tag ${DOCKER_REPO}:release ${DOCKER_REPO}:latest \
		&& docker push ${DOCKER_REPO}:${DOCKER_TAG} && docker push ${DOCKER_REPO}:latest; \
	else \
		echo "NB! Docker release skipped! (DO_RELEASE != true)"; \
	fi

wrapper: PREFIX:=/usr/local
wrapper:
	install -o root -g root -m755 scripts/wrapper.sh ${PREFIX}/bin/lstags

install: wrapper
