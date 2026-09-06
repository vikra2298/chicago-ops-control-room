.PHONY: test run build

test:
	cd backend && go test ./...

build:
	cd frontend && npm install && npm run build

run: build
	cd backend && go run ./cmd/server
