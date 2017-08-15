package main

import (
	"fmt"

	"github.com/jessevdk/go-flags"

	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/registry"
)

type options struct {
	Registry   string `short:"r" long:"registry" default:"https://registry.hub.docker.com" description:"Docker registry to use" env:"REGISTRY"`
	Positional struct {
		Repository string `positional-arg-name:"REPOSITORY" description:"Docker repository to list tags from"`
	} `positional-args:"yes" required:"yes"`
}

func main() {
	o := options{}

	_, err := flags.Parse(&o)
	if err != nil {
		panic(err)
	}

	authorization, err := auth.NewAuthorization(o.Registry, o.Positional.Repository)
	if err != nil {
		panic(err)
	}

	registryTags, err := registry.GetTags(
		o.Registry,
		o.Positional.Repository,
		authorization,
	)
	if err != nil {
		panic(err)
	}
	for name, digest := range registryTags {
		fmt.Printf("> %-20s %s\n", name, digest)
	}

	localTags, err := local.GetTags(o.Positional.Repository)
	if err != nil {
		panic(err)
	}
	for name, digest := range localTags {
		fmt.Printf("< %-20s %s\n", name, digest)
	}
}
