package registry

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/util/getenv"
	"github.com/ivanilves/lstags/util/wait"
)

const (
	imageRef   = "registry:2"
	baseName   = "registry"
	basePort   = 5000
	retryCount = 3
)

// Container is a Docker container running Docker registry inside
type Container struct {
	id           string
	hostname     string
	dockerClient *dockerclient.DockerClient
}

func getRandomPort() int {
	b := make([]byte, 1)

	rand.Read(b)

	return basePort + int(b[0])
}

func getDockerClient() (*dockerclient.DockerClient, error) {
	dockerConfig, err := dockerconfig.Load(dockerconfig.DefaultDockerJSON)
	if err != nil {
		return nil, err
	}

	dockerClient, err := dockerclient.New(dockerConfig)
	if err != nil {
		return nil, err
	}

	return dockerClient, nil
}

func getHostname(port int) string {
	return fmt.Sprintf("%s:%d", getenv.String("LOCAL_REGISTRY", "127.0.0.1"), port)
}

func run(dockerClient *dockerclient.DockerClient, hostPort int) (string, error) {
	portSpec := fmt.Sprintf("0.0.0.0:%d:%d", hostPort, basePort)

	name := fmt.Sprintf("%s-%d", baseName, hostPort)

	id, err := dockerClient.Run(imageRef, name, []string{portSpec})
	if err != nil {
		return "", err
	}

	return id, nil
}

func verify(hostname string) error {
	url := fmt.Sprintf("http://%s/v2/", hostname)

	var err error
	var resp *http.Response
	for retry := 0; retry < retryCount; retry++ {
		time.Sleep(1 * time.Second)

		resp, err = http.Get(url)
		if err == nil {
			break
		}
	}

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("Unexpected status code from '%s': %d", url, resp.StatusCode)
	}

	return nil
}

// LaunchContainer launches a Docker container with Docker registry inside
func LaunchContainer() (*Container, error) {
	dockerClient, err := getDockerClient()
	if err != nil {
		return nil, err
	}

	hostPort := getRandomPort()

	id, err := run(dockerClient, hostPort)
	if err != nil {
		return nil, err
	}

	hostname := getHostname(hostPort)

	if err := verify(hostname); err != nil {
		dockerClient.ForceRemove(id)
		return nil, err
	}

	return &Container{id: id, hostname: hostname, dockerClient: dockerClient}, nil
}

// ID gets container ID
func (c *Container) ID() string {
	return c.id
}

// Hostname gets hostname to access Docker registry we run inside our container
func (c *Container) Hostname() string {
	return c.hostname
}

// Destroy force-stops and destroys Docker container with registry
func (c *Container) Destroy() error {
	return c.dockerClient.ForceRemove(c.id)
}

// SeedWithImages pulls specified images from whatever registry
// and then re-pushes em to our [local] container-based registry
func (c *Container) SeedWithImages(refs ...string) ([]string, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("must pass one or more image references")
	}

	pushRefs := make([]string, len(refs))

	done := make(chan error, len(refs))

	for i, ref := range refs {
		go func(i int, ref string) {
			repo, err := repository.ParseRef(ref)
			if err != nil {
				done <- err
				return
			}
			if !repo.IsSingle() {
				done <- fmt.Errorf("invalid reference: %s (only REPOSITORY:TAG form is allowed)", ref)
				return
			}

			tag := repo.Tags()[0]

			pushRef := fmt.Sprintf("%s%s/%s:%s", c.hostname, repo.PushPrefix(), repo.Path(), tag)
			pushRepo, err := repository.ParseRef(pushRef)
			if err != nil {
				done <- err
				return
			}

			src := fmt.Sprintf("%s:%s", repo.Name(), tag)
			dst := fmt.Sprintf("%s:%s", pushRepo.Name(), tag)

			if err := c.dockerClient.Pull(src); err != nil {
				done <- err
				return
			}
			if err := c.dockerClient.Tag(src, dst); err != nil {
				done <- err
				return
			}
			if err := c.dockerClient.Push(dst); err != nil {
				done <- err
				return
			}

			pushRefs[i] = pushRef

			done <- nil
		}(i, ref)
	}

	if err := wait.Until(done); err != nil {
		return nil, err
	}

	return pushRefs, nil
}
