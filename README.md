# lstags

*Compare local Docker images with ones present in registry.*

## Why would someone use this?
You could use `lstags`, if you ...
* ... continuously pull Docker images from some public or private registry to speed-up Docker run.
* ... poll registry for the new images pushed (to take some action afterwards, run CI for example).
* ... compare local images with registry ones (e.g. know, if image tagged "latest" was re-pushed).

## How do I use it myself?
I run `lstags` inside a Cron Job on my Kubernetes worker nodes to poll my own Docker registry for a new [stable] images.
**NB!** `lstags` itself doesn't pull images, I use `grep`, `aws` and `xargs` to filter its output and pass it then do `docker pull`:
```
lstags -r registry.ivanilves.local -u myuser -p mypass tools/sicario \
  | egrep "^(ABSENT|CHANGED).*:v1\.[0-9]+\.[0-9]+$" \
  | awk '{print $NF}' ] \
  | xargs -i docker pull {}
```
... and following cronjob runs on my CI server to ensure I always have latest Ubuntu 14.04 and 16.04 images to play with:
```
lstags ubuntu | egrep "^(ABSENT|CHANGED).*:1[46].04$" | awk '{print $NF}' | xargs -i docker pull {}
```
My CI server is connected over crappy Internet link and pulling images in advance makes `docker run` much faster.

## Image state
`lstags` distinguishes four states of Docker image:
* `ABSENT` - present in registry, but absent locally
* `PRESENT` -  present in registry, present locally, with local and remote digests being equal
* `CHANGED` - present in registry, present locally, but with **different** local and remote digests
* `LOCAL-ONLY` - present locally, absent in registry

There is also special `UNKNOWN` state, which means `lstags` failed to detect image state for some reason.

## Install: From source
```
git clone git@github.com:ivanilves/lstags.git
cd lstags
dep ensure
go build
./lstags -h
```
**NB!** I assume you have current versions of Go & [dep](https://github.com/golang/dep) installed and also have set up [GOPATH](https://github.com/golang/go/wiki/GOPATH) correctly.
