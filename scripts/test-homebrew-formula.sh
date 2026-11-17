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

# render_as <version> <arm64-sha> <linux-amd64-sha> <linux-arm64-sha>
render_as() {
    VERSION="$1" ARM64_SHA="$2" LINUX_AMD64_SHA="$3" LINUX_ARM64_SHA="$4" render |
        sed -e "s/\${VERSION#v}/${1#v}/g" \
            -e "s/\${VERSION}/${1}/g" \
            -e "s/\${ARM64_SHA}/${2}/g" \
            -e "s/\${LINUX_AMD64_SHA}/${3}/g" \
            -e "s/\${LINUX_ARM64_SHA}/${4}/g"
}

# render_in_shell <version> <arm64-sha> <linux-amd64-sha> <linux-arm64-sha>
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
            LINUX_ARM64_SHA="$4" "$BASH" 2>/dev/null) || return 1
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
sha_c="$(printf 'c%.0s' $(seq 64))"
formula="$(render_as v9.9.9 "$sha_a" "$sha_b" "$sha_c")"

# block_body <os> <text> — prints the whole `on_<os> do … end` block, nested
# blocks included. The per-CPU `if Hardware::CPU.arm?` branch holds its own
# `def install … end` and its own `end` for the `if`, so a reader that stops at
# the first `end` it sees returns a truncated block and misses the branch.
block_body() {
    printf '%s\n' "$2" | awk -v os="$1" '
        !inside && $0 == "  on_" os " do" { inside = 1 }
        inside {
            line = $0
            sub(/#.*/, "", line)
            print
            if (line ~ /(^|[[:space:]])do[[:space:]]*$/) depth++
            if (line ~ /^[[:space:]]*(if|unless|case|while|until|begin|def)[[:space:]]/) depth++
            if (line ~ /^[[:space:]]*end[[:space:]]*$/) depth--
            if (depth <= 0) exit
        }
    '
}

# platform_problem <os> <text> — echoes the reason a block is unusable, or
# nothing when it is fine. Homebrew needs url + sha256 in the *active* platform
# block; install is what actually puts the binary on $PATH.
platform_problem() {
    local os="$1" body
    body="$(block_body "$1" "$2")"
    if [ -z "$body" ]; then
        printf 'no on_%s block' "$os"
        return
    fi
    grep -q 'url "https://' <<<"$body" || printf 'on_%s has no url' "$os"
    grep -qE 'sha256 "[0-9a-f]{64}"' <<<"$body" || printf 'on_%s has no sha256' "$os"
    grep -q 'def install' <<<"$body" || printf 'on_%s has no install' "$os"
}

# arm_branch <on_linux block> — prints the `if Hardware::CPU.arm?` branch of
# the Linux block, stopping at the `else`.
arm_branch() {
    awk '
        /^[[:space:]]*if Hardware::CPU\.arm\?/ { inside = 1; next }
        inside && /^[[:space:]]*(else|end)[[:space:]]*$/ { exit }
        inside { print }
    ' <<<"$1"
}

# linux_arch_problem <text> — echoes why the Linux block cannot install on
# every CPU it loads on, or nothing when it can. Linuxbrew evaluates on_linux
# on both x86_64 and arm64, so a single url at the top of the block hands arm
# users the amd64 binary; the url has to be chosen from the architecture the
# way install picks the binary name (issue #111: the block existed but only
# knew about amd64, so arm64 hit an odie instead of an install).
linux_arch_problem() {
    local linux arm install
    linux="$(block_body linux "$1")"
    arm="$(arm_branch "$linux")"
    install="$(sed -n '/^[[:space:]]*def install/,$p' <<<"$linux")"
    if [ -z "$arm" ]; then
        printf 'on_linux has no Hardware::CPU.arm? branch'
        return
    fi
    grep -q 'toast-linux-arm64\.zip' <<<"$arm" || printf 'on_linux arm branch has no arm64 url'
    grep -qE 'sha256 "[0-9a-f]{64}"' <<<"$arm" || printf 'on_linux arm branch has no sha256'
    grep -q 'toast-linux-amd64\.zip' <<<"$linux" || printf 'on_linux has no amd64 url'
    grep -q 'toast-linux-arm64' <<<"$install" || printf 'on_linux install never copies the arm64 binary'
    grep -q 'toast-linux-amd64' <<<"$install" || printf 'on_linux install never copies the amd64 binary'
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
if ! shell_formula="$(render_in_shell v9.9.9 "$sha_a" "$sha_b" "$sha_c")"; then
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

problem="$(linux_arch_problem "$formula")"
if [ -z "$problem" ]; then
    pass "on_linux has a branch per CPU"
else
    fail "on_linux has a branch per CPU" "$problem"
fi

# The arm64 branch must carry the arm64 sha, not the amd64 one: a copy-pasted
# placeholder would otherwise sail through the checks above.
arm_pair="$(awk '/toast-linux-arm64\.zip/ { getline; print }' <<<"$formula")"
if grep -q "sha256 \"$sha_c\"" <<<"$arm_pair"; then
    pass "arm64 url carries the arm64 sha256"
else
    fail "arm64 url carries the arm64 sha256" "${arm_pair:-no sha256 line after the arm64 url}"
fi

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

# Regression proof: an on_linux block that only knows the amd64 asset (the bug
# in #111) must be reported, otherwise the new check would pass on a formula
# that hands arm64 users an x86_64 binary.
broken="$(printf '%s\n' "$formula" | awk '
    /^[[:space:]]*if Hardware::CPU\.arm\?[[:space:]]*$/ { skip = 1; next }
    skip && /^[[:space:]]*else[[:space:]]*$/ { skip = 0; next }
    !skip
')"
problem="$(linux_arch_problem "$broken")"
if [ -z "$problem" ]; then
    fail "a formula without a Linux arm64 branch is rejected" "no problem reported"
elif [[ "$problem" == "on_linux has no Hardware::CPU.arm? branch" ]]; then
    pass "a formula without a Linux arm64 branch is rejected"
else
    fail "a formula without a Linux arm64 branch is rejected" "unexpected reason: $problem"
fi

if [ "$failures" -gt 0 ]; then
    printf '\n%d homebrew formula test(s) failed\n' "$failures"
    exit 1
fi
printf '\nall homebrew formula tests passed\n'
