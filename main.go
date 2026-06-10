// Command lsf is a minimal lf-style terminal file manager whose preview pane
// renders text files bat-style: chroma syntax highlighting plus a line-number
// gutter.
//
// Keys: q quit · hjkl/arrows move · enter/l open · gg/G top/bottom ·
// ctrl-d/u half page · ctrl-f/b page · zh or . toggle hidden · / search ·
// n/N next/prev match · e edit (binaries: etch) · v view with rubric
// (binaries: scry) · x hex peek in preview · r reload.
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
	hexPath  string // file whose preview shows a hex dump (x key)
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
				app.dropPreviews(f)
			}
		case 'e':
			app.editCurr()
		case 'v':
			app.viewCurr()
		case 'x':
			app.toggleHexPeek()
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

// editCurr edits the file under the cursor: $EDITOR (fallback vi) for text,
// the hex editor for binaries.
func (app *app) editCurr() {
	f := app.nav.currDir().curr()
	if f == nil || f.isDir() {
		return
	}
	if app.isBinaryFile(f) {
		app.runTool(hexEditor(), f.path)
		app.dropPreviews(f)
		return
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	app.runExternal(editor, f.path)
	app.dropPreviews(f)
}

// viewCurr views the file under the cursor read-only: rubric for text, the
// hex viewer for binaries.
func (app *app) viewCurr() {
	f := app.nav.currDir().curr()
	if f == nil || f.isDir() {
		return
	}
	if app.isBinaryFile(f) {
		app.runTool(hexViewer(), f.path)
		return
	}
	app.runTool("rubric", f.path)
}

// toggleHexPeek switches the preview pane to/from a hex dump of the file
// under the cursor. Moving to another file reverts to the normal preview.
func (app *app) toggleHexPeek() {
	f := app.nav.currDir().curr()
	if f == nil || f.isDir() || !f.Mode().IsRegular() {
		return
	}
	if app.hexPath == f.path {
		app.hexPath = ""
	} else {
		app.hexPath = f.path
	}
}

// isBinaryFile reports whether the (cached) preview classified f as binary.
func (app *app) isBinaryFile(f *file) bool {
	return f.Mode().IsRegular() && app.previewFile(f).binary
}

// hexEditor returns the hex editor command: $LSF_HEXEDITOR or etch.
func hexEditor() string {
	if c := os.Getenv("LSF_HEXEDITOR"); c != "" {
		return c
	}
	return "etch"
}

// hexViewer returns the hex viewer command: $LSF_HEXVIEWER or scry.
func hexViewer() string {
	if c := os.Getenv("LSF_HEXVIEWER"); c != "" {
		return c
	}
	return "scry"
}

// installHint maps a goforge.dev tool to its go install hint.
var installHint = map[string]string{
	"rubric": "go install goforge.dev/rubric@latest",
	"etch":   "go install goforge.dev/etch/cmd/etch@latest",
	"scry":   "go install goforge.dev/etch/cmd/scry@latest",
}

// runTool runs an external tool on a path if it is installed, otherwise
// shows an install hint on the status line.
func (app *app) runTool(name string, path string) {
	if _, err := exec.LookPath(name); err != nil {
		msg := name + " not found in PATH"
		if hint, ok := installHint[name]; ok {
			msg += " (" + hint + ")"
		}
		app.msg = msg
		return
	}
	app.runExternal(name, path)
}

// dropPreviews invalidates the cached text and hex previews of f.
func (app *app) dropPreviews(f *file) {
	delete(app.previews, f.path)
	delete(app.previews, f.path+"\x00hex")
}

// runExternal suspends the screen, runs the command attached to the
// terminal, and resumes.
func (app *app) runExternal(name string, args ...string) {
	if err := app.screen.Suspend(); err != nil {
		app.msg = err.Error()
		return
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if rerr := app.screen.Resume(); rerr != nil && err == nil {
		err = rerr
	}
	if err != nil {
		app.msg = err.Error()
	}
}
