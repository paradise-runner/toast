//go:build !embed_rg

package main

// bundledRipgrep returns "" when the binary is built without an embedded
// ripgrep, so search resolves rg from PATH.
func bundledRipgrep() string { return "" }
