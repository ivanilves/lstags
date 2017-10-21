package client

import (
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"

	"golang.org/x/net/context"

	"github.com/ivanilves/lstags/docker"
	"github.com/ivanilves/lstags/docker/client/api/version"
	"github.com/ivanilves/lstags/docker/config"
)

// DockerSocket is a socket we use to connect to the Docker daemon
var DockerSocket = "/var/run/docker.sock"

// DockerClient is a raw Docker client convenience wrapper
type DockerClient struct {
	cli *client.Client
	cnf *config.Config
}

// New creates new instance of DockerClient (our Docker client wrapper)
func New(cnf *config.Config) (*DockerClient, error) {
	apiVersion, err := version.Detect(DockerSocket)
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClient("unix://"+DockerSocket, apiVersion, nil, nil)
	if err != nil {
		return nil, err
	}

	return &DockerClient{cli: cli, cnf: cnf}, nil
}

// ListImagesForRepo lists images present locally for the repo specified
func (dc *DockerClient) ListImagesForRepo(repo string) ([]types.ImageSummary, error) {
	listOptions, err := buildImageListOptions(repo)
	if err != nil {
		return nil, err
	}

	return dc.cli.ImageList(context.Background(), listOptions)
}

func buildImageListOptions(repo string) (types.ImageListOptions, error) {
	repoFilter := "reference=" + repo
	filterArgs := filters.NewArgs()

	filterArgs, err := filters.ParseFlag(repoFilter, filterArgs)
	if err != nil {
		return types.ImageListOptions{}, err
	}

	return types.ImageListOptions{Filters: filterArgs}, nil
}

// Pull pulls Docker image specified
func (dc *DockerClient) Pull(ref string) error {
	registryAuth := dc.cnf.GetRegistryAuth(
		docker.GetRegistry(ref),
	)

	pullOptions := types.ImagePullOptions{RegistryAuth: registryAuth}
	if registryAuth == "" {
		pullOptions = types.ImagePullOptions{}
	}

	resp, err := dc.cli.ImagePull(context.Background(), ref, pullOptions)
	if err != nil {
		return err
	}

	_, err = ioutil.ReadAll(resp)

	return err
}

// Push pushes Docker image specified
func (dc *DockerClient) Push(ref string) error {
	registryAuth := dc.cnf.GetRegistryAuth(
		docker.GetRegistry(ref),
	)

	pushOptions := types.ImagePushOptions{RegistryAuth: registryAuth}
	if registryAuth == "" {
		pushOptions = types.ImagePushOptions{RegistryAuth: "IA=="}
	}

	resp, err := dc.cli.ImagePush(context.Background(), ref, pushOptions)
	if err != nil {
		return err
	}

	_, err = ioutil.ReadAll(resp)

	return err
}

// Tag puts a "dst" tag on "src" Docker image
func (dc *DockerClient) Tag(src, dst string) error {
	return dc.cli.ImageTag(context.Background(), src, dst)
}
