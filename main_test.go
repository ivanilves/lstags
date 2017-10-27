/*

NB!
NB! "main" package tests are only for integration testing
NB! "main" package is bare and all unit tests are put into packages
NB!

*/
package main

import (
	"testing"

	"os"

	"github.com/ivanilves/lstags/tag/remote"
)

const dockerHub = "registry.hub.docker.com"

const dockerJSON = "./fixtures/docker/config.json"

func TestDockerHubWithPublicRepo(t *testing.T) {
	const repo = "library/alpine"

	tags, err := remote.FetchTags(dockerHub, repo, ".*", "", "")
	if err != nil {
		t.Fatalf("Failed to list DockerHub public repo (%s) tags: %s", repo, err.Error())
	}

	_, defined := tags["latest"]
	if !defined {
		t.Fatalf("DockerHub public repo (%s) tag 'latest' not found: %#v", repo, tags)
	}
}

func TestDockerHubWithPrivateRepo(t *testing.T) {
	if os.Getenv("DOCKERHUB_USERNAME") == "" {
		t.Skipf("DOCKERHUB_USERNAME environment variable not set!")
	}
	if os.Getenv("DOCKERHUB_PASSWORD") == "" {
		t.Skipf("DOCKERHUB_PASSWORD environment variable not set!")
	}
	if os.Getenv("DOCKERHUB_PRIVATE_REPO") == "" {
		t.Skipf("DOCKERHUB_PRIVATE_REPO environment variable not set!")
	}

	user := os.Getenv("DOCKERHUB_USERNAME")
	pass := os.Getenv("DOCKERHUB_PASSWORD")
	repo := os.Getenv("DOCKERHUB_PRIVATE_REPO")

	tags, err := remote.FetchTags(dockerHub, repo, ".*", user, pass)
	if err != nil {
		t.Fatalf("Failed to list DockerHub private repo (%s) tags: %s", repo, err.Error())
	}

	_, defined := tags["latest"]
	if !defined {
		t.Fatalf("DockerHub private repo (%s) tag 'latest' not found: %#v", repo, tags)
	}
}
