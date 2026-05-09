export GO111MODULE := on
export CGO_ENABLED := 0
DOCKER_TAG := $(or ${GITHUB_TAG_NAME}, latest)

SHA := $(shell git rev-parse --short=8 HEAD)
GITVERSION := $(shell git describe --long --all)
# gnu date format iso-8601 is parsable with Go RFC3339
BUILDDATE := $(shell date --iso-8601=seconds)
VERSION := $(or ${VERSION},$(shell git describe --tags --exact-match 2> /dev/null || git symbolic-ref -q --short HEAD || git rev-parse --short HEAD))

.PHONY: all
all: test server

.PHONY: release
release: test server

.PHONY: clean
clean:
	rm -f bin/*

.PHONY: test
test:
	CGO_ENABLED=1 go test -cover -race -timeout 30s ./...

.PHONY: bench
bench:
	cd pkg/datastore/postgres && CGO_ENABLED=1 go test -bench=. -run=- -benchmem -count 5 && cd -

.PHONY: lint
lint:
	golangci-lint run

.PHONY: server
server:
	go build -tags netgo -ldflags "-X 'github.com/metal-stack/v.Version=$(VERSION)' \
								   -X 'github.com/metal-stack/v.Revision=$(GITVERSION)' \
								   -X 'github.com/metal-stack/v.GitSHA1=$(SHA)' \
								   -X 'github.com/metal-stack/v.BuildDate=$(BUILDDATE)'" \
						 -o bin/server github.com/metal-stack/tenant-apiserver/cmd/server
	strip bin/server

.PHONY: mini-lab-push
mini-lab-push:
	docker build -t metalstack/tenant-apiserver:latest .
	kind --name metal-control-plane load docker-image metalstack/tenant-apiserver:latest
	kubectl --kubeconfig=$(MINI_LAB_KUBECONFIG) patch deployments.apps -n metal-control-plane tenant-apiserver --patch='{"spec":{"template":{"spec":{"containers":[{"name": "tenant-apiserver","imagePullPolicy":"IfNotPresent","image":"metalstack/tenant-apiserver:latest"}]}}}}'
	kubectl --kubeconfig=$(MINI_LAB_KUBECONFIG) delete pod -n metal-control-plane -l app=tenant-apiserver
