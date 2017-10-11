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

const dockerJSON = "./fixtures/docker/config.json"

func TestGetVersion(t *testing.T) {
	flag.Parse()
	if *runIntegrationTests {
		t.SkipNow()
	}

	const expected = "CURRENT"

	version := getVersion()

	if version != expected {
		t.Fatalf(
			"Unexpected version: '%s' (expected: '%s')",
			version,
			expected,
		)
	}
}

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

func TestTrimFilter(t *testing.T) {
	flag.Parse()
	if *runIntegrationTests {
		t.SkipNow()
	}

	expected := []struct {
		repoWithFilter string
		repo           string
		filter         string
		iserr          bool
	}{
		{"nginx", "nginx", ".*", false},
		{"registry.hipster.io/hype/sdn", "registry.hipster.io/hype/sdn", ".*", false},
		{"mesosphere/mesos~/^1\\.[0-9]+\\.[0-9]+$/", "mesosphere/mesos", "^1\\.[0-9]+\\.[0-9]+$", false},
		{"registry.hipster.io/hype/drone~/v[0-9]+$/", "registry.hipster.io/hype/drone", "v[0-9]+$", false},
		{"bogohost:5000/hype/drone~/v[0-9]+$/", "bogohost:5000/hype/drone", "v[0-9]+$", false},
		{"registry.clown.bad/cache/merd~x[0-9]", "", "", true},
		{"cabron/~plla~x~", "", "", true},
	}

	for _, e := range expected {
		repo, filter, err := trimFilter(e.repoWithFilter)

		if repo != e.repo {
			t.Fatalf(
				"Unexpected repository name '%s' trimmed from '%s' (expected: '%s')",
				repo,
				e.repoWithFilter,
				e.repo,
			)
		}

		if filter != e.filter {
			t.Fatalf(
				"Unexpected repository filter '%s' trimmed from '%s' (expected: '%s')",
				filter,
				e.repoWithFilter,
				e.filter,
			)
		}

		iserr := err != nil
		if iserr != e.iserr {
			t.Fatalf("Passing badly formatted repository '%s' should trigger an error", e.repoWithFilter)
		}
	}
}

func TestMatchesFilter(t *testing.T) {
	flag.Parse()
	if *runIntegrationTests {
		t.SkipNow()
	}

	expected := []struct {
		s       string
		pattern string
		matched bool
	}{
		{"latest", "^latest$", true},
		{"v1.0.1", "^v1\\.0\\.1$", true},
		{"barbos", ".*", true},
		{"3.4", "*", false},
	}

	for _, e := range expected {
		matched := matchesFilter(e.s, e.pattern)

		action := "should"
		if !e.matched {
			action = "should not"
		}

		if matched != e.matched {
			t.Fatalf(
				"String '%s' %s match pattern '%s'",
				e.s,
				action,
				e.pattern,
			)
		}
	}
}

func TestGetRegistryName(t *testing.T) {
	flag.Parse()
	if *runIntegrationTests {
		t.SkipNow()
	}

	expected := map[string]string{
		"mesosphere/marathon":             dockerHub,
		"bogohost/my/inner/troll":         dockerHub,
		"registry.hipsta.io/hype/hotshit": "registry.hipsta.io",
		"localhost/my/image":              "localhost",
		"bogohost:5000/mymymy/img":        "bogohost:5000",
	}

	for repo, expectedRegistryName := range expected {
		registryName := getRegistryName(repo, dockerHub)

		if registryName != expectedRegistryName {
			t.Fatalf(
				"Got unexpected Docker registry name '%s' from repo '%s' (expected: '%s')",
				registryName,
				repo,
				expectedRegistryName,
			)
		}
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
