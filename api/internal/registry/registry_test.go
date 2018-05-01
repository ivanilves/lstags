package registry

import (
	"testing"

	"fmt"
	"regexp"
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

	const idExpr = "^[a-f0-9]{64}$"
	if matched, _ := regexp.MatchString(idExpr, c.ID()); !matched {
		t.Fatalf("id '%s' does not match regex: %s", c.ID(), idExpr)
	}

	const hostnameExpr = "^[a-z0-9][a-z0-9\\-\\.]+[a-z0-9]:[0-9]{4,5}$"
	if matched, _ := regexp.MatchString(hostnameExpr, c.Hostname()); !matched {
		t.Fatalf("hostname '%s' does not match regex: %s", c.Hostname(), hostnameExpr)
	}

	if err := c.Destroy(); err != nil {
		t.Fatal(err)
	}

	if err := c.Destroy(); err == nil {
		t.Fatalf("Container can not be destroyed more than once: %s", c.ID())
	}
}

func TestLaunchManyContainersWithoutNamingCollisions(t *testing.T) {
	const createContainers = 3

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
			resp, err := dockerClient.Pull(ref)
			if err != nil {
				done <- err
				return
			}

			logDebugData(resp)

			done <- nil
		}(ref)
	}

	if err := wait.Until(done); err != nil {
		t.Fatal(err)
	}
}

func TestSeedContainerWithImagesGuaranteedFailure(t *testing.T) {
	c, err := LaunchContainer()
	if err != nil {
		t.Fatal(err)
	}

	defer c.Destroy()

	if _, err := c.SeedWithImages(); err == nil {
		t.Fatal("should not process nil as image list")
	}

	if _, err := c.SeedWithImages([]string{}...); err == nil {
		t.Fatal("should not process empty image list")
	}

	if _, err := c.SeedWithImages([]string{"", "", ""}...); err == nil {
		t.Fatal("should not process list of empty strings")
	}

	if _, err := c.SeedWithImages([]string{"1u[pine~!.*/"}...); err == nil {
		t.Fatal("should not process invalid references")
	}

	if _, err := c.SeedWithImages([]string{"alpine"}...); err == nil {
		t.Fatal("should not process references without tag specified")
	}
}
