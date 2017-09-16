package main

import (
	"testing"

	"flag"
	"os"

	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/tag/registry"
)

var runIntegrationTests = flag.Bool("integration", false, "run integration tests")

const dockerHub = "registry.hub.docker.com"

type MockedTokenResponse struct {
}

func (tr MockedTokenResponse) Token() string {
	return "8c896241e2774507489849ab1981e582"
}

func (tr MockedTokenResponse) Method() string {
	return "Mocked"
}

func (tr MockedTokenResponse) ExpiresIn() int {
	return 0
}

func TestGetAuthorization(t *testing.T) {
	flag.Parse()
	if *runIntegrationTests {
		t.SkipNow()
	}

	const expected = "Mocked 8c896241e2774507489849ab1981e582"

	authorization := getAuthorization(MockedTokenResponse{})

	if authorization != expected {
		t.Fatalf(
			"Unexpected authorization string: '%s' (expected: '%s')",
			authorization,
			expected,
		)
	}
}

func TestDockerHubWithPublicRepo(t *testing.T) {
	flag.Parse()
	if !*runIntegrationTests {
		t.SkipNow()
	}

	const repo = "library/alpine"

	tresp, err := auth.NewToken(dockerHub, repo, "", "")
	if err != nil {
		t.Fatalf("Failed to get DockerHub public repo token: %s", err.Error())
	}

	authorization := getAuthorization(tresp)

	tags, err := registry.FetchTags(dockerHub, repo, authorization, 128)
	if err != nil {
		t.Fatalf("Failed to list DockerHub public repo (%s) tags: %s", repo, err.Error())
	}

	_, defined := tags["latest"]
	if !defined {
		t.Fatalf("DockerHub public repo (%s) tag 'latest' not found: %#v", repo, tags)
	}
}

func TestDockerHubWithPrivateRepo(t *testing.T) {
	flag.Parse()
	if !*runIntegrationTests {
		t.SkipNow()
	}

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

	tresp, err := auth.NewToken(dockerHub, repo, user, pass)
	if err != nil {
		t.Fatalf("Failed to get DockerHub private repo token: %s", err.Error())
	}

	authorization := getAuthorization(tresp)

	tags, err := registry.FetchTags(dockerHub, repo, authorization, 128)
	if err != nil {
		t.Fatalf("Failed to list DockerHub private repo (%s) tags: %s", repo, err.Error())
	}

	_, defined := tags["latest"]
	if !defined {
		t.Fatalf("DockerHub private repo (%s) tag 'latest' not found: %#v", repo, tags)
	}
}
