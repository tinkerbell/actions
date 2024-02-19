# syntax=docker/dockerfile:experimental

# Build cexec
FROM golang:1.21-alpine as cexec
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src/cexec
WORKDIR /src/cexec
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o cexec

# Build final image
FROM scratch
COPY --from=cexec /src/cexec/cexec .
ENTRYPOINT ["/cexec"]
