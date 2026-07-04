# Plugin-capable build of jughead.
#
# Go's `plugin` package requires CGO and a matching libc between the host
# binary and every .so, so the builder (and the runtime base) use glibc-based
# Debian bookworm rather than alpine/scratch. The host binary and all plugin
# .so files are built in the same builder stage from the same toolchain and
# go.mod, then the .so files are dropped into /plugins in the runtime image so
# the loader discovers them via JUGHEAD_PLUGINS_DIR=/plugins.
#
# Plugin source lives under plugins/providers/*/ and plugins/sites/*/. The
# committed *_templ.go files are used (templ generate is not run in the image),
# so .dockerignore excludes *.templ without breaking the build.

FROM golang:1.26-bookworm AS builder

WORKDIR /build

# Go module cache.
COPY go.mod go.sum ./
RUN go mod download

# Source (minus .dockerignore). Includes plugins/ source.
COPY . .

# Build every plugin .so (providers + sites) flat into /build/plugins, then
# build the CGO-enabled host binary that will load them at runtime. The loop
# mirrors the `task plugins` Taskfile target.
RUN mkdir -p /build/plugins && \
    for dir in plugins/providers/*/ plugins/sites/*/; do \
        [ -d "$$dir" ] || continue; \
        name=$$(basename "$$dir"); \
        echo "building plugin: $$name.so"; \
        CGO_ENABLED=1 go build -buildmode=plugin -o /build/plugins/$$name.so ./$$dir || exit 1; \
    done
RUN CGO_ENABLED=1 GIN_MODE=release go build -ldflags="-s -w" -o jughead

FROM debian:bookworm-slim

# CA certs so Go's TLS stack can verify external HTTPS endpoints.
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/jughead /jughead
COPY --from=builder /build/static /static
COPY --from=builder /build/plugins /plugins

ENV BACKEND_PORT=8080 \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt \
    JUGHEAD_PLUGINS_DIR=/plugins

ENTRYPOINT ["/jughead"]
