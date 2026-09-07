FROM golang:1.26-alpine AS cidataio
COPY . /src
WORKDIR /src/cidataio
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o cidataio

# cidataio shells out to sgdisk, partprobe, mkfs.vfat and mount, so it needs a
# real userland rather than scratch.
FROM alpine:3.22
RUN apk add --no-cache sgdisk dosfstools util-linux
COPY --from=cidataio /src/cidataio/cidataio /usr/bin/cidataio
ENTRYPOINT ["/usr/bin/cidataio"]
