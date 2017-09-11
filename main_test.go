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

func TestShortify(t *testing.T) {
	const cutToLength = 10

	shortString := "so short!"
	longString := "size does matter after all!"

	var resultString string

	resultString = shortify(shortString, cutToLength)
	if resultString != shortString {
		t.Fatalf(
			"String with length <= %d should not be modified (We got: %s => %s)",
			cutToLength,
			shortString,
			resultString,
		)
	}

	resultString = shortify(longString, cutToLength)
	if len(resultString) != cutToLength {
		t.Fatalf(
			"String with length > %d should be cut exactly to this length (We got: %s => %s, length: %d)",
			cutToLength,
			longString,
			resultString,
			len(resultString),
		)
	}
	if resultString != longString[0:cutToLength] {
		t.Fatalf(
			"Should return first %d characters of the passed string (We got: %s => %s)",
			cutToLength,
			longString,
			resultString,
		)
	}
}

func TestGetRepoRegistryName(t *testing.T) {
	const registry = "registry.nerd.io"

	expectations := map[string]string{
		"nginx": "library/nginx",
		"registry.nerd.io/hype/cube": "hype/cube",
		"observability/metrix":       "observability/metrix",
	}

	for input, expected := range expectations {
		output := getRepoRegistryName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected registry repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestGetRepoLocalNameForPublicRegistry(t *testing.T) {
	const registry = "registry.hub.docker.com"

	expectations := map[string]string{
		"library/nginx": "nginx",
		"hype/cube":     "hype/cube",
	}

	for input, expected := range expectations {
		output := getRepoLocalName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected local repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestGetRepoLocalNameForPrivateRegistry(t *testing.T) {
	const registry = "registry.nerd.io"

	expectations := map[string]string{
		"empollon/nginx":             "registry.nerd.io/empollon/nginx",
		"registry.nerd.io/hype/cube": "registry.nerd.io/hype/cube",
	}

	for input, expected := range expectations {
		output := getRepoLocalName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected registry repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
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
