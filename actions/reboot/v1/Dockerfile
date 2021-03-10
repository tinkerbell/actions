# syntax=docker/dockerfile:experimental

# Build reboot
FROM golang:1.15-alpine as reboot
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /go/src/github.com/markyjackson-taulia/hub/actions/reboot
WORKDIR /go/src/github.com/markyjackson-taulia/hub/actions/reboot
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build

# Build final image
FROM scratch
COPY --from=reboot /go/src/github.com/markyjackson-taulia/hub/actions/reboot/reboot.go .
ENTRYPOINT ["/reboot"]
