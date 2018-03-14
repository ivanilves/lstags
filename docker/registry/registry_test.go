package registry

import (
	"testing"

	"fmt"
	"strings"

	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/util/wait"
)

func TestRandomPort(t *testing.T) {
	const repeat = 5

	memory := make(map[int]int)

	for r := 0; r < repeat; r++ {
		port := getRandomPort()

		n, defined := memory[port]
		if defined {
			t.Fatalf(
				"already got port %d at repetition %d (current: %d)",
				port, n, r,
			)
		}

		memory[port] = r
	}
}

func TestGetHostname(t *testing.T) {
	port := getRandomPort()

	hostname := getHostname(port)
	endsWith := fmt.Sprintf(":%d", port)

	if !strings.HasSuffix(hostname, endsWith) {
		t.Fatalf("'%s' does not end with '%s'", hostname, endsWith)
	}
}

func TestRun(t *testing.T) {
	dc, _ := getDockerClient()

	port := getRandomPort()

	if _, err := run(dc, port); err != nil {
		t.Fatal(err.Error())
	}
}

func TestRunGuaranteedFailure(t *testing.T) {
	dc, _ := getDockerClient()

	const port = 2375

	if _, err := run(dc, port); err == nil {
		t.Fatal("how could you forward Docker's own port?")
	}
}

func testVerify(t *testing.T) {
	c, _ := LaunchContainer()

	defer c.Destroy()

	if err := verify(c.Hostname()); err != nil {
		t.Fatal(err.Error())
	}
}

func testVerifyGuaranteedFailure(t *testing.T) {
	const badHostname = "i.do.not.exist:8888"

	if err := verify(badHostname); err == nil {
		t.Fatalf("shoud fail on bad hostname: %s", badHostname)
	}
}

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
	const createContainers = 5

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
