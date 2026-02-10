# vibespace Container Images Build
#
# Usage:
#   make base          # Build base image
#   make claude-code   # Build Claude Code agent image
#   make codex         # Build Codex agent image
#   make all           # Build all images
#
# Variables:
#   REGISTRY - Container registry (default: ghcr.io/vibespacehq/vibespace)
#   VERSION  - Image version tag (default: latest)

REGISTRY ?= ghcr.io/vibespacehq/vibespace
VERSION ?= latest

.PHONY: all base claude-code codex push-all push-base push-claude-code push-codex clean

# Build all images
all: claude-code codex

# Build base image (dependency for agent images)
base:
	docker build -t $(REGISTRY)/base:$(VERSION) ./base

# Build Claude Code agent image
claude-code: base
	docker build \
		--build-arg REGISTRY=$(REGISTRY) \
		--build-arg VERSION=$(VERSION) \
		-t $(REGISTRY)/claude-code:$(VERSION) \
		./agents/claude-code

# Build Codex agent image
codex: base
	docker build \
		--build-arg REGISTRY=$(REGISTRY) \
		--build-arg VERSION=$(VERSION) \
		-t $(REGISTRY)/codex:$(VERSION) \
		./agents/codex

# Push all images
push-all: push-base push-claude-code push-codex

push-base:
	docker push $(REGISTRY)/base:$(VERSION)

push-claude-code:
	docker push $(REGISTRY)/claude-code:$(VERSION)

push-codex:
	docker push $(REGISTRY)/codex:$(VERSION)

# Clean up local images
clean:
	-docker rmi $(REGISTRY)/base:$(VERSION)
	-docker rmi $(REGISTRY)/claude-code:$(VERSION)
	-docker rmi $(REGISTRY)/codex:$(VERSION)
