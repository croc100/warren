run:
	go run ./cmd/node

build:
	go build -o bin/node ./cmd/node

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run
