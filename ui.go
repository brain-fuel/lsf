package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v3"
)

// ANSI palette indices so the terminal theme's colors apply, as in lf.
var (
	colorRed   = tcell.PaletteColor(1)
	colorGreen = tcell.PaletteColor(2)
	colorBlue  = tcell.PaletteColor(4)
	colorCyan  = tcell.PaletteColor(6)

	msgStyle    = tcell.StyleDefault.Foreground(tcell.PaletteColor(244)).Italic(true)
	errStyle    = tcell.StyleDefault.Foreground(colorRed)
	promptStyle = tcell.StyleDefault.Foreground(colorGreen).Bold(true)
	pathStyle   = tcell.StyleDefault.Foreground(colorBlue).Bold(true)
	fileNameSt  = tcell.StyleDefault.Bold(true)
)

type rect struct {
	x, y, w, h int
}

// printLine draws str at (x, y) clipped to maxw display columns and returns
// the number of columns used.
func printLine(s tcell.Screen, x, y, maxw int, str string, st tcell.Style) int {
	col := 0
	for _, r := range str {
		if col >= maxw {
			break
		}
		s.PutStrStyled(x+col, y, string(r), st)
		col++
	}
	return col
}

func fileStyle(f *file) tcell.Style {
	st := tcell.StyleDefault
	switch {
	case f.broken:
		return st.Foreground(colorRed)
	case f.Mode()&os.ModeSymlink != 0:
		return st.Foreground(colorCyan)
	case f.IsDir():
		return st.Foreground(colorBlue).Bold(true)
	case f.Mode()&0111 != 0:
		return st.Foreground(colorGreen).Bold(true)
	}
	return st
}

// columnWidths splits the usable width into three panes with lf's default
// 1:2:3 ratios, separated by single-column gaps.
func columnWidths(w int) (ws, xs [3]int) {
	ratios := [3]int{1, 2, 3}
	const gap = 1
	avail := w - 2*gap
	if avail < 3 {
		avail = 3
	}
	ws[0] = avail * ratios[0] / 6
	ws[1] = avail * ratios[1] / 6
	ws[2] = avail - ws[0] - ws[1]
	xs[0] = 0
	xs[1] = ws[0] + gap
	xs[2] = xs[1] + ws[1] + gap
	return ws, xs
}

func (app *app) draw() {
	s := app.screen
	s.Clear()
	w, h := s.Size()
	if h < 3 || w < 12 {
		s.Show()
		return
	}
	ws, xs := columnWidths(w)
	paneH := h - 2

	app.drawPromptLine(w)

	if parent := app.nav.parentDir(); parent != nil {
		parent.bound(paneH)
		app.drawPane(rect{xs[0], 1, ws[0], paneH}, parent, false)
	}
	curr := app.nav.currDir()
	curr.bound(paneH)
	app.drawPane(rect{xs[1], 1, ws[1], paneH}, curr, true)
	app.drawPreview(rect{xs[2], 1, ws[2], paneH})

	app.drawStatusLine(w, h)
	s.Show()
}

func (app *app) drawPromptLine(w int) {
	x := 0
	if app.userHost != "" {
		x += printLine(app.screen, x, 0, w, app.userHost+":", promptStyle)
	}
	dir := shortenPath(app.nav.path, w/2)
	if dir != "/" {
		dir += "/"
	}
	x += printLine(app.screen, x, 0, w-x, dir, pathStyle)
	if f := app.nav.currDir().curr(); f != nil {
		printLine(app.screen, x, 0, w-x, f.Name(), fileNameSt)
	}
}

func (app *app) drawPane(win rect, d *dir, active bool) {
	s := app.screen
	if d.err != nil {
		printLine(s, win.x, win.y, win.w, d.err.Error(), errStyle)
		return
	}
	if len(d.files) == 0 {
		printLine(s, win.x, win.y, win.w, "empty", msgStyle)
		return
	}
	for row := 0; row < win.h; row++ {
		i := d.off + row
		if i >= len(d.files) {
			break
		}
		f := d.files[i]
		st := fileStyle(f)
		if i == d.ind {
			st = st.Reverse(true)
		}
		info := ""
		if active && f.Mode().IsRegular() {
			info = humanSize(f.Size())
		}
		drawRow(s, win, row, f.Name(), info, st, i == d.ind)
	}
}

// drawRow draws " name…padding…info " across the pane width. When the row is
// the cursor, the style spans the full width.
func drawRow(s tcell.Screen, win rect, row int, name, info string, st tcell.Style, cursor bool) {
	y := win.y + row
	maxName := win.w - 2
	if info != "" {
		maxName = win.w - len(info) - 3
	}
	if maxName < 1 {
		maxName = 1
	}
	col := printLine(s, win.x, y, win.w, " ", st)
	nr := []rune(name)
	if len(nr) > maxName {
		nr = append(nr[:maxName-1], '~')
	}
	col += printLine(s, win.x+col, y, maxName, string(nr), st)
	if !cursor && info == "" {
		return
	}
	pad := win.w - col - len(info)
	if info != "" {
		pad-- // trailing space after info
	}
	for ; pad > 0; pad-- {
		s.PutStrStyled(win.x+col, y, " ", st)
		col++
	}
	if info != "" {
		col += printLine(s, win.x+col, y, win.w-col, info+" ", st)
	}
}

func (app *app) drawStatusLine(w, h int) {
	s := app.screen
	y := h - 1
	if app.mode == modeSearch {
		printLine(s, 0, y, w, "/"+app.input, tcell.StyleDefault)
		s.ShowCursor(len(app.input)+1, y)
		return
	}
	s.HideCursor()
	if app.msg != "" {
		printLine(s, 0, y, w, app.msg, errStyle)
		return
	}
	d := app.nav.currDir()
	left := ""
	if f := d.curr(); f != nil {
		left = fmt.Sprintf("%s %4s %s", f.Mode(), humanSize(f.Size()),
			f.ModTime().Format("Jan _2 15:04"))
		if f.Mode()&os.ModeSymlink != 0 {
			if target, err := os.Readlink(f.path); err == nil {
				left += " -> " + target
			}
		}
	}
	right := fmt.Sprintf("%d/%d", min(d.ind+1, len(d.files)), len(d.files))
	printLine(s, 0, y, w-len(right)-1, left, tcell.StyleDefault)
	printLine(s, w-len(right), y, len(right), right, tcell.StyleDefault)
}

func humanSize(size int64) string {
	if size < 1000 {
		return fmt.Sprintf("%dB", size)
	}
	suffixes := "KMGTPE"
	curr := float64(size) / 1000
	for i := 0; ; i++ {
		if curr < 10 {
			return fmt.Sprintf("%.1f%c", curr, suffixes[i])
		}
		if curr < 1000 {
			return fmt.Sprintf("%.0f%c", curr, suffixes[i])
		}
		curr /= 1000
	}
}

func userAtHost() string {
	u, err := user.Current()
	if err != nil {
		return ""
	}
	host, err := os.Hostname()
	if err != nil {
		return u.Username
	}
	return u.Username + "@" + host
}

// shortenPath is used in the prompt when the path would not fit.
func shortenPath(path string, maxw int) string {
	if len(path) <= maxw {
		return path
	}
	parts := strings.Split(path, string(filepath.Separator))
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] != "" {
			parts[i] = parts[i][:1]
		}
		if len(strings.Join(parts, string(filepath.Separator))) <= maxw {
			break
		}
	}
	return strings.Join(parts, string(filepath.Separator))
}
