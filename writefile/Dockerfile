# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS writefile
RUN apk add --no-cache git ca-certificates gcc musl-dev
COPY . /src
WORKDIR /src/writefile
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o writefile

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=writefile /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=writefile /src/writefile/writefile /usr/bin/writefile
ENTRYPOINT ["/usr/bin/writefile"]
