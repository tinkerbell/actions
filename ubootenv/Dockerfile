FROM golang:1.26-alpine AS ubootenv
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/ubootenv
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o ubootenv

FROM scratch
COPY --from=ubootenv /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=ubootenv /src/ubootenv/ubootenv /usr/bin/ubootenv
ENTRYPOINT ["/usr/bin/ubootenv"]
