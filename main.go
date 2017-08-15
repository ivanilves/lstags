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

func concatTagNames(registryTags, localTags map[string]string) []string {
	tagNames := make([]string, 0)

	for tagName, _ := range registryTags {
		tagNames = append(tagNames, tagName)
	}

	for tagName, _ := range localTags {
		_, defined := registryTags[tagName]
		if !defined {
			tagNames = append(tagNames, tagName)
		}
	}

	return tagNames
}

func getDigest(tagName string, registryTags, localTags map[string]string) string {
	registryDigest, defined := registryTags[tagName]
	if defined && registryDigest != "" {
		return registryDigest
	}

	localDigest, defined := localTags[tagName]
	if defined && localDigest != "" {
		return localDigest
	}

	return "UNKNOWN"
}

func getState(tagName string, registryTags, localTags map[string]string) string {
	registryDigest, definedInRegistry := registryTags[tagName]
	localDigest, definedLocally := localTags[tagName]

	if definedInRegistry && !definedLocally {
		return "ABSENT"
	}

	if !definedInRegistry && definedLocally {
		return "LOCAL-ONLY"
	}

	if definedInRegistry && definedLocally {
		if registryDigest == localDigest {
			return "PRESENT"
		} else {
			return "CHANGED"
		}
	}

	return "UNKNOWN"
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
	localTags, err := local.GetTags(o.Positional.Repository)
	if err != nil {
		panic(err)
	}

	tagNames := concatTagNames(registryTags, localTags)
	for _, tagName := range tagNames {
		digest := getDigest(tagName, registryTags, localTags)
		state := getState(tagName, registryTags, localTags)

		fmt.Printf("%-12s %-80s %s\n", state, digest, o.Positional.Repository+":"+tagName)
	}
}
