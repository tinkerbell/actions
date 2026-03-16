FROM golang:1.24-alpine AS cidataio
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src
WORKDIR /src/cidataio
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o cidataio

FROM alpine:latest
RUN apk add --no-cache sgdisk dosfstools util-linux
COPY --from=cidataio /src/cidataio/cidataio /usr/bin/cidataio
ENTRYPOINT ["/usr/bin/cidataio"]
