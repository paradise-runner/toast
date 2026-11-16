#!/usr/bin/env bash
# Tests for the Homebrew formula that release.yml renders into the tap.
#
# The formula is built by a heredoc inside .github/workflows/release.yml, so
# nothing type-checks it: a missing OS branch only shows up when a user runs
# `brew install` (issue #105 — no on_linux block meant Linuxbrew died with
# "formula requires at least a URL" before downloading anything). This renders
# the heredoc exactly as the workflow does and asserts every platform block has
# what Homebrew needs to load it.
#
# Run by `make test`.
set -uo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
workflow="$root/.github/workflows/release.yml"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

failures=0
pass() { printf 'ok   %s\n' "$1"; }
fail() { printf 'FAIL %s\n     %s\n' "$1" "$2"; failures=$((failures + 1)); }

# The heredoc body is indented to sit inside YAML's `run: |` block scalar, so
# the first line's indentation (10 spaces) has to be stripped from every line.
render() {
    awk '
        /cat > homebrew-tap\/toast\.rb <<EOF/ { inside = 1; next }
        inside && /^ *EOF$/ { exit }
        inside { sub(/^ {10}/, ""); print }
    ' "$workflow"
}

# render_as <version> <arm64-sha> <linux-sha>
render_as() {
    VERSION="$1" ARM64_SHA="$2" LINUX_AMD64_SHA="$3" render |
        sed -e "s/\${VERSION#v}/${1#v}/g" \
            -e "s/\${VERSION}/${1}/g" \
            -e "s/\${ARM64_SHA}/${2}/g" \
            -e "s/\${LINUX_AMD64_SHA}/${3}/g"
}

# render_in_shell <version> <arm64-sha> <linux-sha>
#
# The workflow runs the heredoc with bash, so the rendered file is what bash
# makes of it, not what the template says: the delimiter is unquoted, so
# backticks and $( ) in the formula body are evaluated. Running the extracted
# block for real is the only way to see that (a pair of backticks around
# "brew install" silently ate a word in the first version of the #105 fix).
#
# The render is deliberately starved of commands -- only cat is on PATH -- so a
# body that does try to execute something fails to find it instead of running
# it here.
render_in_shell() {
    mkdir -p "$tmp/shell/homebrew-tap" "$tmp/bin"
    ln -sf "$(command -v cat)" "$tmp/bin/cat"
    printf '%s\n' "$(render_block)" |
        (cd "$tmp/shell" && PATH="$tmp/bin" VERSION="$1" ARM64_SHA="$2" LINUX_AMD64_SHA="$3" \
            "$BASH" 2>/dev/null) || return 1
    cat "$tmp/shell/homebrew-tap/toast.rb"
}

# render_block prints the workflow's shell step verbatim: the `cat > ... <<EOF`
# line and the body, indentation stripped as YAML would strip it.
render_block() {
    awk '
        /cat > homebrew-tap\/toast\.rb <<EOF/ { inside = 1 }
        inside { sub(/^ {10}/, ""); print }
        inside && /^EOF$/ { exit }
    ' "$workflow"
}

sha_a="$(printf 'a%.0s' $(seq 64))"
sha_b="$(printf 'b%.0s' $(seq 64))"
formula="$(render_as v9.9.9 "$sha_a" "$sha_b")"

# platform_problem <os> <text> — echoes the reason a block is unusable, or
# nothing when it is fine. Homebrew needs url + sha256 in the *active* platform
# block; install is what actually puts the binary on $PATH.
platform_problem() {
    local os="$1" text="$2" body
    body="$(printf '%s\n' "$text" | awk -v os="$os" '
        $0 == "  on_" os " do" { inside = 1; next }
        inside && $0 == "  end" { exit }
        inside { print }
    ')"
    if [ -z "$body" ]; then
        printf 'no on_%s block' "$os"
        return
    fi
    grep -q 'url "https://' <<<"$body" || printf 'on_%s has no url' "$os"
    grep -qE 'sha256 "[0-9a-f]{64}"' <<<"$body" || printf 'on_%s has no sha256' "$os"
    grep -q 'def install' <<<"$body" || printf 'on_%s has no install' "$os"
}

printf '%s\n' "$formula" >"$tmp/toast.rb"

if [ -z "$(printf '%s\n' "$formula" | tr -d '[:space:]')" ]; then
    fail "render the formula from release.yml" "heredoc not found in $workflow"
else
    pass "render the formula from release.yml"
fi

if grep -q '\${' <<<"$formula"; then
    fail "every placeholder is substituted" "left in place: $(grep -o '\${[A-Za-z_]*}' <<<"$formula" | sort -u | tr '\n' ' ')"
else
    pass "every placeholder is substituted"
fi

# bash must render the same bytes the template asks for. A difference means
# something in the body was expanded (backticks, $( ), unescaped $VAR).
if ! shell_formula="$(render_in_shell v9.9.9 "$sha_a" "$sha_b")"; then
    fail "bash renders the template unchanged" "the heredoc step failed"
else
    printf '%s\n' "$shell_formula" >"$tmp/from-shell.rb"
    printf '%s\n' "$formula" >"$tmp/from-template.rb"
    if diff -u "$tmp/from-template.rb" "$tmp/from-shell.rb" >"$tmp/diff.txt"; then
        pass "bash renders the template unchanged"
    else
        fail "bash renders the template unchanged" "$(sed -n '3,12p' "$tmp/diff.txt")"
    fi
fi

if command -v ruby >/dev/null 2>&1; then
    if ruby -c "$tmp/toast.rb" >/dev/null 2>&1; then
        pass "ruby parses the rendered formula"
    else
        fail "ruby parses the rendered formula" "$(ruby -c "$tmp/toast.rb" 2>&1)"
    fi
else
    printf 'skip ruby parses the rendered formula (no ruby)\n'
fi

for os in macos linux; do
    problem="$(platform_problem "$os" "$formula")"
    if [ -z "$problem" ]; then
        pass "on_$os is loadable"
    else
        fail "on_$os is loadable" "$problem"
    fi
done

# A version with dots must not leak into the sha256 substitutions, and the
# version in the class body is the bare one (no leading v).
if grep -q '^  version "9\.9\.9"$' <<<"$formula"; then
    pass "class version has no leading v"
else
    fail "class version has no leading v" "$(grep -m1 '^  version' <<<"$formula")"
fi

# Regression proof: removing the on_linux block (the bug in #105) must be
# reported, otherwise this test would pass on the broken workflow too.
broken="$(printf '%s\n' "$formula" | awk '
    $0 == "  on_linux do" { skip = 1; next }
    skip && $0 == "  end" { skip = 0; next }
    !skip
')"
problem="$(platform_problem linux "$broken")"
if [ -z "$problem" ]; then
    fail "a formula without on_linux is rejected" "no problem reported"
elif [[ "$problem" == "no on_linux block" ]]; then
    pass "a formula without on_linux is rejected"
else
    fail "a formula without on_linux is rejected" "unexpected reason: $problem"
fi

if [ "$failures" -gt 0 ]; then
    printf '\n%d homebrew formula test(s) failed\n' "$failures"
    exit 1
fi
printf '\nall homebrew formula tests passed\n'
