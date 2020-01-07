NAME := zabbix-impersonator

ifndef $(GOPATH)
    GOPATH=$(shell go env GOPATH)
    export GOPATH
endif

.PHONY: \
	build \
	lint \
	test \
	vendor\
	vet

build:
	go build -o $(NAME) .

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
