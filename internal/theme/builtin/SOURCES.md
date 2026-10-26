# Builtin theme sources

`toast-dark.json` and `toast-light.json` are hand-written. Every other theme in
this directory is generated from a [Zed](https://zed.dev) theme extension with:

```bash
toast migrate-theme zed path/to/theme.json internal/theme/builtin
```

One upstream file usually contains a whole family, and every variant becomes
its own Toast theme. Upstream projects are all MIT licensed; their licence
texts are kept verbatim in [`licenses/`](licenses/), and each theme name below
corresponds to the upstream theme of the same name.

| Vendor | Upstream | Commit | Source file | Themes |
| --- | --- | --- | --- | --- |
| Catppuccin | [catppuccin/zed](https://github.com/catppuccin/zed) | `4314cb05c74d` | `themes/catppuccin-mauve.json` | `catppuccin-latte`, `catppuccin-frappe`, `catppuccin-macchiato`, `catppuccin-mocha` |
| Tokyo Night | [ssaunderss/zed-tokyo-night](https://github.com/ssaunderss/zed-tokyo-night) | `a4ef7dd64e14` | `themes/tokyo-night.json` | `tokyo-night`, `tokyo-night-light`, `tokyo-night-storm`, `tokyo-night-moon` |
| GitHub | [PyaeSoneAungRgn/github-zed-theme](https://github.com/PyaeSoneAungRgn/github-zed-theme) | `a6a28d366211` | `themes/github_theme.json` | `github-light`, `github-light-high-contrast`, `github-dark`, `github-dark-high-contrast`, `github-dark-dimmed` |
| Dracula | [dracula/zed](https://github.com/dracula/zed) | `c419d710e77f` | `themes/dracula.json` | `dracula`, `dracula-solid`, `dracula-light-alucard` |
| Nord | [mikasius/zed-nord-theme](https://github.com/mikasius/zed-nord-theme) | `d0b459c49797` | `themes/nord.json` | `nord-dark`, `nord-darker`, `nord-light` |
| Kanagawa | [ethangilmore/zed-kanagawa](https://github.com/ethangilmore/zed-kanagawa) | `e844633f3e64` | `themes/Kanagawa.json` | `kanagawa-wave`, `kanagawa`, `kanagawa-dragon`, `kanagawa-lotus` |
| macOS Classic | [huacnlee/zed-theme-macos-classic](https://github.com/huacnlee/zed-theme-macos-classic) | `22266e11c4b8` | `themes/macos-classic.json` | `macos-classic-light`, `macos-classic-dark` |
| Nightfox | [cange/nightfox.zed](https://github.com/cange/nightfox.zed) | `3511a6f1f665` | `themes/nvim-nightfox.json` | `nightfox`, `dayfox`, `dawnfox`, `duskfox`, `nordfox`, `terafox`, `carbonfox` |

## Omissions and renames

Variants that only duplicate an already-shipped theme were left out:

- GitHub: the Colorblind and Tritanopia variants.
- Nord: the "No Italics" variants.
- Nightfox: the "blurred" variants (the opaque ones ship), and the
  ` - opaque` suffix is dropped from both the theme name and the file name.

Everything else is converted as-is. The conversion keeps the upstream palette;
`internal/migrate/zed.go` documents which Zed key each Toast token reads, and
`TestAllBuiltinThemesLoad` in `internal/theme/theme_test.go` fails if a
generated theme is missing a token the UI needs.

## Regenerating

1. Download the upstream file at the commit listed above, for example:

   ```bash
   curl -LO https://raw.githubusercontent.com/catppuccin/zed/4314cb05c74d/themes/catppuccin-mauve.json
   ```

2. Convert it, then delete the variants listed under "Omissions" and rename the
   Nightfox files (see the table above):

   ```bash
   go run ./cmd/toast migrate-theme zed catppuccin-mauve.json internal/theme/builtin
   ```

3. `go test ./internal/theme/` to confirm every token resolves.
