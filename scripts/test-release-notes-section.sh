#!/usr/bin/env bash
# Tests for scripts/release-notes-section.sh, which decides what a release body
# says (and what the pre-push hook accepts). Run by `make test`.
set -uo pipefail

script="$(cd "$(dirname "$0")" && pwd)/release-notes-section.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

cat >"$tmp/RELEASE_NOTES.md" <<'EOF'
# Release Notes

## [1.2.0](https://example.com/compare/v1.1.0...v1.2.0) (2026-01-02)

Prose for one-two-zero.

### Features
- a feature

---

## v1.1.0 — 2026-01-01

Prose for one-one-zero.

## v1.0.0 — 2025-12-31

Prose for one-zero-zero.

---

*Keep this file updated.*
EOF

failures=0
pass() { printf 'ok   %s\n' "$1"; }
fail() { printf 'FAIL %s\n     %s\n' "$1" "$2"; failures=$((failures + 1)); }

# assert_section <name> <tag> <expected>
assert_section() {
    local name="$1" tag="$2" expected="$3" got
    got="$(cd "$tmp" && "$script" "$tag")"
    if [ "$got" = "$expected" ]; then
        pass "$name"
    else
        fail "$name" "got:
$got
want:
$expected"
    fi
}

# assert_exit <name> <expected-code> <tag>
assert_exit() {
    local name="$1" want="$2" tag="$3" got
    (cd "$tmp" && "$script" "$tag" >/dev/null 2>&1)
    got=$?
    if [ "$got" -eq "$want" ]; then
        pass "$name"
    else
        fail "$name" "exit $got, want $want"
    fi
}

assert_section "release-please heading stops at ---" v1.2.0 "## [1.2.0](https://example.com/compare/v1.1.0...v1.2.0) (2026-01-02)

Prose for one-two-zero.

### Features
- a feature"

assert_section "hand-written heading stops at the next heading" v1.1.0 "## v1.1.0 — 2026-01-01

Prose for one-one-zero."

assert_section "last section does not swallow the footer" v1.0.0 "## v1.0.0 — 2025-12-31

Prose for one-zero-zero."

assert_exit "a version prefix does not match a longer version" 1 v1.2
assert_exit "an unknown version fails" 1 v2.0.0

if (cd "$tmp" && "$script" v9.9.9 2>/dev/null | grep -q .); then
    fail "failure prints nothing to stdout" "stdout was not empty"
else
    pass "failure prints nothing to stdout"
fi

if out="$(cd "$tmp" && "$script" v9.9.9 2>&1 >/dev/null)"; then
    fail "failure is explained on stderr" "exit code was 0"
elif printf '%s' "$out" | grep -q "no section for 'v9.9.9'"; then
    pass "failure is explained on stderr"
else
    fail "failure is explained on stderr" "unexpected stderr: $out"
fi

if (cd "$tmp" && "$script" v1.0.0 "$tmp/missing.md" >/dev/null 2>&1); then
    fail "missing notes file fails" "exit code was 0"
else
    pass "missing notes file fails"
fi

if [ "$failures" -gt 0 ]; then
    printf '\n%d release-notes test(s) failed\n' "$failures"
    exit 1
fi
printf '\nall release-notes tests passed\n'
