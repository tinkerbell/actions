# syntax=docker/dockerfile:1

FROM golang:1.21-alpine as grub2disk
RUN apk add --no-cache grub grub-bios git ca-certificates gcc musl-dev
COPY . /src
WORKDIR /src/grub2disk
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o grub2disk

FROM scratch
COPY --from=grub2disk /src/grub2disk/grub2disk /usr/bin/grub2disk
ENTRYPOINT ["/usr/bin/grub2disk"]
