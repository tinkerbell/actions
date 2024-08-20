# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS kexec
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src
WORKDIR /src/kexec
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o kexec

FROM scratch
COPY --from=kexec /src/kexec/kexec /usr/bin/kexec
ENTRYPOINT ["/usr/bin/kexec"]
