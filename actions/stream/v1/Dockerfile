# syntax=docker/dockerfile:experimental


# Build stream
FROM golang:1.15-alpine as stream
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev 
COPY . /go/src/github.com/thebsdbox/stream/
WORKDIR /go/src/github.com/thebsdbox/stream
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o stream

# Build final image
FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=stream /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=stream /go/src/github.com/thebsdbox/stream/stream .
ENTRYPOINT ["/stream"]