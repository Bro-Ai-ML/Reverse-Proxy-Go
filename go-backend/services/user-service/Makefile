.PHONY: test test-cover lint

test:
	go test -v -short ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run
