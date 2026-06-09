package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/gdamore/tcell/v3"
)

const (
	// previewMaxBytes caps how much of a file is read for previewing.
	previewMaxBytes = 512 * 1024
	// previewMaxLines caps how many lines are highlighted and cached.
	previewMaxLines = 500
	tabWidth        = 4
	defaultTheme    = "monokai"
)

var gutterStyle = tcell.StyleDefault.Foreground(tcell.PaletteColor(238))

type seg struct {
	text  string
	style tcell.Style
}

type preview struct {
	lines   [][]seg
	binary  bool
	empty   bool
	tooLong bool // file had more lines than previewMaxLines
	err     error
	// cache validity
	mtime int64
	size  int64
}

// previewFile returns a (possibly cached) highlighted preview of f.
func (app *app) previewFile(f *file) *preview {
	if p, ok := app.previews[f.path]; ok &&
		p.mtime == f.ModTime().UnixNano() && p.size == f.Size() {
		return p
	}
	p := renderPreview(f)
	p.mtime = f.ModTime().UnixNano()
	p.size = f.Size()
	app.previews[f.path] = p
	return p
}

func renderPreview(f *file) *preview {
	p := &preview{}
	fh, err := os.Open(f.path)
	if err != nil {
		p.err = err
		return p
	}
	defer fh.Close()
	buf := make([]byte, previewMaxBytes)
	n, err := fh.Read(buf)
	if n == 0 {
		if err != nil && err.Error() != "EOF" {
			p.err = err
		} else {
			p.empty = true
		}
		return p
	}
	buf = buf[:n]
	if isBinary(buf) {
		p.binary = true
		return p
	}
	source := string(buf)

	lexer := lexers.Match(filepath.Base(f.path))
	if lexer == nil {
		head := source
		if len(head) > 4096 {
			head = head[:4096]
		}
		lexer = lexers.Analyse(head)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	theme := styles.Get(themeName())
	if theme == nil {
		theme = styles.Fallback
	}

	it, err := lexer.Tokenise(nil, source)
	if err != nil {
		// Highlighting failed; fall back to plain lines.
		for _, l := range strings.SplitAfter(source, "\n") {
			p.appendLine([]seg{{text: l, style: tcell.StyleDefault}})
			if p.tooLong {
				break
			}
		}
		return p
	}
	for _, line := range chroma.SplitTokensIntoLines(it.Tokens()) {
		segs := make([]seg, 0, len(line))
		for _, tok := range line {
			segs = append(segs, seg{text: tok.Value, style: entryStyle(theme.Get(tok.Type))})
		}
		p.appendLine(segs)
		if p.tooLong {
			break
		}
	}
	return p
}

func (p *preview) appendLine(segs []seg) {
	if len(p.lines) >= previewMaxLines {
		p.tooLong = true
		return
	}
	p.lines = append(p.lines, segs)
}

func themeName() string {
	if t := os.Getenv("LSF_THEME"); t != "" {
		return t
	}
	return defaultTheme
}

// entryStyle converts a chroma style entry to a tcell style.
func entryStyle(e chroma.StyleEntry) tcell.Style {
	st := tcell.StyleDefault
	if e.Colour.IsSet() {
		st = st.Foreground(tcell.NewRGBColor(
			int32(e.Colour.Red()), int32(e.Colour.Green()), int32(e.Colour.Blue())))
	}
	if e.Bold == chroma.Yes {
		st = st.Bold(true)
	}
	if e.Italic == chroma.Yes {
		st = st.Italic(true)
	}
	if e.Underline == chroma.Yes {
		st = st.Underline(true)
	}
	return st
}

// isBinary reports whether the buffer looks like binary data (any NUL byte
// in the first KiB, matching the heuristic bat and rubric use).
func isBinary(b []byte) bool {
	if len(b) > 1024 {
		b = b[:1024]
	}
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

// drawPreview renders the preview pane for the file under the cursor.
func (app *app) drawPreview(win rect) {
	d := app.nav.currDir()
	f := d.curr()
	if f == nil {
		return
	}
	if f.isDir() {
		app.drawDirPreview(win, f)
		return
	}
	if !f.Mode().IsRegular() {
		printLine(app.screen, win.x, win.y, win.w, modeDesc(f), msgStyle)
		return
	}
	p := app.previewFile(f)
	switch {
	case p.err != nil:
		printLine(app.screen, win.x, win.y, win.w, p.err.Error(), errStyle)
		return
	case p.binary:
		printLine(app.screen, win.x, win.y, win.w, "binary", msgStyle)
		return
	case p.empty:
		printLine(app.screen, win.x, win.y, win.w, "empty", msgStyle)
		return
	}

	numW := digits(len(p.lines))
	if numW < 3 {
		numW = 3
	}
	for row := 0; row < win.h && row < len(p.lines); row++ {
		gutter := fmt.Sprintf(" %*d │ ", numW, row+1)
		printLine(app.screen, win.x, win.y+row, win.w, gutter, gutterStyle)
		col := 0
		maxw := win.w - numW - 4
		for _, sg := range p.lines[row] {
			col = drawSeg(app.screen, win.x+numW+4, win.y+row, maxw, col, sg)
			if col >= maxw {
				break
			}
		}
	}
}

// drawSeg draws one highlighted segment starting at column col within the
// preview text area, expanding tabs and stopping at maxw. Returns the new
// column.
func drawSeg(s tcell.Screen, x, y, maxw, col int, sg seg) int {
	for _, r := range strings.TrimRight(sg.text, "\n") {
		if col >= maxw {
			break
		}
		if r == '\t' {
			next := (col/tabWidth + 1) * tabWidth
			for col < next && col < maxw {
				s.PutStrStyled(x+col, y, " ", sg.style)
				col++
			}
			continue
		}
		s.PutStrStyled(x+col, y, string(r), sg.style)
		col++
	}
	return col
}

func (app *app) drawDirPreview(win rect, f *file) {
	d := app.nav.dir(f.path)
	if d.err != nil {
		printLine(app.screen, win.x, win.y, win.w, d.err.Error(), errStyle)
		return
	}
	if len(d.files) == 0 {
		printLine(app.screen, win.x, win.y, win.w, "empty", msgStyle)
		return
	}
	for row := 0; row < win.h && row < len(d.files); row++ {
		ff := d.files[row]
		printLine(app.screen, win.x, win.y+row, win.w, " "+ff.Name(), fileStyle(ff))
	}
}

func modeDesc(f *file) string {
	switch {
	case f.Mode()&os.ModeSymlink != 0:
		if target, err := os.Readlink(f.path); err == nil {
			return "-> " + target
		}
		return "symlink"
	case f.Mode()&os.ModeNamedPipe != 0:
		return "fifo"
	case f.Mode()&os.ModeSocket != 0:
		return "socket"
	case f.Mode()&os.ModeDevice != 0:
		return "device"
	default:
		return f.Mode().String()
	}
}

func digits(n int) int {
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}
