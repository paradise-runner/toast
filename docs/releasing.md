# Releasing

Releases are driven by [Release Please](https://github.com/googleapis/release-please):
it reads conventional commit messages on `main`, keeps a release PR open with
the next version and its notes, and — when that PR is merged — tags the commit,
publishes the GitHub release, and starts the build of the binaries and the
Homebrew formula.

```
push to main ──▶ release-please.yml ──▶ release PR (version + RELEASE_NOTES.md)
                                        │
                          merge ◀───────┘
                            │
                            ├─▶ tag vX.Y.Z + GitHub release (notes from RELEASE_NOTES.md)
                            └─▶ release.yml: build zips, attach assets, update homebrew-tap
```

## Prerequisites

One repository setting must be on before the workflow can open a release PR:

**Settings → Actions → General → Workflow permissions → Allow GitHub Actions to
create and approve pull requests** must be checked.

Release Please opens the release PR with the repository's `GITHUB_TOKEN`, and
GitHub blocks that identity from creating pull requests unless the setting (or
an organisation policy allowing it) is enabled. With it off, Release Please
still does all the work — creates the release branch and commit, bumps the
version, writes the notes — and then fails at the very last step:

```
release-please failed: GitHub Actions is not permitted to create or approve pull requests.
```

Enable it with:

```bash
gh api -X PUT repos/OWNER/REPO/actions/permissions/workflow \
  -F default_workflow_permissions=read \
  -F can_approve_pull_request_reviews=true
```

`default_workflow_permissions` can stay `read`; only the pull-request permission
is needed. Alternatively, add a `RELEASE_PLEASE_TOKEN` secret (see
[Tokens](#tokens)) so the action runs as your user instead of `GITHUB_TOKEN` —
useful when the repository cannot enable the setting.

## Writing commits

Release Please only sees [Conventional Commits](https://www.conventionalcommits.org/),
so **the title of the squashed commit that lands on `main` decides the release**.
Repository settings squash-merge with the PR title, so title your PRs:

| Prefix                                | Release                        |
| ------------------------------------- | ------------------------------ |
| `fix: …`                              | patch (`0.12.0` → `0.12.1`)    |
| `docs: …`, `perf: …`, `revert: …`     | patch — they have visible changelog sections |
| `feat: …`                             | minor (`0.12.0` → `0.13.0`)    |
| any type with `!`, or `BREAKING CHANGE:` footer | minor while pre-1.0 (`bump-minor-pre-major`) |
| `chore:`, `refactor:`, `test:`, `ci:`, `build:`, `style:` | no release on their own |

The rule is the `changelog-sections` list in `release-please-config.json`: a
commit whose type has a **visible** section produces a release — `feat` bumps
minor, every other visible type bumps patch — while types marked `hidden`
produce no release on their own. So `docs:`, `perf:` and `revert:` *do* cut a
release; use a hidden type such as `chore:` for a change that must not.

Prefixes outside that list (`readme: …`, `sidebar: …`) are invisible to Release
Please, so a PR with a non-conventional title silently never releases. If a
release needs a different version than the commits imply, put a `Release-As:
x.y.z` footer in the commit body, or change the version directly in the release
PR.

## Merging the release PR

Leave the release PR alone and it stays up to date as more work lands; merge it
when you want to cut a release.

To shape the notes beyond the generated entries, either:

- push prose commits straight onto the release PR branch
  (`release-please--branches--main--components--toast`) — any edit to the PR is
  carried through, or
- edit the PR body with a `BEGIN_COMMIT_OVERRIDE` / `END_COMMIT_OVERRIDE` block
  before merging, so the squashed commit message (and therefore the generated
  notes) is replaced by the block's contents.

Entries are written into `RELEASE_NOTES.md` — the same file the hand-written
sections live in. Release Please inserts its section above the previous one and
never rewrites what is already there, so old prose keeps its formatting.

## Files involved

| File                              | Purpose                                                                 |
| --------------------------------- | ----------------------------------------------------------------------- |
| `release-please-config.json`      | Release type (`go`), tag shape, changelog sections, `RELEASE_NOTES.md` as the changelog |
| `.release-please-manifest.json`   | Last released version; bumped inside the release PR                     |
| `.github/workflows/release-please.yml` | Runs Release Please on `main`, then calls `release.yml` when a release is created |
| `.github/workflows/release.yml`   | Builds the platform zips + `Toast.app`, attaches them to the release, updates the Homebrew tap |

Release PRs are titled `chore: release X.Y.Z` (`pull-request-title-pattern`).
That pattern intentionally omits `${scope}` and `${component}`, since this is a
single-component repo, so each Release Please run logs
`pullRequestTitlePattern miss the part of '${scope}'` — expected noise, not an
error.

The Go release strategy keeps no version file: the version is the git tag, and
`release.yml` bakes it into the binaries with
`-ldflags "-X main.version=<tag>"`.

## Tokens

`release-please.yml` uses `GITHUB_TOKEN` by default. With the permission in
[Prerequisites](#prerequisites) enabled, that is enough to open the release PR,
tag, publish, and build. Adding a `RELEASE_PLEASE_TOKEN` secret (a PAT with
`repo` scope) is optional but recommended: events caused by `GITHUB_TOKEN` do
not start other workflows, so with the default token CI does not run on the
release PR. The workflow picks up the PAT automatically when the secret exists.

`HOMEBREW_TAP_TOKEN` is still required for the tap push.

## Manual builds and recovery

- A tag with no artifacts yet: `gh workflow run release.yml -f tag_name=v0.13.0`
  (creates the release with GitHub's generated notes if it does not exist).
- The pre-push hook in `.githooks/check-release-notes` still blocks locally
  created tags that have no `## vX.Y.Z` section in `RELEASE_NOTES.md`.
- A stuck release PR usually means a leftover `autorelease: pending` label on an
  old PR; remove the label and re-run the workflow.
- To re-run Release Please without a new commit, re-run the last
  `release-please` workflow run from the Actions tab.
