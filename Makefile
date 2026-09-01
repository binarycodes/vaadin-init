# vaadin-init's own build.
#
# The distribution is one static binary per platform, which is the whole reason
# the tool is written in Go: the person running it is bootstrapping a project and
# has no toolchain set up yet, so anything they must install first is a step too
# many.

BINARY := vaadin-init
DIST := dist

# Stamped into the binary, so `vaadin-init --version` names a commit rather than a
# number somebody remembered to bump. A checkout with no tags still produces
# something usable.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Every platform the tool claims to support. Kept as one list so that adding a
# platform is one entry rather than a new rule.
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: all build test vet fmt check dist install clean

all: check build

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

# Formatting is checked rather than applied, because a build that rewrites the
# tree under you is a build that makes `git diff` lie.
fmt:
	@unformatted=$$(gofmt -l . ); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed:"; echo "$$unformatted"; exit 1; \
	fi

check: fmt vet test

# CGO_ENABLED=0 is what makes these static: with it on, a linux/amd64 build links
# against the glibc of the machine that built it and then refuses to start on an
# older one.
dist: check
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		suffix=''; [ "$$os" = windows ] && suffix='.exe'; \
		output="$(DIST)/$(BINARY)-$$os-$$arch$$suffix"; \
		echo "building $$output"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags '$(LDFLAGS)' -o "$$output" . || exit 1; \
	done
	@echo
	@ls -lh $(DIST)

install:
	go install -ldflags '$(LDFLAGS)' .

clean:
	rm -rf $(BINARY) $(DIST)
