.PHONY: build test run clean

build:
	go build -o bin/toast ./cmd/toast

test:
	go test ./...

run:
	go run ./cmd/toast .

clean:
	rm -rf bin/
