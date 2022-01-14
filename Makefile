.PHONY: clean test lint build.local build.linux build.osx build.docker build.push

BINARY        ?= es-operator
VERSION       ?= $(shell git describe --tags --always --dirty)
IMAGE         ?= registry-write.opensource.zalan.do/poirot/$(BINARY)
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
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:crdVersions=v1beta1,preserveUnknownFields=false paths=./pkg/apis/... output:crd:dir=docs || /bin/true || true
	mv docs/zalando.org_elasticsearchdatasets.yaml docs/zalando.org_elasticsearchdatasets_v1beta1.yaml
	mv docs/zalando.org_elasticsearchmetricsets.yaml docs/zalando.org_elasticsearchmetricsets_v1beta1.yaml
	# workaround for CRD issue with k8s 1.18 & controller-gen 0.3
	# ref: https://github.com/kubernetes/kubernetes/issues/91395
	perl -i -p0e 's/\s*x-kubernetes-list-map-keys:.*?x-kubernetes-list-type: map//seg' docs/zalando.org_elasticsearchdatasets_v1beta1.yaml docs/zalando.org_elasticsearchmetricsets_v1beta1.yaml
	go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:crdVersions=v1 paths=./pkg/apis/... output:crd:dir=docs || /bin/true || true

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

build.docker: build.linux build/linux/e2e
	docker build --rm -t "$(E2E_IMAGE):$(TAG)" -f Dockerfile.e2e .
	docker build --rm -t "$(IMAGE):$(TAG)" -f $(DOCKERFILE) .

build.push: build.docker
	docker push "$(E2E_IMAGE):$(TAG)"
	docker push "$(IMAGE):$(TAG)"
