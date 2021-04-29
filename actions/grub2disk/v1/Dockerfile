# syntax=docker/dockerfile:1
FROM golang:1.15-alpine as grub2disk
RUN apk add --no-cache grub grub-bios git ca-certificates gcc musl-dev
COPY . /go/src/github.com/tinkerbell/hub/actions/grub2disk/v1
WORKDIR /go/src/github.com/tinkerbell/hub/actions/grub2disk/v1
ENV GO111MODULE=auto
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o grub2disk

# Build final image
FROM scratch
COPY --from=grub2disk /go/src/github.com/tinkerbell/hub/actions/grub2disk/v1/grub2disk .
ENTRYPOINT ["/grub2disk"]
