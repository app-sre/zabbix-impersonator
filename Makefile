NAME := zabbix-impersonator

.PHONY: build
build:
	go build -o $(NAME) .

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify
