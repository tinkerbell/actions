FROM golang:1.26-alpine AS archive2disk
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/archive2disk
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o archive2disk

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=archive2disk /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=archive2disk /src/archive2disk/archive2disk /usr/bin/archive2disk
ENTRYPOINT ["/usr/bin/archive2disk"]
