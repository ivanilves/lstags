[![Build Status](https://travis-ci.org/ivanilves/lstags.svg?branch=master)](https://travis-ci.org/ivanilves/lstags)
[![Go Report Card](https://goreportcard.com/badge/github.com/ivanilves/lstags)](https://goreportcard.com/report/github.com/ivanilves/lstags)
[![Coverage Status](https://coveralls.io/repos/github/ivanilves/lstags/badge.svg?branch=master)](https://coveralls.io/github/ivanilves/lstags?branch=master)

![](https://github.com/ivanilves/lstags/raw/master/heading.png)

# L/S tags

`lstags` is a utility and an **[API](#api)** to manipulate (analyze, synchronize and aggregate) images across different Docker registries.

### Example invocation
```
$ lstags alpine~/^3\\./
<STATE>      <DIGEST>                                   <(local) ID>    <Created At>          <TAG>
ABSENT       sha256:9363d03ef12c8c25a2def8551e609f146   n/a             2017-09-13T16:32:00   alpine:3.1
CHANGED      sha256:9866438860a1b28cd9f0c944e42d3f6cd   39be345c901f    2017-09-13T16:32:05   alpine:3.2
ABSENT       sha256:ae4d16d132e3c93dd09aec45e4c13e9d7   n/a             2017-09-13T16:32:10   alpine:3.3
CHANGED      sha256:0d82f2f4b464452aac758c77debfff138   f64255f97787    2017-09-13T16:32:15   alpine:3.4
PRESENT      sha256:129a7f8c0fae8c3251a8df9370577d9d6   074d602a59d7    2017-09-13T16:32:20   alpine:3.5
PRESENT      sha256:f006ecbb824d87947d0b51ab8488634bf   76da55c8019d    2017-09-13T16:32:26   alpine:3.6
```
**NB!** You can specify many images to operate on, e.g: `lstags nginx~/^1\\.13/ mesosphere/chronos alpine~/^3\\./`

## Why would someone use this?
You could use `lstags`, if you ...
* ... aggregate images from different external registries into your own registry for **speed and locality** reasons.
* ... compare images present locally with the registry ones (e.g.: know if image tagged "latest" was re-pushed).
* ... continuously pull Docker images from some public or private registry to speed-up Docker run on your system.

### How?
... pull Ubuntu 14.04 & 16.04, all the Alpine images and Debian "stretch" to have the latest software to play with:
```sh
lstags --pull ubuntu~/^1[46]\\.04$/ alpine debian~/stretch/
```
... pull and re-push CoreOS-related images from `quay.io` to your own registry (in case these hipsters will break everything):
```sh
lstags -P /quay -r registry.company.io quay.io/coreos/hyperkube quay.io/coreos/flannel
```
**NB!** In case you use private registry with authentication, make sure your Docker client knows how to authenticate against it!
`lstags` will reuse credentials saved by Docker client in its `config.json` file, one usually found at `~/.docker/config.json`

## Possible image states
`lstags` distinguishes five states of Docker image:
* `ABSENT` - present in registry, but absent locally
* `PRESENT` -  present in registry, present locally, with local and remote digests being equal
* `CHANGED` - present in registry, present locally, but with **different** local and remote digests
* `ASSUMED` - **maybe** present in registry, not discovered by search, its presence assumed by user
* `LOCAL-ONLY` - present locally, absent in registry

There is also special `UNKNOWN` state, which means `lstags` failed to detect image state for some reason.

## Authentication
You can either:
* rely on `lstags` discovering credentials "automagically" :tophat:
* load credentials from any Docker JSON config file specified

## Assume tags
Sometimes registry may contain tags not exposed to any kind of search though still existing.
`lstags` is unable to discover these tags, but if you need to pull or push them, you may "assume"
they exist and make `lstags` blindly try to pull these tags from the registry. To inject assumed
tags into the registry query you need to extend repository specification with a `=` followed by a
comma-separated list of tags you want to assume.

e.g. we assume tags `v1.6.1` and `v1.7.0` exist like this: `lstags quay.io/calico/cni=v1.6.1,v1.7.0`

## Repository specification
Full repository specification looks like this:
```
[REGISTRY[:PORT]/]REPOSITORY[~/FILTER_REGEXP/][=TAG1,TAG2,TAGn]
```
You may provide infinite number of repository specifications to `lstags`

## YAML
:bulb: You can load repositories from the YAML file just like you do it from the command line arguments:
```
lstags -f file.yaml
```
A valid YAML file looks like this (mandatory `lstags` root key is here to be able to use "shared" YAMLs):
```yaml
lstags:
  repositories:
    - busybox
    - nginx:stable
    - mesosphere/marathon-lb~/^v1/
    - quay.io/coreos/awscli=master,latest,edge
    - gcr.io/google-containers/hyperkube~/^v1\.(9|10)\./
```
**NB!** `lstags` can load repositories from YAML or from CLI args, but not from both at the same time!

## Install: Binaries
https://github.com/ivanilves/lstags/releases

## Install: Wrapper
```sh
git clone git@github.com:ivanilves/lstags.git
cd lstags
sudo make wrapper
lstags -h
```
A special wrapper script will be installed to manage `lstags` invocation and updates. :sunglasses:

## Install: From source
```sh
git clone git@github.com:ivanilves/lstags.git
cd lstags
dep ensure
go build
./lstags -h
```
**NB!** I assume you have current versions of Go & [dep](https://github.com/golang/dep) installed and also have set up [GOPATH](https://github.com/golang/go/wiki/GOPATH) correctly.

## Using it with Docker

```
docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock ivanilves/lstags
Usage:
  lstags [OPTIONS] REPO1 REPO2 REPOn...

Application Options:
  -j, --docker-json=          JSON file with credentials (default:
                              ~/.docker/config.json) [$DOCKER_JSON]
  -p, --pull                  Pull Docker images matched by filter (will use
                              local Docker deamon) [$PULL]
  -P, --push                  Push Docker images matched by filter to some
                              registry (See 'push-registry') [$PUSH]
  -r, --push-registry=        [Re]Push pulled images to a specified remote
                              registry [$PUSH_REGISTRY]

--- OUTPUT WAS CUT HERE TO SAVE SPACE ---
```

### Analyze an image

```
docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock ivanilves/lstags alpine~/^3\\./
ANALYZE alpine
FETCHED alpine
-
<STATE>   <DIGEST>                                  <(local) ID>    <Created At>            <TAG>
CHANGED   sha256:b40e202395eaec699f2d0c5e01e6d6cb8  76da55c8019d    2017-10-25T23:19:51Z    alpine:3.6
ABSENT    sha256:d95da16498d5d6fb4b907cbe013f95032  n/a             2017-10-25T23:20:18Z    alpine:3.1
ABSENT    sha256:cb275b62f789b211114f28b391fca3cc2  n/a             2017-10-25T23:20:32Z    alpine:3.2
ABSENT    sha256:27af7da847283a947c008592f2b2cd6d2  n/a             2017-10-25T23:20:45Z    alpine:3.3
CHANGED   sha256:246bbbaa81b28837b64cb9dfc574de958  1a19a71e5d38    2017-10-25T23:20:59Z    alpine:3.4
CHANGED   sha256:aa96c8dc3815c44d4aceaf1ee7903ce58  37c7be7a096b    2017-10-25T23:21:13Z    alpine:3.5
-
```

## Development

**You are very welcome to open pull requests to this repository!** :wink:

To maximize our collaboration efficiency we would humbly ask you to follow these recommendations:
* Please add reasonable description (what?/why?/etc) to your pull request :exclamation:
* Your code should pass CI (Travis) and a [pretty liberal] code review :mag:
* If code adds or changes some logic, it should be covered by a unit test :neckbeard:
* Please, please, please: Put meaningful messages on your commits :pray:

**NB!** Not a requirement, but a GIF included in PR description would make our world a happier place!

### 'NORELEASE' branches and commits
**We have automatic release system.** Every PR merge will create a new application release with a changelog generated from PR branch commits.
For the most cases it is OK. However, if you work with things that do not need to be released (e.g. non user-facing changes), you have following options:
* If you don't want to create release from your PR, make it from branch containing "NORELEASE" keyword in its name.
* If you want to prevent single commit from appearing in a changelog, please start commit message with "NORELEASE".

#### Automatic releases are "preproduction" ones. They pass manual promotion :up: when we believe they are stable.

:warning: We don't build RPMs/DEBs/etc, as we see no need for it. We ship `lstags` as a single binary or as a Docker container.

## API

You may use `lstags` either as a standalone CLI or as a Golang package inside your own application:

### PoC application with our `v1` API
```golang
package main

import (
  "fmt"

  "github.com/ivanilves/lstags/api/v1"
)

func main() {
  api, _ := v1.New(v1.Config{})

  collection, _ := api.CollectTags(
    "alpine",
    "nginx=latest,stable",
    "gcr.io/google_containers/pause-amd64:3.1",
    "quay.io/coreos/flannel~/v0.10/",
  )

  for _, repo := range collection.Repos() {
    for _, tag := range collection.Tags(repo.Ref()) {
      fmt.Printf(
        "- %-40s %-15s %s %s\n",
        repo.Name(),
        tag.Name(),
        tag.GetCreatedString(),
        tag.GetState(),
      )
    }
  }
}
```

**NB!** Far more complete API usage example could be found in **[main.go](https://github.com/ivanilves/lstags/blob/master/main.go)** :wink:
