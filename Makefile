.PHONY: default offline prepare dep test unit-test whitebox-integration-test blackbox-integration-test \
	start-local-registry stop-local-registry shell-test-alpine shell-test-wrong-image shell-test-pull-public shell-test-pull-private shell-test-push-local-alpine shell-test-push-local-assumed-tags \
	shell-test-docker-socket shell-test-docker-tcp lint vet fail-on-errors docker-image build xbuild clean changelog release validate-release deploy wrapper install

default: prepare dep test lint vet build

minimal: unit-test lint vet build

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

blackbox-integration-test: build \
	start-local-registry \
	shell-test-alpine \
	shell-test-wrong-image \
	shell-test-pull-public \
	shell-test-pull-private \
	shell-test-push-local-alpine \
	shell-test-push-local-assumed-tags \
	shell-test-docker-socket \
	shell-test-docker-tcp \
	stop-local-registry

start-local-registry: REGISTRY_PORT:=5757
start-local-registry:
	docker rm -f lstags-registry &>/dev/null || true
	docker run -d -p ${REGISTRY_PORT}:5000 --name lstags-registry registry:2

stop-local-registry:
	docker rm -f lstags-registry

shell-test-alpine:
	./lstags alpine | egrep "\salpine:latest"

shell-test-wrong-image:
	./lstags nobody/nothing &>/dev/null && exit 1 || true

shell-test-pull-public: DOCKERHUB_PUBLIC_REPO:=ivanilves/dummy
shell-test-pull-public:
	./lstags --pull ${DOCKERHUB_PUBLIC_REPO}~/latest/

shell-test-pull-private: DOCKER_JSON:=docker.json
shell-test-pull-private:
	if [[ -n "${DOCKERHUB_PRIVATE_REPO}" ]]; then\
		./lstags -j "${DOCKER_JSON}" --pull ${DOCKERHUB_PRIVATE_REPO}~/latest/; \
		else \
		echo "Will not pull from DockerHub private repo: DOCKERHUB_PRIVATE_REPO not set!"; \
	fi

shell-test-push-local-alpine: REGISTRY_PORT:=5757
shell-test-push-local-alpine:
	./lstags --push-registry=localhost:${REGISTRY_PORT} --push-prefix=/qa alpine~/3.6/
	./lstags localhost:${REGISTRY_PORT}/qa/library/alpine

shell-test-push-local-assumed-tags: REGISTRY_PORT:=5757
shell-test-push-local-assumed-tags:
	@echo "NB! quay.io does not expose certain tags via API, so we need to 'believe' they exist."
	./lstags --push-registry=localhost:${REGISTRY_PORT} --push-prefix=/qa quay.io/calico/cni~/^v1\\.[67]/
	@echo "NB! Following command SHOULD fail, because no tags should be loaded without 'assumption'!"
	./lstags localhost:${REGISTRY_PORT}/qa/calico/cni | egrep "v1\.(6\.1|7\.0)" && exit 1 || true
	@echo "NB! Following command is assuming tags 'v1.6.1' and 'v1.7.0' do exist and will be loaded anyway."
	./lstags --push-registry=localhost:${REGISTRY_PORT} --push-prefix=/qa quay.io/calico/cni=v1.6.1,v1.7.0
	@echo "NB! This should NOT fail, because above we assumed tags 'v1.6.1' and 'v1.7.0' do exist."
	./lstags localhost:${REGISTRY_PORT}/qa/calico/cni | egrep "v1\.(6\.1|7\.0)"

shell-test-docker-socket:
	unset DOCKER_HOST && ./lstags alpine~/latest/

shell-test-docker-tcp: DOCKER_HOST:=tcp://127.0.0.1:2375
shell-test-docker-tcp:
	./lstags alpine~/latest/

lint: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i golint {} | tr '`' '|')
lint: fail-on-errors

vet: ERRORS=$(shell find . -name "*.go" ! -path "./vendor/*" | xargs -i go tool vet {} | tr '`' '|')
vet: fail-on-errors

fail-on-errors:
	@echo "${ERRORS}" | grep . || echo "OK"
	@test `echo "${ERRORS}" | grep . | wc -l` -eq 0

docker-image: DOCKER_REPO:=ivanilves/lstags
docker-image: DOCKER_TAG:=latest
docker-image:
	@docker image build -t ${DOCKER_REPO}:${DOCKER_TAG} .

docker-image-async:
	@scripts/async-run.sh docker-image make docker-image

docker-image-wait: DOCKER_REPO:=ivanilves/lstags
docker-image-wait: DOCKER_TAG:=latest
docker-image-wait: TIMEOUT:=60
docker-image-wait:
	@scripts/async-wait.sh docker-image ${TIMEOUT}
	@docker image ls ${DOCKER_REPO}:${DOCKER_TAG} | grep -v "^REPOSITORY" | grep .

build: NAME=$(shell test "${GOOS}" = "windows" && echo 'lstags.exe' || echo 'lstags')
build:
	@if [[ -z "${GOOS}" ]]; then go build -ldflags '-d -s -w' -a -tags netgo -installsuffix netgo; fi
	@if [[ -n "${GOOS}" ]]; then mkdir -p dist/assets/lstags-${GOOS}; GOOS=${GOOS} go build -ldflags '-s -w' -a -tags netgo -installsuffix netgo -o dist/assets/lstags-${GOOS}/${NAME}; fi

xbuild:
	${MAKE} --no-print-directory build GOOS=linux
	${MAKE} --no-print-directory build GOOS=darwin
	${MAKE} --no-print-directory build GOOS=windows

clean:
	rm -rf ./lstags ./dist/ *.log *.pid

changelog: LAST_RELEASE:=$(shell git tag | sed 's/^v//' | sort -n | tail -n1)
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
