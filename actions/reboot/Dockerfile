# syntax=docker/dockerfile:experimental

# Build reboot
FROM golang:1.15-alpine as reboot
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /go/src/github.com/markyjackson-taulia/hub/reboot
WORKDIR /go/src/github.com/markyjackson-taulia/hub/reboot
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o reboot

# Build final image
FROM scratch
COPY --from=kexec /go/src/github.com/markyjackson-taulia/hub/reboot .
ENTRYPOINT ["/reboot"]