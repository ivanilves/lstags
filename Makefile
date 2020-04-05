export SHELL := /bin/bash

API_VERSION:=$(shell cat API_VERSION)

.PHONY: default PHONY clean offline prepare dep test unit-test whitebox-integration-test coverage blackbox-integration-test shell-test-alpine shell-test-wrong-image shell-test-docker-socket shell-test-docker-tcp shell-test-pullpush start-local-registry stop-local-registry push-to-local-registry stress-test stress-test-async stress-test-wait lint vet fail-on-errors docker-image build xbuild changelog release validate-release deploy deploy-github deploy-docker poc-app wrapper install

default: prepare dep test lint vet build

PHONY:
	@egrep "^[0-9a-zA-Z_\-]+:( |$$)" Makefile \
		| cut -d":" -f1 | uniq | tr '\n' ' ' | sed 's/^/.PHONY: /;s/ $$//' \
		| xargs -I {} sed -i "s/^\.PHONY:.*$$/{}/" Makefile

clean:
	git clean -fdx

offline: unit-test lint vet build

prepare:
	go get -u \
		golang.org/x/lint/golint \
		github.com/go-playground/overalls \
		github.com/mattn/goveralls

dep:
	go mod verify
	go mod vendor

test: unit-test whitebox-integration-test

unit-test:
	@find . \
		-mindepth 2 -type f ! -path "./vendor/*" ! -path "./api/*" -name "*_test.go" \
		| xargs -I {} dirname {} \
		| xargs -I {} sh -c "pushd {}; go test -v -cover || exit 1; popd"

whitebox-integration-test:
	@find . \
		-mindepth 2 -type f -path "./api/*" -name "*_test.go" \
		| xargs -I {} dirname {} \
		| xargs -I {} sh -c "pushd {}; go test -v -cover || exit 1; popd"

coverage: PROJECT:=github.com/ivanilves/lstags
coverage: SERVICE:=ci
coverage:
	@overalls -project=${PROJECT} -covermode=count \
		&& if [[ -n "${COVERALLS_REPO_TOKEN}" ]]; then goveralls -coverprofile=overalls.coverprofile -repotoken ${COVERALLS_REPO_TOKEN} -service=${SERVICE}; fi

blackbox-integration-test: shell-test-alpine shell-test-wrong-image \
	shell-test-docker-socket shell-test-docker-tcp shell-test-pullpush

shell-test-alpine:
	./lstags alpine | egrep "\salpine:latest"

shell-test-wrong-image:
	./lstags nobody/nothing &>/dev/null && exit 1 || true

shell-test-docker-socket:
	unset DOCKER_HOST && ./lstags alpine~/latest/

shell-test-docker-tcp: DOCKER_HOST:=tcp://127.0.0.1:2375
shell-test-docker-tcp:
	./lstags nginx~/stable/

shell-test-pullpush: start-local-registry push-to-local-registry stop-local-registry

start-local-registry: REGISTRY_PORT:=5757
start-local-registry:
	docker run -d -p ${REGISTRY_PORT}:5000 --name registry-lstags registry:2

stop-local-registry:
	docker rm -f registry-lstags || true

push-to-local-registry: REPOSITORIES:=alpine:latest busybox:latest
push-to-local-registry: REGISTRY_PORT:=5757
push-to-local-registry:
	./lstags --push-registry=localhost:${REGISTRY_PORT} ${REPOSITORIES}

stress-test: YAML_CONFIG:=./fixtures/config/config-stress.yaml
stress-test: CONCURRENT_REQUESTS:=64
stress-test:
	./lstags --yaml-config=${YAML_CONFIG} --concurrent-requests=${CONCURRENT_REQUESTS}

stress-test-async: YAML_CONFIG:=./fixtures/config/config-stress.yaml
stress-test-async: CONCURRENT_REQUESTS:=64
stress-test-async:
	@scripts/async-run.sh stress-test make stress-test YAML_CONFIG=${YAML_CONFIG} CONCURRENT_REQUESTS=${CONCURRENT_REQUESTS}

stress-test-wait: TIME:=180
stress-test-wait: MODE:=verbose
stress-test-wait:
	@scripts/async-wait.sh stress-test ${TIME} ${MODE}

lint: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -I {} golint {} | tr '`' '|')
lint: fail-on-errors

vet: ERRORS=$(shell go vet)
vet: fail-on-errors

