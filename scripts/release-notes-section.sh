#!/usr/bin/env bash
# Print the RELEASE_NOTES.md section for a tag, formatted to use as a GitHub
# release body. Used by release.yml when a hand-dispatched tag has no release
# yet, so the manual path publishes the same notes as the automated one.
#
# Accepts both heading styles that appear in the file:
#
#   ## [0.13.0](https://github.com/…/compare/v0.12.2...v0.13.0) (2026-10-26)
#   ## v0.10.0 — 2026-10-16
#
# Usage: release-notes-section.sh <tag> [path/to/RELEASE_NOTES.md]
set -euo pipefail

tag="${1:?usage: release-notes-section.sh <tag> [RELEASE_NOTES.md]}"
notes="${2:-RELEASE_NOTES.md}"
version="${tag#v}"

if [ ! -f "$notes" ]; then
    echo "release-notes-section: $notes not found" >&2
    exit 1
fi

# Print from the tag's heading up to (but excluding) the next "## " heading or
# the "---" separator that ends a section. The heading test is done by prefix
# comparison rather than a regex so versions like 0.13.0 cannot match 0.13.01,
# and so no escaping survives the shell -> awk handoff.
section="$(
    awk -v version="$version" '
        function is_heading(line) {
            return substr(line, 1, 3) == "## "
        }
        function matches(line,   rest, tail) {
            rest = substr(line, 4)
            if (substr(rest, 1, 1) == "[") rest = substr(rest, 2)
            if (substr(rest, 1, 1) == "v") rest = substr(rest, 2)
            if (substr(rest, 1, length(version)) != version) return 0
            tail = substr(rest, length(version) + 1, 1)
            return tail == "" || tail == "]" || tail == " " || tail == "\t"
        }
        matches($0) { inside = 1 }
        inside && is_heading($0) && !matches($0) { exit }
        inside && $0 == "---" { exit }
        inside { print }
    ' "$notes"
)"

if [ -z "$section" ]; then
    echo "release-notes-section: no section for '$tag' in $notes" >&2
    exit 1
fi

printf '%s\n' "$section"
