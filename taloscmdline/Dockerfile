FROM golang:1.26-alpine AS taloscmdline
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/taloscmdline
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o taloscmdline

FROM scratch
COPY --from=taloscmdline /src/taloscmdline/taloscmdline /usr/bin/taloscmdline
ENTRYPOINT ["/usr/bin/taloscmdline"]
