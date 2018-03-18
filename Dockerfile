FROM scratch
# * SSL root certificates bundle
COPY ca-certificates.crt /etc/ssl/certs/
# * compiled lstags binary [we assume you've run `make build GOOS=linux` before]
COPY ./dist/assets/lstags-linux/lstags /lstags
# Make sure we [are statically linked and] can run inside a scratch-based container
RUN ["/lstags", "--version"]
ENTRYPOINT ["/lstags"]
CMD ["--help"]
