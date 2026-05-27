FROM golang:1.26-alpine AS oci2disk
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/oci2disk
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o oci2disk

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=oci2disk /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=oci2disk /src/oci2disk/oci2disk /usr/bin/oci2disk
ENTRYPOINT ["/usr/bin/oci2disk"]
