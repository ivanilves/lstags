package registry

import (
	"testing"

	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/wait"
)

func TestLaunchContainerAndThanDestroyIt(t *testing.T) {
	c, err := LaunchContainer()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Destroy(); err != nil {
		t.Fatal(err)
	}

	if err := c.Destroy(); err == nil {
		t.Fatalf("Container can not be destroyed more than once: %s", c.ID())
	}
}

func TestLaunchManyContainersWithoutNamingCollisions(t *testing.T) {
	const createContainers = 7

	done := make(chan error, createContainers)

	for c := 0; c < createContainers; c++ {
		go func() {
			c, err := LaunchContainer()
			if err != nil {
				done <- err
				return
			}

			defer c.Destroy()

			done <- nil
		}()
	}

	if err := wait.Until(done); err != nil {
		t.Error(err)
	}
}

func TestSeedContainerWithImages(t *testing.T) {
	c, err := LaunchContainer()
	if err != nil {
		t.Fatal(err)
	}

	defer c.Destroy()

	refs, err := c.SeedWithImages("alpine:3.7", "busybox:latest")
	if err != nil {
		t.Fatal(err)
	}

	dockerConfig, err := dockerconfig.Load(dockerconfig.DefaultDockerJSON)
	if err != nil {
		t.Fatal(err)
	}

	dockerClient, err := dockerclient.New(dockerConfig)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, len(refs))

	for _, ref := range refs {
		go func(ref string) {
			if err := dockerClient.Pull(ref); err != nil {
				done <- err
				return
			}

			done <- nil
		}(ref)
	}

	if err := wait.Until(done); err != nil {
		t.Fatal(err)
	}
}
