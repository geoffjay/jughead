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

# Build a single plugin by qualified name, e.g. `make plugin NAME=providers/github`
# or `make plugin NAME=sites/docs`. The NAME is the path relative to plugins/.
plugin:
	@test -n "$(NAME)" || { echo "usage: make plugin NAME=<providers|sites>/<name>"; exit 1; }
	@mkdir -p $(PLUGINS_DIR)
	@name=$$(basename "$(NAME)"); \
	echo "building plugin: $(NAME) -> $$name$(PLUGIN_SUFFIX)"; \
	CGO_ENABLED=1 $(GO) build -buildmode=plugin -o $(PLUGINS_DIR)/$$name$(PLUGIN_SUFFIX) ./plugins/$(NAME)

# Build every plugin under plugins/providers/*/ and plugins/sites/*/ into
# $(PLUGINS_DIR). The .so files are placed flat in $(PLUGINS_DIR) so the host
# loader (which scans a single directory) discovers them all in one pass.
plugins:
	@mkdir -p $(PLUGINS_DIR)
	@for dir in plugins/providers/*/ plugins/sites/*/; do \
		[ -d "$$dir" ] || continue; \
		name=$$(basename "$$dir"); \
		kind=$$(basename $$(dirname "$$dir")); \
		echo "building plugin: $$kind/$$name -> $$name$(PLUGIN_SUFFIX)"; \
		CGO_ENABLED=1 $(GO) build -buildmode=plugin -o $(PLUGINS_DIR)/$$name$(PLUGIN_SUFFIX) ./$$dir || exit 1; \
	done

test:
	$(GO) test ./...

clean-plugins:
	rm -rf $(PLUGINS_DIR)

.PHONY: build build-cgo plugin plugins test clean-plugins