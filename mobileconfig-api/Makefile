IMAGE ?= ghcr.io/caraar12345/mobileconfig-api

CREATED  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
REVISION := $(shell git rev-parse HEAD)
VERSION  := $(shell (git describe --tags --match 'mobileconfig-api/*' --abbrev=0 2>/dev/null | sed 's|mobileconfig-api/||' | grep .) || git rev-parse --short HEAD)

.PHONY: build push

build:
	docker build \
		--push \
		--build-arg CREATED=$(CREATED) \
		--build-arg REVISION=$(REVISION) \
		--build-arg VERSION=$(VERSION) \
		--platform linux/amd64,linux/arm64 \
		-t $(IMAGE) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):$(REVISION) \
		.

push: build
	docker push $(IMAGE) $(IMAGE):$(VERSION) $(IMAGE):$(REVISION)
