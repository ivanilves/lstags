package container

import (
	"bufio"
	crand "crypto/rand"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

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
	retryCount = 10
)

// Container is a Docker container running Docker registry inside
type Container struct {
	id           string
	hostname     string
	dockerClient *dockerclient.DockerClient
}

func getRandomPort() int {
	rand.Seed(time.Now().UnixNano())

	b := make([]byte, 1)

	crand.Read(b)

	return basePort + int(b[0]) + rand.Intn(400)
}

func getDockerClient() (*dockerclient.DockerClient, error) {
	dockerConfig, _ := dockerconfig.Load(dockerconfig.DefaultDockerJSON)

	return dockerclient.New(dockerConfig)
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
	for retry := 0; retry < retryCount; retry++ {
		time.Sleep(1 * time.Second)

		_, err = http.Get(url)
		if err == nil {
			break
		}
	}

	return err
}

// Launch launches a Docker container with Docker registry inside
func Launch() (*Container, error) {
	dockerClient, _ := getDockerClient()

	hostPort := getRandomPort()

	id, _ := run(dockerClient, hostPort)

	hostname := getHostname(hostPort)

	return &Container{id: id, hostname: hostname, dockerClient: dockerClient}, verify(hostname)
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

			pushRef := fmt.Sprintf("%s%s%s:%s", c.hostname, repo.PushPrefix(), repo.Path(), tag)

			pushRepo, _ := repository.ParseRef(pushRef)

			src := fmt.Sprintf("%s:%s", repo.Name(), tag)
			dst := fmt.Sprintf("%s:%s", pushRepo.Name(), tag)

			pushRefs[i] = pushRef

			pullResp, err := c.dockerClient.Pull(src)
			if err != nil {
				done <- err
				return
			}
			logDebugData(pullResp)

			c.dockerClient.Tag(src, dst)

			pushResp, err := c.dockerClient.Push(dst)
			if err != nil {
				done <- err
				return
			}
			logDebugData(pushResp)

			done <- err
		}(i, ref)
	}

	return pushRefs, wait.Until(done)
}

func logDebugData(data io.Reader) {
	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		log.Debug(scanner.Text())
	}
}
