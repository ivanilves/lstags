package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/registry"
)

type options struct {
	Registry   string `short:"r" long:"registry" default:"registry.hub.docker.com" description:"Docker registry to use" env:"REGISTRY"`
	Username   string `short:"u" long:"username" default:"" description:"Docker registry username" env:"USERNAME"`
	Password   string `short:"p" long:"password" default:"" description:"Docker registry password" env:"PASSWORD"`
	Positional struct {
		Repository string `positional-arg-name:"REPOSITORY" description:"Docker repository to list tags from"`
	} `positional-args:"yes" required:"yes"`
}

func suicide(err error) {
	fmt.Printf("%s\n", err.Error())
	os.Exit(1)
}

func shortify(str string, length int) string {
	if len(str) <= length {
		return str
	}

	return str[0 : length-1]
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

	sort.Strings(tagNames)

	return tagNames
}

func getShortImageID(imageID string) string {
	fields := strings.Split(imageID, ":")

	id := fields[1]

	return id[0:11]
}

func formatImageIDs(localImageIDs map[string]string, tagNames []string) map[string]string {
	imageIDs := make(map[string]string)

	for _, tagName := range tagNames {
		imageID, defined := localImageIDs[tagName]
		if defined {
			imageIDs[tagName] = getShortImageID(imageID)
		} else {
			imageIDs[tagName] = "n/a"
		}
	}

	return imageIDs
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

	return "n/a"
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

func getRepoRegistryName(repository, registry string) string {
	if !strings.Contains(repository, "/") {
		return "library/" + repository
	}

	if strings.HasPrefix(repository, registry) {
		return strings.Replace(repository, registry+"/", "", 1)
	}

	return repository
}

func getRepoLocalName(repository, registry string) string {
	if registry == "registry.hub.docker.com" {
		return repository
	}

	if strings.HasPrefix(repository, registry) {
		return repository
	}

	return registry + "/" + repository
}

func main() {
	o := options{}

	_, err := flags.Parse(&o)
	if err != nil {
		os.Exit(1)
	}

	repoRegistryName := getRepoRegistryName(o.Positional.Repository, o.Registry)
	repoLocalName := getRepoLocalName(o.Positional.Repository, o.Registry)

	authorization, err := auth.NewAuthorization(o.Registry, repoRegistryName, o.Username, o.Password)
	if err != nil {
		suicide(err)
	}
	registryTags, err := registry.FetchTags(o.Registry, repoRegistryName, authorization)
	if err != nil {
		suicide(err)
	}
	localTags, localImageIDs, err := local.FetchTags(repoLocalName)
	if err != nil {
		suicide(err)
	}

	tagNames := concatTagNames(registryTags, localTags)
	imageIDs := formatImageIDs(localImageIDs, tagNames)
	const format = "%-12s %-45s %-15s %s\n"
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<ID>", "<IMAGE>")
	for _, tagName := range tagNames {
		digest := shortify(getDigest(tagName, registryTags, localTags), 40)
		state := getState(tagName, registryTags, localTags)

		fmt.Printf(format, state, digest, imageIDs[tagName], repoLocalName+":"+tagName)
	}
}
