NAME := zabbix-impersonator
REPO := quay.io/app-sre/$(NAME)
TAG := $(shell git rev-parse --short HEAD)


ifneq (,$(wildcard $(CURDIR)/.docker))
	DOCKER_CONF := $(CURDIR)/.docker
else
	DOCKER_CONF := $(HOME)/.docker
endif

ifndef $(GOPATH)
    GOPATH=$(shell go env GOPATH)
    export GOPATH
endif

.PHONY: \
	all \
	build \
	clean \
	image \
	image-push \
	lint \
	test \
	vendor\
	vet

all: build

build:
	go build -o $(NAME) .

clean:
	git clean -Xfd .

vendor:/
	go mod tidy
	go mod vendor
	go mod verify

test:
	go test -race -cover -covermode=atomic -mod=vendor 

vet:
	go vet -v ./...

lint:
	GO111MODULE=off go get -u github.com/golangci/golangci-lint/cmd/golangci-lint 
	$(GOPATH)/bin/golangci-lint run ./.../

image: build
	docker build -t $(REPO):$(TAG) -f hack/Dockerfile .

image-push:
	docker tag $(REPO):$(TAG) $(REPO):latest
	docker --config=$(DOCKER_CONF) push $(REPO):$(TAG)
	docker --config=$(DOCKER_CONF) push $(REPO):latest
