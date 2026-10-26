# Releasing

Releases are driven by [Release Please](https://github.com/googleapis/release-please):
it reads conventional commit messages on `main`, keeps a release PR open with the
next version and its notes, and — when that PR is merged — tags the commit,
publishes the GitHub release, and starts the build of the binaries and the
Homebrew formula.

```
push to main ──▶ release-please.yml ──▶ release PR (version, file, body)
                                        │
                          merge ◀───────┘
                            │
                            ├─▶ tag vX.Y.Z + GitHub release (body from the PR)
                            └─▶ release.yml: build zips, assets, homebrew-tap
```

## Writing commits

Release Please only sees [Conventional Commits](https://www.conventionalcommits.org/),
so **the title of the squashed commit that lands on `main` decides the release**.
Repository settings squash-merge with the PR title, so title your PRs:

| Prefix                                | Release                        |
| ------------------------------------- | ------------------------------ |
| `fix: …`                              | patch (`0.12.0` → `0.12.1`)    |
| `feat: …`                             | minor (`0.12.0` → `0.13.0`)    |
| any type with `!`, or `BREAKING CHANGE:` footer | minor while pre-1.0 (`bump-minor-pre-major`) |
| `docs:`, `chore:`, `refactor:`, `test:`, `ci:`, `build:` | no release on their own |

Prefixes outside that list (`readme: …`, `sidebar: …`) are invisible to Release
Please, so a PR with a non-conventional title silently never releases. If a
release needs a different version than the commits imply, put a `Release-As:
x.y.z` footer in the commit body, or change the version directly in the release
PR.

## Where the notes live

While the release PR is open, the same notes exist in two places:

| Place | What it is |
| --- | --- |
| `RELEASE_NOTES.md` on the release branch | the changelog, which lands on `main` when the PR merges |
| the release PR body, after its `---` line | the **GitHub release body** |

Release Please generates both from the same changelog entry, so they start
identical — but editing one does not update the other. `release.yml` compares the
two while it attaches assets and emits a `::warning` when they disagree, so a
divergence shows up in the release run.

Release Please also *requires* the body to keep its shape: a bare `---` line must
separate the header, and the changelog section below it must start with
`## x.y.z` or `## [x.y.z](…)` — with no leading `v` in the body. Without that the
PR is not recognised as a release PR at all (`✖ Pull request body did not match`
in the workflow log), nothing is tagged, and the next run opens a second release
PR listing every commit since the last tag.

Section headings in `RELEASE_NOTES.md` itself may use either Release Please's
generated `## [x.y.z](…)` or the hand-written `## vX.Y.Z — date`;
`scripts/release-notes-section.sh` and the pre-push hook accept both.

## Shaping the notes

To write real prose instead of the generated bullets — the safe procedure:

1. Check out the release PR branch
   (`release-please--branches--main--components--toast`), rewrite the `## [x.y.z]`
   section of `RELEASE_NOTES.md`, commit and push.
2. Mirror the same text into the PR body's changelog section. Keep the `---`
   lines around it and leave the `## [x.y.z](…compare…)` heading in place. The
   `:robot:` header and footer may stay or go; the `---` delimiters and the
   version heading may not.

Do **not** replace the body wholesale. That is what broke the `0.13.0` release:
the body lost its `---` markers, Release Please stopped recognising the release
PR, and merging it tagged nothing. Recovery is described below.

`BEGIN_COMMIT_OVERRIDE` / `END_COMMIT_OVERRIDE` in the PR body is a different
tool: it overrides the *squash commit message* that generated the notes (for a
commit whose message is not conventional), not the release body.

## Merging the release PR

Leave the release PR alone and it stays up to date as more work lands; merge it
when you want to cut a release. Merging is all that is needed: the workflow tags,
publishes, builds, and updates the tap.

The invariant after a successful release — worth checking by eye if anything
looked odd:

> `.release-please-manifest.json`, the newest `vX.Y.Z` tag, and the top version
> heading in `RELEASE_NOTES.md` all name the same version.

## Verifying a release

```bash
gh release view v0.13.0 --json tagName,assets          # four zips
git ls-remote --tags origin | grep v0.13.0             # tag exists
gh api repos/:owner/toast/git/ref/tags/v0.13.0         # tag → the release PR merge commit
gh api 'repos/paradise-runner/homebrew-tap/commits?per_page=1'   # "toast v0.13.0"
scripts/release-notes-section.sh v0.13.0               # what the body should say
```

The binaries bake in the tag, so the quickest end-to-end check is:

```bash
gh release download v0.13.0 -p toast-darwin-arm64.zip
unzip -o toast-darwin-arm64.zip && ./toast-darwin-arm64 --version   # → v0.13.0
```

## When it goes wrong

### The release PR merged, but nothing was tagged

Look for the reason first:

```bash
gh run list --workflow=release-please.yml --limit 1
gh run view <run-id> --log | grep -E "did not match|Found pull request|release_created"
```

`Pull request body did not match` means the body was reshaped (see above). The
merge commit is still a perfectly good release commit, so recover by hand:

```bash
# 1. Create the tag + release at the release PR's merge commit, with the notes
#    from RELEASE_NOTES.md (the extractor understands both heading styles).
scripts/release-notes-section.sh v0.13.0 > /tmp/notes.md
gh release create v0.13.0 --target <merge-sha> --title v0.13.0 --notes-file /tmp/notes.md

# 2. Build the binaries and push the tap (this is the normal release workflow).
gh workflow run release.yml -f tag_name=v0.13.0

# 3. Close the duplicate release PR Release Please opened, drop the stale
#    pending label from the merged one, and delete the release branch so the
#    next run starts clean.
gh pr close <duplicate-pr>
gh pr edit <merged-release-pr> --remove-label "autorelease: pending"
gh api -X DELETE repos/:owner/toast/git/refs/heads/release-please--branches--main--components--toast

# 4. Confirm the steady state: it must open no new release PR.
gh workflow run release-please.yml
gh pr list --search "chore: release"
```

Step 2 reuses the tag's `RELEASE_NOTES.md` section for the release body, so a
hand-recovered release publishes the same notes an automated one would. If the
tag has no section at all, the workflow falls back to GitHub's generated notes.

### A tag exists but has no assets

```bash
gh workflow run release.yml -f tag_name=v0.13.0
```

### A release PR is stuck, or re-lists old commits

- A leftover `autorelease: pending` label on an old PR blocks new ones: remove
  the label and re-run `gh workflow run release-please.yml`.
- A release PR that lists commits from already-released versions means the
  manifest and the tags disagree — restore the invariant above (create the
  missing tag, or correct the manifest) before merging anything.

### A locally created tag is blocked by the pre-push hook

`.githooks/check-release-notes` refuses to push a tag without a matching section
in `RELEASE_NOTES.md`. It accepts both heading styles, because Release Please
writes `## [0.13.0](compare-link)` while hand-written sections look like
`## v0.13.0 — 2026-10-26`:

```bash
scripts/release-notes-section.sh v0.13.0   # exits non-zero if the section is missing
```

Tagging through `gh release create` (the recovery path) does not run local hooks;
add the notes section either way.

## Files involved

| File                              | Purpose                                                                 |
| --------------------------------- | ----------------------------------------------------------------------- |
| `release-please-config.json`      | Release type (`go`), tag shape, changelog sections, `RELEASE_NOTES.md` as the changelog |
| `.release-please-manifest.json`   | Last released version; bumped inside the release PR                     |
| `.github/workflows/release-please.yml` | Runs Release Please on `main` (or via `gh workflow run`), then calls `release.yml` when a release is created |
| `.github/workflows/release.yml`   | Builds the platform zips + `Toast.app`, attaches them to the release, updates the Homebrew tap; also publishes `RELEASE_NOTES.md` notes for a hand-dispatched tag |
| `scripts/release-notes-section.sh` | Prints a tag's notes section; used by `release.yml` and by the pre-push hook |
| `.githooks/check-release-notes`   | pre-push hook: no local tag without a notes section                     |

Release PRs are titled `chore: release X.Y.Z` (`pull-request-title-pattern`).
That pattern intentionally omits `${scope}` and `${component}`, since this is a
single-component repo, so each Release Please run logs
`pullRequestTitlePattern miss the part of '${scope}'` — expected noise, not an
error.

The Go release strategy keeps no version file: the version is the git tag, and
`release.yml` bakes it into the binaries with
`-ldflags "-X main.version=<tag>"`.

## Tokens

`release-please.yml` uses `GITHUB_TOKEN` by default, which is enough to open the
release PR, tag, publish, and build. Adding a `RELEASE_PLEASE_TOKEN` secret (a
PAT with `repo` scope) is optional but recommended: events caused by
`GITHUB_TOKEN` do not start other workflows, so with the default token CI does
not run on the release PR. The workflow picks up the PAT automatically when the
secret exists.

`HOMEBREW_TAP_TOKEN` is still required for the tap push.
