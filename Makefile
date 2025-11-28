#!make
PORT = 8080
SERVICE_NAME = gsn-cli
CONTAINER_NAME = $(SERVICE_NAME)
DOCKER_COMPOSE_TAG = $(SERVICE_NAME)_1
TICKET_PREFIX := $(shell git branch --show-current | cut -d '_' -f 1)

# App Commands
dev:
	go run ./cmd/main.go
