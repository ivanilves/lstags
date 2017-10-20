[![Build Status](https://travis-ci.org/ivanilves/lstags.svg?branch=master)](https://travis-ci.org/ivanilves/lstags)

# L/S tags

* *Compare local Docker images with ones present in registry.*
* *Sync your local Docker images with ones from the registry.*
* *Get insights on changes in watched Docker registries, easily.*
* *Facilitate maintenance of your own local "proxy" registries.*

**NB!** [Issues](https://github.com/ivanilves/lstags/issues) are welcome, [pull requests](https://github.com/ivanilves/lstags/pulls) are even more welcome! :smile:

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
**NB!** You can specify many images to list or pull: `lstags nginx~/^1\\.13/ mesosphere/chronos alpine~/^3\\./`

## Why would someone use this?
You could use `lstags`, if you ...
* ... continuously pull Docker images from some public or private registry to speed-up Docker run.
* ... poll registry for the new images pushed (to take some action afterwards, run CI for example).
* ... compare local images with registry ones (e.g. know, if image tagged "latest" was re-pushed).

... pull Ubuntu 14.04 & 16.04, all the Alpine images and Debian "stretch" to have latest software to play with:
```
lstags --pull ubuntu~/^1[46]\\.04$/ alpine debian~/stretch/
```
... pull and re-push CoreOS-related images from `quay.io` to your own registry (in case these hipsters will break everything):
```
lstags -P /quay -r registry.company.io quay.io/coreos/hyperkube quay.io/coreos/flannel
```
**NB!** In case you use private registry with authentication, make sure your Docker client knows how to authenticate against it!
`lstags` will reuse credentials saved by Docker client in its `config.json` file, one usually found at `~/.docker/config.json`

## Image state
`lstags` distinguishes four states of Docker image:
* `ABSENT` - present in registry, but absent locally
* `PRESENT` -  present in registry, present locally, with local and remote digests being equal
* `CHANGED` - present in registry, present locally, but with **different** local and remote digests
* `LOCAL-ONLY` - present locally, absent in registry

There is also special `UNKNOWN` state, which means `lstags` failed to detect image state for some reason.

## Authentication
You can either:
* rely on `lstags` discovering credentials "automagically" :tophat:
* load credentials from any Docker JSON config file specified

## Install: Binaries
https://github.com/ivanilves/lstags/releases

## Install: From source
```
git clone git@github.com:ivanilves/lstags.git
cd lstags
dep ensure
go build
./lstags -h
```
**NB!** I assume you have current versions of Go & [dep](https://github.com/golang/dep) installed and also have set up [GOPATH](https://github.com/golang/go/wiki/GOPATH) correctly.