semantic: REGEX:="^(feat|fix|docs|style|refactor|test|chore|localize)(\([a-zA-Z0-9\/_-]+\))?: [a-zA-Z]"
semantic:
	@if [[ -n "${RANGE}" ]]; then \
		git log --pretty="format:%s" ${RANGE} | grep -v "Merge pull request" \
		| egrep -v ${REGEX} | awk '{print "NON-SEMANTIC: "$$0}' | grep . \
		&& echo -e "\e[1m\e[31mFATAL: Non-semantic commit messages found (${RANGE})!\e[0m" && exit 1 || echo "OK"; \
	else \
		echo -e "\e[33mERROR: Please define 'RANGE' variable!\e[0m"; exit 1; \
	fi

fail-on-errors:
	@echo "${ERRORS}" | grep . || echo "OK"
	@test `echo "${ERRORS}" | grep . | wc -l` -eq 0

./dist/assets/lstags-linux/lstags: export GOOS:=linux
./dist/assets/lstags-linux/lstags:
	mkdir -p dist/assets/lstags-linux; \
		go build -mod=vendor -ldflags '-s -w' -a -tags netgo -installsuffix netgo -o dist/assets/lstags-linux/lstags

docker-image: DOCKER_REPO:=ivanilves/lstags
docker-image: DOCKER_TAG:=latest
docker-image: ./dist/assets/lstags-linux/lstags
docker-image:
	@docker image build --no-cache -t ${DOCKER_REPO}:${DOCKER_TAG} .

build: LDFLAGS=$(shell test `uname -s` = "Darwin" && echo '-s -w' || echo '-d -s -w')
build: NAME=$(shell test "${GOOS}" = "windows" && echo 'lstags.exe' || echo 'lstags')
build:
	@if [[ -z "${GOOS}" ]]; then go build -mod=vendor -ldflags '${LDFLAGS}' -a -tags netgo -installsuffix netgo; fi
	@if [[ -n "${GOOS}" ]]; then mkdir -p dist/assets/lstags-${GOOS}; GOOS=${GOOS} go build -mod=vendor -ldflags '-s -w' -a -tags netgo -installsuffix netgo -o dist/assets/lstags-${GOOS}/${NAME}; fi

xbuild:
	${MAKE} --no-print-directory build GOOS=linux
	${MAKE} --no-print-directory build GOOS=darwin
	${MAKE} --no-print-directory build GOOS=windows

changelog: LAST_RELEASED_TAG:=$(shell git tag --sort=creatordate | tail -n1)
changelog: GITHUB_COMMIT_URL:=https://github.com/ivanilves/lstags/commit
changelog:
	@echo "## Changelog" \
	&& git log --oneline --reverse ${LAST_RELEASED_TAG}..HEAD | egrep -iv "^[0-9a-f]{7,} (Merge pull request |Merge branch |.*NORELEASE)" | \
		sed -r "s|^([0-9a-f]{7,}) (.*)|* [\`\1\`](${GITHUB_COMMIT_URL}/\1) \2|"

release: clean
release: LAST_BUILD_NUMBER:=$(shell git fetch --tags && git tag --sort=creatordate | grep "^v${API_VERSION}\."  | tail -n1 | sed 's/^v.*\.//' | grep . || echo -1)
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

deploy: DO_RELEASE:=$(shell git log --oneline -n1 | grep -i "Merge.*NORELEASE" >/dev/null && echo "false" || echo "true")
deploy: deploy-github deploy-docker

deploy-github: TAG=$(shell cat ./dist/release/TAG)
deploy-github:
	@if [[ "${DO_RELEASE}" == "true" ]]; then \
		${MAKE} --no-print-directory validate-release \
		&& test -n "${GITHUB_TOKEN}" && git tag ${TAG} && git push --tags \
		&& GITHUB_TOKEN=${GITHUB_TOKEN} scripts/github-create-release.sh ./dist/release \
		&& GITHUB_TOKEN=${GITHUB_TOKEN} scripts/github-upload-assets.sh ${TAG} ./dist/assets; \
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

poc-app: APP_PATH=../lstags-api
poc-app: prepare dep
poc-app:
	@echo -e "\e[1mInitializing PoC application:\e[0m" \
		&& mkdir -p ${APP_PATH} \
		&& cp api_poc.go.sample ${APP_PATH}/main.go \
		&& pushd ${APP_PATH} >/dev/null; go build -mod=vendor; pwd; popd >/dev/null \
		&& echo -e "\e[31mHINT: Set 'APP_PATH' makefile variable to adjust PoC application path ;)\e[0m"

wrapper: PARAMS?=$(shell test "$${UID}" = "0" && echo "-o root -g 0 -m755" || echo "-m755")
wrapper: PREFIX:=/usr/local
wrapper:
	install ${PARAMS} scripts/wrapper.sh ${PREFIX}/bin/lstags

install: wrapper
