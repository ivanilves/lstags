#
# NB! This file is a template and might need editing before it works on your project!
#
FROM golang:1.8 AS builder
ENV DOCKER_HOST tcp://172.17.0.1:2375
ENV LOCAL_REGISTRY 172.17.0.1
WORKDIR /go/src/github.com/ivanilves/lstags
COPY . ./
RUN ln -nfs /bin/bash /bin/sh
RUN make DOCKER_HOST=${DOCKER_HOST} LOCAL_REGISTRY=${LOCAL_REGISTRY}
# Prevent empty useless directory from appearing in final image
RUN mv /go/src/github.com/ivanilves/lstags/lstags /lstags
FROM scratch
# Since we started from scratch, we'll copy following files from the builder:
# * SSL root certificates bundle
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# * compiled lstags binary
COPY --from=builder /lstags /lstags
ENTRYPOINT [ "/lstags" ]
CMD ["--help"]
