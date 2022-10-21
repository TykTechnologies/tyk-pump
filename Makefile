.PHONY: all test bench clean dev build install docker up down logs teste

export GOBIN := /usr/local/bin
export GOPRIVATE := github.com/TykTechnologies/*
export DOCKER_BUILDKIT := 1
export CGO_ENABLED := 1

GO_VERSION ?= 1.16

GOCMD=go
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install

NAME=tyk-pump
TAGS=none-so-far
CONF=pump.example.conf

TEST_REGEX=.
TEST_COUNT=1

BENCH_REGEX=.
BENCH_RUN=NONE

all:
	@grep -F -h "##" Makefile | grep -F -v grep | perl -p -e 's/:([^#]+)//' | sed -e 's/^# /\n/' | sed -e 's/## /\t\\033[0m/' | expand -t 15 | xargs -d"\n" -I{} echo -e '\033[0;32m{}'

test: ## Run go unit tests
	$(GOTEST) -run=$(TEST_REGEX) -count=$(TEST_COUNT) ./...

bench: ## Run go benchmarks
	$(GOTEST) -run=$(BENCH_RUN) -bench=$(BENCH_REGEX) ./...

clean: ## Clean the go build cache, remove tyk-pump binary
	$(GOCLEAN)
	rm -f $(NAME)

dev: build ## Run the tyk-pump without docker
	./$(NAME) --conf $(CONF)

build: ## Build tyk-dashboard
	$(GOBUILD) -tags "$(TAGS)" -o $(NAME) -v .

install: ## Install tyk-pump to $GOBIN
	$(GOINSTALL) -tags "$(TAGS)"

docker: ## Build tyk-pump development image
	@docker build --no-cache --rm --ssh default --build-arg GO_VERSION=$(GO_VERSION) -t internal/$(NAME) -f Dockerfile .

up: ## Bring up docker compose env
	@docker compose up -d

down: ## Shut down docker compose env
	@docker compose down

logs: ## Output and follow docker compose logs
	@docker compose logs -f