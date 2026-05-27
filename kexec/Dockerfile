FROM golang:1.26-alpine AS kexec
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/kexec
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o kexec

FROM scratch
COPY --from=kexec /src/kexec/kexec /usr/bin/kexec
ENTRYPOINT ["/usr/bin/kexec"]
