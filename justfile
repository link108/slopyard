set dotenv-load := true
set shell := ["zsh", "-cu"]

app := "slopyard"
image := "slopyard:local"
gocache := "/tmp/slopyard-go-build"

default:
    @just --list

alias run := docker-run
alias stop := docker-stop
alias logs := docker-logs
alias migrate := db-migrate
alias seed := db-seed

docker-build:
    docker build -t {{image}} .

build:
    GOCACHE={{gocache}} go build ./cmd/slopyard ./cmd/migrate ./cmd/seed ./cmd/dbsetup

lint:
    GOCACHE={{gocache}} go vet ./...

typecheck:
    GOCACHE={{gocache}} go test -run '^$' ./...

verify: typecheck lint test build

dev: db-setup
    GOCACHE={{gocache}} go run ./cmd/slopyard

docker-run: docker-build
    -docker rm -f {{app}}
    docker run -d --name {{app}} --env-file .env -e DATABASE_URL="${DOCKER_DATABASE_URL:-${DATABASE_URL/localhost/host.docker.internal}}" -e REDIS_URL="${DOCKER_REDIS_URL:-${REDIS_URL/localhost/host.docker.internal}}" -p 8080:8080 --add-host=host.docker.internal:host-gateway {{image}}

docker-stop:
    -docker stop {{app}}
    -docker rm {{app}}

docker-logs:
    docker logs -f {{app}}

db-migrate:
    GOCACHE={{gocache}} go run ./cmd/migrate up

db-setup:
    GOCACHE={{gocache}} go run ./cmd/dbsetup
    GOCACHE={{gocache}} go run ./cmd/migrate up

db-reset:
    GOCACHE={{gocache}} go run ./cmd/migrate down
    GOCACHE={{gocache}} go run ./cmd/migrate up

db-seed:
    GOCACHE={{gocache}} go run ./cmd/seed

test:
    GOCACHE={{gocache}} go test ./...
