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
| `l`, right, `enter` | enter directory / edit file |
| `gg` / `G` | first / last entry |
| `ctrl-d` / `ctrl-u` | half page down / up |
| `ctrl-f` / `ctrl-b` | page down / up |
| `zh`, `.` | toggle hidden files |
| `/` | search (then `n`/`N` for next/previous match) |
| `e` | open file in `$EDITOR` (fallback `vi`) |
| `v` | view file with [rubric](https://goforge.dev/rubric) (paged, read-only) |
| `r` | reload current directory and preview |

## Configuration

- `LSF_THEME` — chroma theme name for previews (default `monokai`).
  Any name from chroma's gallery works: `dracula`, `github`, `nord`,
  `solarized-dark`, …
- `EDITOR` — editor used by `e`/`enter` on files.

## Design notes

- Layout, navigation model (cursor + scroll offset with scrolloff, per-dir
  cursor memory), dirs-first natural sort, and the prompt/status lines follow
  lf's defaults (1:2:3 column ratios).
- Preview pipeline follows rubric/bat: NUL-byte binary sniff, lexer by file
  name then content analysis, chroma tokenize → per-line segments, 4-space
  tabs, `nnn │` gutter in xterm-238 grey.
- Previews are cached per path and invalidated by mtime/size.
- Files are read up to 512 KiB and highlighted up to 500 lines.
