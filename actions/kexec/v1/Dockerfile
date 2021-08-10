# syntax=docker/dockerfile:experimental

# Build kexec
FROM golang:1.15-alpine as kexec
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /go/src/github.com/tinkerbell/hub/actions/kexec/v1/kexec/
WORKDIR /go/src/github.com/tinkerbell/hub/actions/kexec/v1/kexec
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o kexec

# Build final image
FROM scratch
COPY --from=kexec /go/src/github.com/tinkerbell/hub/actions/kexec/v1/kexec/kexec .
ENTRYPOINT ["/kexec"]
