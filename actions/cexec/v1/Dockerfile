# syntax=docker/dockerfile:experimental

# Build cexec
FROM golang:1.15-alpine as cexec
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /go/src/github.com/thebsdbox/cexec/
WORKDIR /go/src/github.com/thebsdbox/cexec
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o cexec

# Build final image
FROM scratch
COPY --from=cexec /go/src/github.com/thebsdbox/cexec/cexec .
ENTRYPOINT ["/cexec"]