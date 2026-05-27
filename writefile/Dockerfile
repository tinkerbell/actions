FROM golang:1.26-alpine AS writefile
RUN apk add --no-cache ca-certificates
COPY . /src
WORKDIR /src/writefile
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o writefile

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=writefile /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=writefile /src/writefile/writefile /usr/bin/writefile
ENTRYPOINT ["/usr/bin/writefile"]
