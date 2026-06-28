# lsf / lf parity

Reference: local `gokcehan/lf` checkout at `190e305`.

`lsf` started as a minimal `lf`-style browser with a richer source preview.
Full `lf` parity is not a pile of extra key cases; upstream `lf` is built around
a command evaluator, configurable mappings, stateful navigation, selections,
preview processes, and a client/server control plane. The path to parity should
therefore be staged around subsystems.

## Current lsf surface

- Three-pane Miller layout with `1:2:3` ratios.
- Parent/current/preview panes.
- Natural sort with directories first.
- Hidden-file toggle for dotfiles.
- Cursor memory per visited directory.
- Directory preview.
- Text preview with Chroma syntax highlighting and line-number gutter.
- Binary detection, hex peek, `etch`/`scry` integration.
- Basic navigation: `j/k`, arrows, `h/l`, `gg/G`, `H/M/L`, viewport scroll,
  half/page up/down, home/end, and numeric prefixes for existing bindings.
- Basic search: `/`, `n`, `N`.
- Mouse wheel, primary click, middle click.
- `$EDITOR`, `rubric`, `etch`, and `scry` handoff.

## Upstream lf capabilities missing from lsf

### Input and command model

- Config file loading and `-command`.
- Command parser/evaluator for built-ins and user commands.
- Normal, visual, and command-line modes.
- Configurable `map`, `nmap`, `vmap`, and `cmap`.
- User-defined multi-key mappings.
- Binding discovery menu for partial mappings.
- Readline-like command-line editing, history, completion, and menus.
- `push` command for simulated input.

### Navigation

- Jump list (`[` / `]`).
- `find`, `find-back`, `find-next`, `find-prev`.
- Reverse search (`?`) and search methods (`text`, `glob`, `regex`).
- Incremental search and filter.
- Wrapscan and wrapscroll options.
- Starting at a file path and selecting it.

### File selection and operations

- Toggle, invert, visual selection, unselect.
- Copy, cut, paste, clear, delete, rename.
- Clipboard state shared through server.
- Duplicate-name policy (`dupfilefmt`).
- Attribute preservation (`mode`, `timestamps`).
- Selection output mode (`-print-selection`, `-selection-path`).
- Selection modes (`all`, `dir`).

### Marks, tags, and metadata

- Marks/bookmarks (`m`, `'`, `"`).
- Temporary marks.
- Tags and tag display.
- Custom info (`addcustominfo`).
- Directory-size calculation (`calcdirsize`).

### Sorting, filtering, and display options

- Sort modes: name, ext, size, time, atime, btime, ctime, custom.
- Reverse sort.
- Dir-only mode.
- Hidden-file glob patterns with negation.
- Per-directory local options.
- Info columns: perm, user, group, size, time, atime, btime, ctime, custom.
- Binary vs decimal size units.
- Number and relative-number columns.
- Icons.
- GNU `LS_COLORS`-style colors.
- Borders, border styles, configurable formats, ruler.
- Prompt/stat/ruler format expansion.
- Filename truncation policy.
- Configurable pane ratios and preview toggle.

### Previewing

- External previewer command with width/height/x/y/mode arguments.
- Preview cleaner command.
- Volatile preview cache disabling on previewer failure.
- Preview preloading.
- Directory preview option.
- Sixel/image-oriented preview support.
- Configurable tab width and binary heuristic semantics.

### Shell integration and remote control

- Server/client architecture.
- `-remote` commands and `query`.
- `-server`, `-single`, `autoquit`.
- `-print-last-dir`, `-last-dir-path`.
- Shell command modes: `$`, `%`, `!`, `&`.
- Environment contract: `f`, `fs`, `fv`, `fx`, `id`, `PWD`, `OLDPWD`,
  `LF_LEVEL`, `lf_*`, dimensions, count, and mode.
- Hooks: `pre-cd`, `on-cd`, `on-load`, `on-init`, `on-select`, `on-redraw`,
  `on-quit`, focus hooks.

### Filesystem freshness

- Async directory/file loading.
- File operation progress.
- Periodic reload (`period`).
- Optional filesystem watch via `fsnotify`.
- Cache invalidation for removed/changed paths.

### Platform integration

- Windows-specific shell/open defaults.
- Shell integration scripts.
- Desktop entry, shell completions, man page generation.
- Logging and CPU/memory profiles.

## Suggested parity phases

1. **Navigation correctness**: match lf movement defaults, scroll commands,
   counts, `H/M/L`, `home/end`, `pgup/pgdn`, and path-or-file startup.
2. **Input architecture**: replace hardcoded switch handling with an lf-style
   key-string mapper and command dispatch table.
3. **Config surface**: add `set`, `map`, config loading, and enough options to
   drive navigation/display without recompiling.
4. **Selections and file ops**: selection state, visual mode, copy/cut/paste,
   delete, rename, and shell command overrides.
5. **Preview parity**: external previewer/cleaner, preloading, volatile caches,
   and configurable preview behavior while preserving lsf's Chroma preview as
   the default.
6. **Search/filter/sort parity**: find, search-back, glob/regex matching,
   filters, all lf sort modes, and local options.
7. **Marks/tags/info/ruler**: bookmarks, tags, info columns, status/ruler
   formatting, colors, icons, and numbers.
8. **Server and shell integration**: remote commands, query state, last-dir and
   selection output, hooks, logging, and scripts.
9. **Filesystem freshness**: async loading, operation progress, periodic reload,
   and optional `fsnotify` watch.

## Bug standard

For each parity phase:

- import the upstream behavior from `lf` docs/source before implementing
- add focused tests for state transitions
- prefer deterministic model tests over terminal-only manual checks
- keep `rubric`/Chroma preview as lsf's deliberate improvement
- document every intentional divergence from `lf`
