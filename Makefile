# Jughead Makefile.
#
# Default targets keep the host build simple (CGO off, static binary). Plugin
# targets are opt-in: they require CGO and a matching libc (glibc on Linux,
# libSystem on macOS) so the host and plugin share compatible runtimes.

GO ?= go
PLUGINS_DIR ?= ./plugins
PLUGIN_SUFFIX ?= .so

# Host binary (static, no CGO) — the default deployment path.
build:
	$(GO) build -ldflags="-s -w" -o jughead .

# Host binary built with CGO enabled so it can load plugins at runtime. Use
# this when JUGHEAD_PLUGINS_DIR will point at one or more .so files.
build-cgo:
	CGO_ENABLED=1 $(GO) build -ldflags="-s -w" -o jughead .

# Build a single plugin by name, e.g. `make plugin NAME=github`.
plugin:
	@test -n "$(NAME)" || { echo "usage: make plugin NAME=<plugin-dir>"; exit 1; }
	@mkdir -p $(PLUGINS_DIR)
	CGO_ENABLED=1 $(GO) build -buildmode=plugin -o $(PLUGINS_DIR)/$(NAME)$(PLUGIN_SUFFIX) ./plugins/$(NAME)

# Build every plugin subpackage under plugins/ into $(PLUGINS_DIR).
plugins:
	@mkdir -p $(PLUGINS_DIR)
	@for dir in plugins/*/; do \
		name=$$(basename "$$dir"); \
		echo "building plugin: $$name"; \
		CGO_ENABLED=1 $(GO) build -buildmode=plugin -o $(PLUGINS_DIR)/$$name$(PLUGIN_SUFFIX) ./plugins/$$name || exit 1; \
	done

test:
	$(GO) test ./...

clean-plugins:
	rm -rf $(PLUGINS_DIR)

.PHONY: build build-cgo plugin plugins test clean-plugins