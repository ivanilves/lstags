/*

NB!
NB! "main" package tests are only for integration testing
NB! "main" package is bare and all unit tests are put into packages
NB!

*/
package main

import (
	"testing"

	"math/rand"
	"os"
	"strconv"

	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag/remote"
)

func getEnvOrDefault(name, defaultValue string) string {
	value := os.Getenv(name)

	if value != "" {
		return value
	}

	return defaultValue
}

//
// Here we check the ability to fetch tags from remote registry
//
func runTestForFetchTags(
	t *testing.T,
	repoName, filter string,
	username, password string,
	checkTagNames []string,
) {
	repo, _ := repository.ParseRef(repoName)

	registry := repo.Registry()
	repoPath := repo.Path()

	tags, err := remote.FetchTags(registry, repoPath, filter, username, password, repo.WebSchema())
	if err != nil {
		t.Fatalf(
			"Failed to fetch tags (%s~/%s/) from '%s' registry: %s",
			repoPath,
			filter,
			registry,
			err.Error(),
		)
	}

	for _, name := range checkTagNames {
		_, defined := tags[name]
		if !defined {
			t.Fatalf(
				"Tag '%s' not found in query (%s~/%s/) to '%s' registry.\nTags: %#v",
				name,
				repoPath,
				filter,
				registry,
				tags,
			)
		}
	}
}

func TestFetchTags_DockerHub_PublicRepo(t *testing.T) {
	runTestForFetchTags(
		t,
		"alpine",
		"^(3.6|latest)$",
		"",
		"",
		[]string{"3.6", "latest"},
	)
}

func TestFetchTags_DockerHub_PrivateRepo(t *testing.T) {
	if os.Getenv("DOCKERHUB_PRIVATE_REPO") == "" {
		t.Skipf("DOCKERHUB_PRIVATE_REPO environment variable not set!")
	}
	if os.Getenv("DOCKERHUB_USERNAME") == "" {
		t.Skipf("DOCKERHUB_USERNAME environment variable not set!")
	}
	if os.Getenv("DOCKERHUB_PASSWORD") == "" {
		t.Skipf("DOCKERHUB_PASSWORD environment variable not set!")
	}

	repository := os.Getenv("DOCKERHUB_PRIVATE_REPO")
	username := os.Getenv("DOCKERHUB_USERNAME")
	password := os.Getenv("DOCKERHUB_PASSWORD")

	runTestForFetchTags(
		t,
		repository,
		".*",
		username,
		password,
		[]string{"latest"},
	)
}

//
// Here we check if we could:
// * Pull specified images from remote registry
// * Push them to the local registry then (ephemeral container used to run registry)
// * Check if images pushed to the local registry are present there
//
func runTestForPullPush(
	t *testing.T,
	srcRepository, filter string,
	username, password string,
	checkTagNames []string,
) {
	const dockerJSON = "./fixtures/docker/config.json"
	const registryImageRef = "registry:2"
	const registryContainerName = "lstags-ephemeral-registry"

	println("[will use] DOCKER_HOST: " + getEnvOrDefault("DOCKER_HOST", "unix:///var/run/docker.sock"))

	hostPort := strconv.Itoa(5000 + rand.Intn(1000))
	localRegistry := getEnvOrDefault("LOCAL_REGISTRY", "127.0.0.1") + ":" + hostPort
	localPortSpec := "0.0.0.0:" + hostPort + ":5000"

	dockerConfig, err := dockerconfig.Load(dockerJSON)
	if err != nil {
		t.Fatal(err)
	}

	dc, err := dockerclient.New(dockerConfig)
	if err != nil {
		t.Fatal(err)
	}

	dc.ForceRemove(registryContainerName)

	id, err := dc.Run(
		registryImageRef,
		registryContainerName,
		[]string{localPortSpec},
	)
	if err != nil {
		t.Fatal(err)
	}

	srcRepo, _ := repository.ParseRef(srcRepository)

	srcRepoPath := srcRepo.Path()
	dstRepoPath := "lstags/" + srcRepoPath
	dstRepository := localRegistry + "/" + dstRepoPath

	for _, name := range checkTagNames {
		src := srcRepository + ":" + name
		dst := dstRepository + ":" + name

		if err := dc.Pull(src); err != nil {
			t.Fatal(err)
		}

		if err := dc.Tag(src, dst); err != nil {
			t.Fatal(err)
		}

		if err := dc.Push(dst); err != nil {
			t.Fatal(err)
		}
	}

	dc.ForceRemove(id)
}

func TestPullPush_DockerHub_PublicRegistry(t *testing.T) {
	runTestForPullPush(t, "alpine", "^(3.6|latest)$", "", "", []string{"3.6", "latest"})
}
