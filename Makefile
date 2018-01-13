.PHONY: docker

all: prepare dep test lint vet build

offline: unit-test lint vet build

prepare:
	go get -u -v \
		github.com/golang/dep/cmd/dep \
		github.com/golang/lint/golint

dep:
	dep ensure -v

test: unit-test whitebox-integration-test

unit-test:
	@find \
		-mindepth 2 -type f ! -path "./vendor/*" -name "*_test.go" \
		| xargs dirname \
		| xargs -i sh -c "pushd {}; go test -v || exit 1; popd"

whitebox-integration-test:
	go test -v

env:
	env

docker-json:
	test -n "${DOCKER_JSON}" && mkdir -p `dirname "${DOCKER_JSON}"` && touch "${DOCKER_JSON}" && chmod 0600 "${DOCKER_JSON}" \
		&& echo "{ \"auths\": { \"registry.hub.docker.com\": { \"auth\": \"${DOCKERHUB_AUTH}\" } } }" >${DOCKER_JSON}

start-local-registry: REGISTRY_PORT=5757
start-local-registry:
	docker rm -f lstags-registry &>/dev/null || true
	docker run -d -p ${REGISTRY_PORT}:5000 --name lstags-registry registry:2

stop-local-registry:
	docker rm -f lstags-registry

blackbox-integration-test: build \
	start-local-registry \
	shell-test-alpine \
	shell-test-wrong-image \
	shell-test-pull-public \
	shell-test-pull-private \
	shell-test-push-local-alpine \
	shell-test-push-local-assumed-tags \
	stop-local-registry

shell-test-alpine:
	./lstags alpine | egrep "\salpine:latest"

shell-test-wrong-image:
	./lstags nobody/nothing &>/dev/null && exit 1 || true

shell-test-pull-public: DOCKERHUB_PUBLIC_REPO?=ivanilves/dummy
shell-test-pull-public:
	./lstags --pull ${DOCKERHUB_PUBLIC_REPO}~/latest/

shell-test-pull-private: DOCKER_JSON:=tmp/docker.json.private-repo
shell-test-pull-private: docker-json
	if [[ -n "${DOCKERHUB_PRIVATE_REPO}" && -n "${DOCKERHUB_AUTH}" ]]; then\
		./lstags -j "${DOCKER_JSON}" --pull ${DOCKERHUB_PRIVATE_REPO}~/latest/; \
		else \
		echo "DOCKERHUB_PRIVATE_REPO or DOCKERHUB_AUTH not set!"; \
	fi

shell-test-push-local-alpine: REGISTRY_PORT=5757
shell-test-push-local-alpine:
	./lstags --push-registry=localhost:${REGISTRY_PORT} --push-prefix=/qa alpine~/3.6/
	./lstags localhost:${REGISTRY_PORT}/qa/library/alpine

shell-test-push-local-assumed-tags: REGISTRY_PORT=5757
shell-test-push-local-assumed-tags:
	@echo "NB! quay.io does not expose certain tags via API, so we need to 'believe' they exist."
	./lstags --push-registry=localhost:${REGISTRY_PORT} --push-prefix=/qa quay.io/calico/cni~/^v1\\.[67]/
	@echo "NB! Following command SHOULD fail, because no tags should be loaded without 'assumption'!"
	./lstags localhost:${REGISTRY_PORT}/qa/calico/cni | egrep "v1\.(6\.1|7\.0)" && exit 1 || true
	@echo "NB! Following command is assuming tags 'v1.6.1' and 'v1.7.0' do exist and will be loaded anyway."
	./lstags --push-registry=localhost:${REGISTRY_PORT} --push-prefix=/qa quay.io/calico/cni~/^v1\\.[67]/=v1.6.1,v1.7.0
	@echo "NB! This should NOT fail, because above we assumed tags 'v1.6.1' and 'v1.7.0' do exist."
	./lstags localhost:${REGISTRY_PORT}/qa/calico/cni | egrep "v1\.(6\.1|7\.0)"

test-docker-socket:
	unset DOCKER_HOST && ./lstags alpine~/latest/

test-docker-tcp: DOCKER_HOST=tcp://127.0.0.1:2375
test-docker-tcp:
	./lstags alpine~/latest/

lint: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {} | tr '`' '|')
lint: fail-on-errors

vet: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i go tool vet {} | tr '`' '|')
vet: fail-on-errors

fail-on-errors:
	@echo "${ERRORS}" | grep . || echo "OK"
	@test `echo "${ERRORS}" | grep . | wc -l` -eq 0

build:
	@if [[ -z "${GOOS}" ]]; then go build -ldflags '-d -s -w' -a -tags netgo -installsuffix netgo; fi
	@if [[ -n "${GOOS}" ]]; then mkdir -p dist/assets/lstags-${GOOS}; GOOS=${GOOS} go build -ldflags '-d -s -w' -a -tags netgo -installsuffix netgo -o dist/assets/lstags-${GOOS}/lstags; fi

xbuild:
	${MAKE} --no-print-directory build GOOS=linux
	${MAKE} --no-print-directory build GOOS=darwin

clean:
	rm -rf ./lstags ./dist/

changelog: LAST_RELEASE?=$(shell git tag | sed 's/^v//' | sort -n | tail -n1)
changelog: GITHUB_COMMIT_URL:=https://github.com/ivanilves/lstags/commit
changelog:
	@echo "## Changelog"
	@git log --oneline --reverse v${LAST_RELEASE}..HEAD | egrep -iv "^[0-9a-f]{7,} (Merge pull request |Merge branch |NORELEASE)" | \
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
	test -f ./dist/release/CHANGELOG.md
	[[ `find dist/assets -mindepth 2 -type f | wc -l` -ge 2 ]]

deploy: TAG=$(shell cat ./dist/release/TAG)
deploy: NORELEASE_MERGE=$(shell git show | grep -i "Merge.*NORELEASE" >/dev/null && echo "true" || echo "false")
deploy:
	@if [[ "${NORELEASE_MERGE}" == "false" ]]; then \
		${MAKE} --no-print-directory validate-release \
		&& test -n "${GITHUB_TOKEN}" && git tag ${TAG} && git push --tags \
		&& GITHUB_TOKEN=${GITHUB_TOKEN} ./scripts/github-create-release.sh ./dist/release \
		&& GITHUB_TOKEN=${GITHUB_TOKEN} ./scripts/github-upload-assets.sh ${TAG} ./dist/assets; \
	else \
		echo "NB! Release skipped because of 'NORELEASE' branch merge!"; \
	fi

docker: DOCKER_REPO:=ivanilves/lstags
docker: RELEASE_TAG:=latest
docker:
	@docker image build -t ${DOCKER_REPO}:${RELEASE_TAG} .

wrapper: PREFIX=/usr/local
wrapper:
	install -o root -g root -m755 scripts/wrapper.sh ${PREFIX}/bin/lstags

install: wrapper
