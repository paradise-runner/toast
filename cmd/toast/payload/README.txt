This directory holds the ripgrep binary embedded by `make build`.

`make build` runs scripts/fetch-ripgrep.sh to download a pinned ripgrep into
payload/rg; the file is embedded under the embed_rg build tag and extracted to
the user cache directory at startup, so project-wide search works without rg on
PATH.

The file is not committed. Without it, plain `go build ./cmd/toast` (no
embed_rg tag) and `make test` still work, and search falls back to PATH.
