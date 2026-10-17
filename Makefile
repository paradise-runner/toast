.PHONY: build test test-integration test-integration-update run app clean rg-payload

# Pinned ripgrep bundled into the standalone binary and the desktop app.
RIPGREP_VERSION ?= 14.1.1
RG_PAYLOAD := cmd/toast/payload/rg

build: rg-payload
	go build -tags embed_rg -o bin/toast ./cmd/toast

test:
	go test ./...

test-integration:
	TOAST_RUN_TERMINAL_INTEGRATION=1 go test -count=1 -v ./integration

test-integration-update:
	TOAST_RUN_TERMINAL_INTEGRATION=1 TOAST_UPDATE_GOLDENS=1 go test -count=1 -v ./integration

run: rg-payload
	go run -tags embed_rg ./cmd/toast .

# Fetch the pinned ripgrep into the embed directory (cached, so repeat runs are
# cheap). `make build`/`make run` embed it; plain `go build` does not.
rg-payload:
	TOAST_RIPGREP_VERSION=$(RIPGREP_VERSION) scripts/fetch-ripgrep.sh $(RG_PAYLOAD)

# Standalone desktop app: toast bundled with a libghostty terminal window.
# See docs/experimental/libghostty-bundle.md. Requires cmake, ninja, and git;
# Zig 0.15.2 is provisioned automatically.
app:
	scripts/build-libghostty-bundle.sh

clean:
	rm -rf bin/ $(RG_PAYLOAD)
