package main

import (
	"fmt"
	"os"

	"github.com/ivanilves/lstags/app"
	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/docker"
	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/registry"
)

var doNotFail bool

func suicide(err error) {
	fmt.Printf("%s\n", err.Error())

	if !doNotFail {
		os.Exit(1)
	}
}

func getVersion() string {
	return VERSION
}

func getAuthorization(t auth.TokenResponse) string {
	return t.Method() + " " + t.Token()
}

func main() {
	o, err := app.ParseFlags()
	if err != nil {
		suicide(err)
	}

	if o.Version {
		fmt.Printf("VERSION: %s\n", getVersion())
		os.Exit(0)
	}

	dockerconfig.DefaultUsername = o.Username
	dockerconfig.DefaultPassword = o.Password

	dockerConfig, err := dockerconfig.Load(o.DockerJSON)
	if err != nil {
		suicide(err)
	}

	registry.TraceRequests = o.TraceRequests

	auth.WebSchema = o.GetWebSchema()
	registry.WebSchema = o.GetWebSchema()

	doNotFail = o.DoNotFail

	const format = "%-12s %-45s %-15s %-25s %s\n"
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<Created At>", "<TAG>")

	repoCount := len(o.Positional.Repositories)
	pullCount := 0
	pushCount := 0

	pullAuths := make(map[string]string)

	dc, err := dockerclient.New(dockerConfig)
	if err != nil {
		suicide(err)
	}

	type tagResult struct {
		Tags     []*tag.Tag
		Repo     string
		Path     string
		Registry string
	}

	trc := make(chan tagResult, repoCount)

	for _, r := range o.Positional.Repositories {
		go func(r string, o *app.Options, trc chan tagResult) {
			repository, filter, err := app.SeparateFilterAndRepo(r)
			if err != nil {
				suicide(err)
			}

			registryName := docker.GetRegistry(repository)

			repoRegistryName := registry.FormatRepoName(repository, registryName)
			repoLocalName := local.FormatRepoName(repository, registryName)

			username, password, _ := dockerConfig.GetCredentials(registryName)

			pullAuths[repoLocalName], _ = dockerConfig.GetRegistryAuth(registryName)

			tresp, err := auth.NewToken(registryName, repoRegistryName, username, password)
			if err != nil {
				suicide(err)
			}

			authorization := getAuthorization(tresp)

			registryTags, err := registry.FetchTags(registryName, repoRegistryName, authorization, o.ConcurrentRequests)
			if err != nil {
				suicide(err)
			}

			imageSummaries, err := dc.ListImagesForRepo(repoLocalName)
			if err != nil {
				suicide(err)
			}
			localTags, err := local.FetchTags(repoLocalName, imageSummaries)
			if err != nil {
				suicide(err)
			}

			sortedKeys, names, joinedTags := tag.Join(registryTags, localTags)

			tags := make([]*tag.Tag, 0)
			for _, key := range sortedKeys {
				name := names[key]

				tg := joinedTags[name]

				if !app.DoesMatch(tg.GetName(), filter) {
					continue
				}

				if tg.NeedsPull() {
					pullCount++
				}
				pushCount++

				tags = append(tags, tg)
			}

			trc <- tagResult{Tags: tags, Repo: repoLocalName, Path: repoRegistryName, Registry: registryName}
		}(r, o, trc)
	}

	tagResults := make([]tagResult, repoCount)
	repoNumber := 0
	for tr := range trc {
		repoNumber++
		tagResults = append(tagResults, tr)
		if repoNumber >= repoCount {
			close(trc)
		}
	}

	for _, tr := range tagResults {
		for _, tg := range tr.Tags {
			fmt.Printf(
				format,
				tg.GetState(),
				tg.GetShortDigest(),
				tg.GetImageID(),
				tg.GetCreatedString(),
				tr.Repo+":"+tg.GetName(),
			)
		}
	}

	if o.Pull {
		done := make(chan bool, pullCount)

		for _, tr := range tagResults {
			go func(tags []*tag.Tag, repo string, done chan bool) {
				for _, tg := range tags {
					if tg.NeedsPull() {
						ref := repo + ":" + tg.GetName()

						fmt.Printf("PULLING %s\n", ref)
						err := dc.Pull(ref)
						if err != nil {
							suicide(err)
						}

						done <- true
					}

				}
			}(tr.Tags, tr.Repo, done)
		}

		pullNumber := 0
		if pullCount > 0 {
			for range done {
				pullNumber++

				if pullNumber >= pullCount {
					close(done)
				}
			}
		}
	}

	if o.Pull && o.PushRegistry != "" {
		done := make(chan bool, pullCount)

		for _, tr := range tagResults {
			go func(tags []*tag.Tag, repo, path, registry string, done chan bool) {
				for _, tg := range tags {
					prefix := o.PushPrefix
					if prefix == "" {
						prefix = app.GeneratePathFromHostname(registry)
					}

					srcRef := repo + ":" + tg.GetName()
					dstRef := o.PushRegistry + prefix + "/" + path + ":" + tg.GetName()

					fmt.Printf("PUSHING %s => %s\n", srcRef, dstRef)

					err := dc.Tag(srcRef, dstRef)
					if err != nil {
						suicide(err)
					}

					err = dc.Push(dstRef)
					if err != nil {
						suicide(err)
					}

					done <- true
				}
			}(tr.Tags, tr.Repo, tr.Path, tr.Registry, done)
		}

		pushNumber := 0
		if pushCount > 0 {
			for range done {
				pushNumber++

				if pushNumber >= pushCount {
					close(done)
				}
			}
		}
	}

}
