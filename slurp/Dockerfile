FROM golang:1.26-alpine AS slurp
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/slurp
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o slurp

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=slurp /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=slurp /src/slurp/slurp /usr/bin/slurp
ENTRYPOINT ["/usr/bin/slurp"]
