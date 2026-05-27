FROM golang:1.26-alpine AS image2disk
RUN apk add --no-cache ca-certificates
WORKDIR /src/image2disk
COPY go.mod go.sum /src/
RUN go mod download
COPY . /src
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o image2disk

FROM scratch
# Add Certificates into the image, for anything that does HTTPS calls
COPY --from=image2disk /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=image2disk /src/image2disk/image2disk /usr/bin/image2disk
ENTRYPOINT ["/usr/bin/image2disk"]
