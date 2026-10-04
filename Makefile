.PHONY: build run migrate test vet race lint fmt db-up
build:
	cd backend && go build -o bin/nebula ./cmd/server
run:
	cd backend && go run ./cmd/server serve
migrate:
	cd backend && go run ./cmd/server migrate
test:
	go test -count=1 ./backend/... ./frontend/...
vet:
	go vet ./backend/... ./frontend/...
race:
	go test -race -count=1 ./backend/... ./frontend/...
lint:
	golangci-lint run ./backend/... ./frontend/...
fmt:
	gofmt -w backend frontend/embed.go
db-up:
	docker compose up -d postgres
