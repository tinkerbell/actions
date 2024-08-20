# syntax=docker/dockerfile:1

FROM golang:1.21-alpine AS slurp
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src
WORKDIR /src/slurp
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o slurp

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=slurp /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=slurp /src/slurp/slurp /usr/bin/slurp
ENTRYPOINT ["/usr/bin/slurp"]
