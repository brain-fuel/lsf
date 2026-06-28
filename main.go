// Command lsf is a minimal lf-style terminal file manager whose preview pane
// renders text files bat-style: chroma syntax highlighting plus a line-number
// gutter.
//
// Keys: q quit · hjkl/arrows move · enter/l open · gg/G top/bottom ·
// ctrl-d/u half page · ctrl-f/b page · zh or . toggle hidden · / search ·
// n/N next/prev match · e edit (binaries: etch) · v view with rubric
// (binaries: scry) · x hex peek in preview · r reload.
//
// Mouse (lf's defaults): wheel moves the cursor, left click selects the
// clicked entry, middle click opens it.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"

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
	keys     keyResolver
	hexPath  string // file whose preview shows a hex dump (x key)
	quit     bool
}

// version is stamped by release builds. Go-installed binaries fall back to the
// module version recorded in build info.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help"):
		usage(stdout)
		return 0
	case len(args) == 1 && (args[0] == "version" || args[0] == "-v" || args[0] == "--version"):
		fmt.Fprintf(stdout, "lsf %s\n", resolvedVersion())
		return 0
	case len(args) > 1:
		fmt.Fprintln(stderr, "lsf: expected at most one directory")
		usage(stderr)
		return 2
	}

	path, err := os.Getwd()
	if len(args) == 1 {
		path, err = filepath.Abs(args[0])
	}
	if err != nil {
		fmt.Fprintln(stderr, "lsf:", err)
		return 1
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		fmt.Fprintln(stderr, "lsf: not a directory:", path)
		return 1
	}
	return launch(path, stderr)
}

func launch(path string, stderr io.Writer) int {
	// Force legacy keyboard reporting, the only protocol lf's tcell v2 ever
	// uses. tcell v3 otherwise negotiates kitty/win32-input keyboard
	// protocols, which several terminals implement badly enough that one
	// keypress arrives as two events (set TCELL_KEYBOARD_PROTOCOL=auto to
	// re-enable negotiation).
	screen, err := tcell.NewScreen(tcell.OptKeyboardProtocol(tcell.LegacyKeyboard))
	if err != nil {
		fmt.Fprintln(stderr, "lsf:", err)
		return 1
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintln(stderr, "lsf:", err)
		return 1
	}
	defer screen.Fini()
	screen.EnableMouse(tcell.MouseButtonEvents)

	app := &app{
		screen:   screen,
		nav:      newNav(path),
		previews: make(map[string]*preview),
		userHost: userAtHost(),
	}
	app.loop()
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `usage: lsf [directory]

A minimal lf-style terminal file manager with highlighted previews.

Keys:
  q/esc quit · hjkl/arrows move · enter/l open · e edit · v view
  gg/G top/bottom · ctrl-d/u half page · / search · x hex peek · r reload

Options:
  -h, --help       print this help
  -v, --version    print the lsf version`)
}

func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
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
		case *tcell.EventMouse:
			if app.mode != modeNormal {
				continue
			}
			app.msg = ""
			app.handleMouse(ev)
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
	key := readKey(ev)
	if key == "" {
		return
	}
	action, ok := app.keys.accept(key, normalBindings)
	if !ok {
		return
	}
	app.runCommand(action.command, action.count)
}

func (app *app) paneHeight() int {
	if app.screen == nil {
		return 1
	}
	_, h := app.screen.Size()
	return max(1, h-2)
}

func (app *app) runCommand(command string, count int) {
	if count < 1 {
		count = 1
	}

	paneH := app.paneHeight()
	switch command {
	case "quit":
		app.quit = true
	case "up":
		app.nav.up(count)
	case "down":
		app.nav.down(count)
	case "updir":
		app.nav.updir()
	case "open":
		app.openCurr()
	case "top":
		if count == 1 {
			app.nav.top()
		} else {
			app.nav.move(count - 1)
		}
	case "bottom":
		if count == 1 {
			app.nav.bottom()
		} else {
			app.nav.move(count - 1)
		}
	case "high":
		app.nav.high(paneH)
	case "middle":
		app.nav.middle(paneH)
	case "low":
		app.nav.low(paneH)
	case "half-up":
		app.nav.up(count * max(1, paneH/2))
	case "half-down":
		app.nav.down(count * max(1, paneH/2))
	case "page-up":
		app.nav.up(count * paneH)
	case "page-down":
		app.nav.down(count * paneH)
	case "scroll-up":
		app.nav.scrollUp(count, paneH)
	case "scroll-down":
		app.nav.scrollDown(count, paneH)
	case "toggle-hidden":
		app.nav.toggleHidden()
	case "reload":
		app.nav.reload()
		if f := app.nav.currDir().curr(); f != nil {
			app.dropPreviews(f)
		}
	case "edit":
		app.editCurr()
	case "view":
		app.viewCurr()
	case "hex-peek":
		app.toggleHexPeek()
	case "search":
		app.mode = modeSearch
		app.input = ""
	case "search-next":
		for range count {
			app.findMatch(false)
		}
	case "search-prev":
		for range count {
			app.findMatch(true)
		}
	}
}

// handleMouse implements lf's default mouse bindings: wheel moves the
// cursor, left click selects the row under the pointer (entering the
// clicked pane's directory if needed), middle click opens it.
func (app *app) handleMouse(ev *tcell.EventMouse) {
	w, h := app.screen.Size()
	paneH := max(1, h-2)

	btn := ev.Buttons()
	switch {
	case btn&tcell.WheelUp != 0:
		app.nav.up(1)
		return
	case btn&tcell.WheelDown != 0:
		app.nav.down(1)
		return
	}
	open := btn&tcell.Button2 != 0
	if btn&tcell.Button1 == 0 && !open {
		return
	}

	x, y := ev.Position()
	row := y - 1 // pane rows start under the prompt line
	if row < 0 || row >= paneH {
		return
	}
	ws, xs := columnWidths(w)
	pane := -1
	for i := range xs {
		if x >= xs[i] && x < xs[i]+ws[i] {
			pane = i
		}
	}

	var d *dir
	switch pane {
	case 0:
		d = app.nav.parentDir()
	case 1:
		d = app.nav.currDir()
	case 2:
		// The preview pane is the listing of the directory under the
		// cursor; clicks on a file preview only mean "open" (lf's m-2).
		f := app.nav.currDir().curr()
		if f == nil {
			return
		}
		if !f.isDir() {
			if open {
				app.openCurr()
			}
			return
		}
		d = app.nav.dir(f.path)
		d.off = 0 // drawDirPreview always renders from the top
	}
	if d == nil || d.err != nil {
		return
	}
	i := d.off + row
	if i >= len(d.files) {
		return
	}

	// Select the clicked entry, entering the clicked pane's directory.
	app.nav.path = d.path
	d.ind = i
	if open {
		app.openCurr()
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
		// --paging always so a one-screen dump doesn't flash and
		// vanish when less quits via -F. Only the known default gets
		// flags; a custom $LSF_HEXVIEWER may not understand them.
		if v := hexViewer(); v == "scry" {
			app.runTool(v, "--paging", "always", f.path)
		} else {
			app.runTool(v, f.path)
		}
		return
	}
	app.runTool("rubric", "--paging=always", f.path)
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

// runTool runs an external tool if it is installed, otherwise shows an
// install hint on the status line.
func (app *app) runTool(name string, args ...string) {
	if _, err := exec.LookPath(name); err != nil {
		msg := name + " not found in PATH"
		if hint, ok := installHint[name]; ok {
			msg += " (" + hint + ")"
		}
		app.msg = msg
		return
	}
	app.runExternal(name, args...)
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
