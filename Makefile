NAME := zabbix-impersonator

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
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint 
	golangci-lint run ./.../
