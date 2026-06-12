# lsf

A minimal [lf](https://github.com/gokcehan/lf)-style terminal file manager
whose preview pane renders text files the way
[bat](https://github.com/sharkdp/bat) /
[rubric](https://goforge.dev/rubric) do: syntax highlighting (via
[chroma](https://github.com/alecthomas/chroma)) with a line-number gutter,
instead of lf's plain-text preview.

## Install

```
go install goforge.dev/lsf@latest
```

## Usage

```
lsf [directory]
```

Three miller columns: parent directory, current directory, preview. The
preview shows highlighted text for regular files, a listing for directories,
`binary` for binary files, and the target for symlinks.

## Keys

| Key | Action |
|-----|--------|
| `q`, `esc` | quit |
| `j`/`k`, arrows | move down/up |
| `h`, left | parent directory |
| `l`, right, `enter` | enter directory / edit file (binaries: hex edit with [etch](https://goforge.dev/etch)) |
| `gg` / `G` | first / last entry |
| `ctrl-d` / `ctrl-u` | half page down / up |
| `ctrl-f` / `ctrl-b` | page down / up |
| `zh`, `.` | toggle hidden files |
| `/` | search (then `n`/`N` for next/previous match) |
| `e` | open file in `$EDITOR` (fallback `vi`); binaries open in [etch](https://goforge.dev/etch) |
| `v` | view file with [rubric](https://goforge.dev/rubric) (paged, read-only); binaries with [scry](https://goforge.dev/etch) |
| `x` | toggle hex peek: preview pane shows an `xxd`-style dump of the file under the cursor |
| `r` | reload current directory and preview |

The mouse works as in lf: the wheel moves the cursor, left click selects
the clicked entry (entering the clicked pane's directory), middle click
opens it.

## Configuration

- `LSF_THEME` — chroma theme name for previews (default `monokai`).
  Any name from chroma's gallery works: `dracula`, `github`, `nord`,
  `solarized-dark`, …
- `EDITOR` — editor used by `e`/`enter` on text files.
- `LSF_HEXEDITOR` — hex editor used by `e`/`enter` on binary files
  (default `etch`).
- `LSF_HEXVIEWER` — hex viewer used by `v` on binary files (default `scry`).

## Design notes

- Layout, navigation model (cursor + scroll offset with scrolloff, per-dir
  cursor memory), dirs-first natural sort, and the prompt/status lines follow
  lf's defaults (1:2:3 column ratios).
- Preview pipeline follows rubric/bat: NUL-byte binary sniff (on a small
  head read, so executables cost microseconds, not a 512 KiB read), lexer by
  file name then content analysis, source cut at 500 lines before
  tokenizing, chroma tokenize → per-line segments, 4-space tabs, `nnn │`
  gutter in xterm-238 grey.
- Previews are cached per path and invalidated by mtime/size.
- Files are read up to 512 KiB and highlighted up to 500 lines.
