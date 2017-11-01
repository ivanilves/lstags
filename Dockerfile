# This file is a template, and might need editing before it works on your project.
FROM golang:1.8-alpine AS builder
# We'll likely need to add SSL root certificates
RUN apk --no-cache add ca-certificates
# Missing part for compiling the application by it self because it requires a sock
FROM scratch
# Since we started from scratch, we'll copy the SSL root certificates from the builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY lstags /usr/local/bin/lstags
ENTRYPOINT [ "/usr/local/bin/lstags" ]
CMD ["--help"]
