.PHONY: build test test-integration test-integration-update run app clean

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

# Standalone desktop app: toast bundled with a libghostty terminal window.
# See docs/experimental/libghostty-bundle.md. Requires cmake, ninja, and git;
# Zig 0.15.2 is provisioned automatically.
app:
	scripts/build-libghostty-bundle.sh

clean:
	rm -rf bin/
