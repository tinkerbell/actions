# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS ubootenv
RUN apk add --no-cache git ca-certificates gcc musl-dev
COPY . /src
WORKDIR /src/ubootenv
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o ubootenv

FROM scratch
COPY --from=ubootenv /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=ubootenv /src/ubootenv/ubootenv /usr/bin/ubootenv
ENTRYPOINT ["/usr/bin/ubootenv"]
