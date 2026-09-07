FROM golang:1.26-alpine AS talosmeta
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/talosmeta
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o talosmeta

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=talosmeta /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=talosmeta /src/talosmeta/talosmeta /usr/bin/talosmeta
ENTRYPOINT ["/usr/bin/talosmeta"]
