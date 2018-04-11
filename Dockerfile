FROM scratch
# * SSL root certificates bundle
COPY ca-certificates.crt /etc/ssl/certs/
# * compiled lstags binary [we assume you've run `make build GOOS=linux` before]
COPY ./dist/assets/lstags-linux/lstags /lstags
ENTRYPOINT ["/lstags"]
CMD ["--help"]
