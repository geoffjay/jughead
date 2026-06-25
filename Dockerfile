FROM golang:1.26-alpine AS builder

# Move to working directory (/build).
WORKDIR /build

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN go mod download

# Copy your code into the container.
COPY . .

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0 GIN_MODE=release
RUN go build -ldflags="-s -w" -o jughead

FROM scratch

# Copy the system CA certificates from the builder so Go's TLS stack can
# verify external HTTPS endpoints (e.g. github.com). Without this, scratch
# has no cert store and every TLS handshake fails with "x509: certificate
# signed by unknown authority".
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

# Copy project's binary and templates from /build to the scratch container.
COPY --from=builder /build/jughead /
COPY --from=builder /build/static /static

# Set runtime environment variables.
ENV BACKEND_PORT=8080

# Set entry point.
ENTRYPOINT ["/jughead"]
