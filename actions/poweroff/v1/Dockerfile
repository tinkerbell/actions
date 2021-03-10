# syntax=docker/dockerfile:experimental

# Build poweroff
FROM golang:1.15-alpine as poweroff
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /go/src/github.com/markyjackson-taulia/hub/actions/poweroff
WORKDIR /go/src/github.com/markyjackson-taulia/hub/actions/poweroff
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build

# Build final image
FROM scratch
COPY --from=poweroff /go/src/github.com/markyjackson-taulia/hub/actions/poweroff/poweroff.go .
ENTRYPOINT ["/poweroff"]
