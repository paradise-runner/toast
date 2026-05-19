.PHONY: build test test-integration test-integration-update run clean

build:
	go build -o bin/toast ./cmd/toast

test:
	go test ./...

test-integration:
	TOAST_RUN_TERMINAL_INTEGRATION=1 go test -count=1 -v ./integration

test-integration-update:
	TOAST_RUN_TERMINAL_INTEGRATION=1 TOAST_UPDATE_GOLDENS=1 go test -count=1 -v ./integration

run:
	go run ./cmd/toast .

clean:
	rm -rf bin/
