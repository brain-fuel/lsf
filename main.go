// Command lsf is a minimal lf-style terminal file manager whose preview pane
// renders text files bat-style: chroma syntax highlighting plus a line-number
// gutter.
//
// Keys: q quit · hjkl/arrows move · enter/l open · gg/G top/bottom ·
// ctrl-d/u half page · ctrl-f/b page · zh or . toggle hidden · / search ·
// n/N next/prev match · e edit · r reload.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gdamore/tcell/v3"
)

type mode int

const (
	modeNormal mode = iota
	modeSearch
)

type app struct {
	screen   tcell.Screen
	nav      *nav
	previews map[string]*preview
	userHost string
	mode     mode
	input    string // search prompt buffer
	msg      string // transient status-line message
	pending  rune   // first key of a g/z sequence
	quit     bool
}

func main() {
	path, err := os.Getwd()
	if len(os.Args) > 1 {
		path, err = filepath.Abs(os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lsf:", err)
		os.Exit(1)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		fmt.Fprintln(os.Stderr, "lsf: not a directory:", path)
		os.Exit(1)
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintln(os.Stderr, "lsf:", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "lsf:", err)
		os.Exit(1)
	}
	defer screen.Fini()

	app := &app{
		screen:   screen,
		nav:      newNav(path),
		previews: make(map[string]*preview),
		userHost: userAtHost(),
	}
	app.loop()
}

func (app *app) loop() {
	app.draw()
	for ev := range app.screen.EventQ() {
		switch ev := ev.(type) {
		case *tcell.EventResize:
			app.screen.Sync()
		case *tcell.EventKey:
			if !ev.Pressed() {
				continue
			}
			app.msg = ""
			if app.mode == modeSearch {
				app.handleSearchKey(ev)
			} else {
				app.handleKey(ev)
			}
		default:
			continue
		}
		if app.quit {
			return
		}
		app.draw()
	}
}

func (app *app) handleKey(ev *tcell.EventKey) {
	_, h := app.screen.Size()
	paneH := max(1, h-2)

	r := keyRune(ev)

	if app.pending != 0 {
		pending := app.pending
		app.pending = 0
		switch {
		case pending == 'g' && r == 'g':
			app.nav.top()
		case pending == 'z' && r == 'h':
			app.nav.toggleHidden()
		}
		return
	}

	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		app.quit = true
	case tcell.KeyUp:
		app.nav.up(1)
	case tcell.KeyDown:
		app.nav.down(1)
	case tcell.KeyLeft:
		app.nav.updir()
	case tcell.KeyRight, tcell.KeyEnter:
		app.openCurr()
	case tcell.KeyCtrlU:
		app.nav.up(paneH / 2)
	case tcell.KeyCtrlD:
		app.nav.down(paneH / 2)
	case tcell.KeyCtrlB:
		app.nav.up(paneH)
	case tcell.KeyCtrlF:
		app.nav.down(paneH)
	case tcell.KeyRune:
		switch r {
		case 'q':
			app.quit = true
		case 'k':
			app.nav.up(1)
		case 'j':
			app.nav.down(1)
		case 'h':
			app.nav.updir()
		case 'l':
			app.openCurr()
		case 'g', 'z':
			app.pending = r
		case 'G':
			app.nav.bottom()
		case '.':
			app.nav.toggleHidden()
		case 'r':
			app.nav.reload()
			if f := app.nav.currDir().curr(); f != nil {
				delete(app.previews, f.path)
			}
		case 'e':
			app.editCurr()
		case '/':
			app.mode = modeSearch
			app.input = ""
		case 'n':
			app.findMatch(false)
		case 'N':
			app.findMatch(true)
		}
	}
}

func (app *app) handleSearchKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		app.mode = modeNormal
		app.input = ""
	case tcell.KeyEnter:
		app.mode = modeNormal
		app.nav.search = app.input
		app.input = ""
		app.findMatch(false)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(app.input) > 0 {
			app.input = app.input[:len(app.input)-1]
		}
	case tcell.KeyRune:
		app.input += ev.Str()
	}
}

// keyRune returns the rune for a KeyRune event, or 0.
func keyRune(ev *tcell.EventKey) rune {
	if ev.Key() != tcell.KeyRune {
		return 0
	}
	for _, r := range ev.Str() {
		return r
	}
	return 0
}

func (app *app) findMatch(back bool) {
	if app.nav.search == "" {
		app.msg = "no search pattern"
		return
	}
	if !app.nav.searchNext(back) {
		app.msg = "pattern not found: " + app.nav.search
	}
}

// openCurr descends into directories and opens regular files in the editor.
func (app *app) openCurr() {
	if app.nav.open() {
		return
	}
	if f := app.nav.currDir().curr(); f != nil && f.Mode().IsRegular() {
		app.editCurr()
	}
}

// editCurr suspends the screen and runs $EDITOR (fallback vi) on the file
// under the cursor.
func (app *app) editCurr() {
	f := app.nav.currDir().curr()
	if f == nil || f.isDir() {
		return
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	if err := app.screen.Suspend(); err != nil {
		app.msg = err.Error()
		return
	}
	cmd := exec.Command(editor, f.path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if rerr := app.screen.Resume(); rerr != nil && err == nil {
		err = rerr
	}
	if err != nil {
		app.msg = err.Error()
	}
	delete(app.previews, f.path)
}
