//go:build embed_rg

package main

import _ "embed"

// bundledRG is the ripgrep binary fetched by `make build` (see
// scripts/fetch-ripgrep.sh).
//
//go:embed payload/rg
var bundledRG []byte

// bundledRipgrep extracts the embedded ripgrep and returns its path, or "" when
// it cannot be written (callers then fall back to PATH).
func bundledRipgrep() string {
	return writeEmbeddedBinary(bundledRG, "rg")
}
