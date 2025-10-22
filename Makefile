.PHONY: clean test lint build.local build.linux build.osx build.docker.internal build.push.internal build.docker.ghcr build.push.ghcr build.multiarch

BINARY        ?= es-operator
VERSION       ?= $(shell git describe --tags --always --dirty)
IMAGE         ?= registry-write.opensource.zalan.do/pandora/$(BINARY)
E2E_IMAGE     ?= $(IMAGE)-e2e
TAG           ?= $(VERSION)
SOURCES       = $(shell find . -name '*.go')
CRD_TYPE_SOURCE = pkg/apis/zalando.org/v1/types.go
GENERATED_CRDS = docs/zalando.org_elasticsearchdatasets.yaml docs/zalando.org_elasticsearchmetricsets.yaml
GENERATED      = pkg/apis/zalando.org/v1/zz_generated.deepcopy.go
DOCKERFILE    ?= Dockerfile
GOPKGS        = $(shell go list ./... | grep -v /e2e)
BUILD_FLAGS   ?= -v
LDFLAGS       ?= -X main.version=$(VERSION) -w -s

default: build.local

clean:
	rm -rf build
	rm -rf vendor
	rm -rf $(GENERATED)

test: $(GENERATED)
	go test -v -coverprofile=profile.cov $(GOPKGS)

lint:
	golangci-lint run ./...

$(GENERATED): go.mod $(CRD_TYPE_SOURCE)
	./hack/update-codegen.sh

$(GENERATED_CRDS): $(GENERATED) $(CRD_SOURCES)
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:crdVersions=v1 paths=./pkg/apis/... output:crd:dir=docs
	go run hack/crd/trim.go < docs/zalando.org_elasticsearchdatasets.yaml > docs/zalando.org_elasticsearchdatasets_trimmed.yaml
	go run hack/crd/trim.go < docs/zalando.org_elasticsearchmetricsets.yaml > docs/zalando.org_elasticsearchmetricsets_trimmed.yaml
	mv docs/zalando.org_elasticsearchdatasets_trimmed.yaml docs/zalando.org_elasticsearchdatasets.yaml
	mv docs/zalando.org_elasticsearchmetricsets_trimmed.yaml docs/zalando.org_elasticsearchmetricsets.yaml

build.local: build/$(BINARY) $(GENERATED_CRDS)
build.linux: build/linux/$(BINARY)

build/linux/e2e: $(GENERATED) $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o build/linux/$(notdir $@) $(BUILD_FLAGS) ./cmd/$(notdir $@)

build/linux/%: $(GENERATED) $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/$(notdir $@) -ldflags "$(LDFLAGS)" ./cmd/$(notdir $@)

build/e2e: $(GENERATED) $(SOURCES)
	CGO_ENABLED=0 go test -c -o build/$(notdir $@) ./cmd/$(notdir $@)

build/%: $(GENERATED) $(SOURCES)
	CGO_ENABLED=0 go build -o build/$(notdir $@) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" ./cmd/$(notdir $@)

build.osx: build/osx/$(BINARY)

build/$(BINARY): $(GENERATED) $(SOURCES)
	CGO_ENABLED=0 go build -o build/$(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

build/linux/$(BINARY): $(GENERATED) $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/$(BINARY) -ldflags "$(LDFLAGS)" .

build/osx/$(BINARY): $(GENERATED) $(SOURCES)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/osx/$(BINARY) -ldflags "$(LDFLAGS)" .

# Internal build targets (for delivery.yaml)
build.docker.internal: build.multiarch
	docker buildx build --platform linux/amd64,linux/arm64 -t "$(IMAGE)-e2e:$(TAG)" -f Dockerfile.e2e .
	docker buildx build --platform linux/amd64,linux/arm64 -t "$(IMAGE):$(TAG)" -f Dockerfile.internal .

build.push.internal: build.multiarch
	docker buildx build --platform linux/amd64,linux/arm64 -t "$(IMAGE)-e2e:$(TAG)" -f Dockerfile.e2e --push .
	docker buildx build --platform linux/amd64,linux/arm64 -t "$(IMAGE):$(TAG)" -f Dockerfile.internal --push .

# Multiarch build targets (for binaries)
build.multiarch: $(GENERATED) $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/amd64/$(BINARY) -ldflags "$(LDFLAGS)" .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/arm64/$(BINARY) -ldflags "$(LDFLAGS)" .
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o build/linux/amd64/e2e $(BUILD_FLAGS) ./cmd/e2e
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -o build/linux/arm64/e2e $(BUILD_FLAGS) ./cmd/e2e

# GHCR build targets (public multiarch images)
build.docker.ghcr: build.multiarch
	docker buildx build --platform linux/amd64,linux/arm64 -t "$(IMAGE):$(TAG)" -f Dockerfile.ghcr .

# Local build target for e2e testing (single arch with --load)
build.docker.local: build.multiarch
	docker buildx build --platform linux/amd64 -t "$(IMAGE):$(TAG)" -f Dockerfile.ghcr --load .

build.push.ghcr: build.multiarch
	docker buildx build --platform linux/amd64,linux/arm64 -t "$(IMAGE):$(TAG)" -f Dockerfile.ghcr --push .
