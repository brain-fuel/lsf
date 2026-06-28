package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"file2", "file10", true},
		{"file10", "file2", false},
		{"a", "b", true},
		{"file1", "file1", false},
		{"file01", "file1", true},
		{"abc", "abcd", true},
		{"10", "9", false},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestIsBinary(t *testing.T) {
	if isBinary([]byte("hello\nworld\n")) {
		t.Error("text flagged binary")
	}
	if !isBinary([]byte{0x7f, 'E', 'L', 'F', 0, 0, 1}) {
		t.Error("ELF header not flagged binary")
	}
}

func TestRenderPreviewBinary(t *testing.T) {
	path := "/bin/ls"
	info, err := os.Lstat(path)
	if err != nil {
		t.Skip("/bin/ls unavailable")
	}
	p := renderPreview(&file{FileInfo: info, path: path})
	if !p.binary {
		t.Error("ELF binary not detected as binary")
	}
}

func TestRenderPreviewGo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	src := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(path)
	p := renderPreview(&file{FileInfo: info, path: path})
	if p.err != nil || p.binary || p.empty {
		t.Fatalf("unexpected preview state: %+v", p)
	}
	if len(p.lines) < 5 {
		t.Fatalf("got %d lines, want >= 5", len(p.lines))
	}
	// The keyword "package" should get a non-default style from the theme.
	styled := false
	for _, sg := range p.lines[0] {
		if sg.style != tcell.StyleDefault {
			styled = true
		}
	}
	if !styled {
		t.Error("first line has no highlighted segments")
	}
}

func TestRenderPreviewEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(path)
	p := renderPreview(&file{FileInfo: info, path: path})
	if !p.empty {
		t.Error("empty file not detected")
	}
}

func TestDirBound(t *testing.T) {
	d := testDir(100)
	h := 10

	d.ind = 0
	d.bound(h)
	if d.off != 0 {
		t.Errorf("top: off = %d, want 0", d.off)
	}

	d.ind = 99
	d.bound(h)
	if d.off != 90 {
		t.Errorf("bottom: off = %d, want 90", d.off)
	}
	if d.ind-d.off >= h {
		t.Errorf("cursor outside window: ind=%d off=%d h=%d", d.ind, d.off, h)
	}

	d.ind, d.off = 50, 0
	d.bound(h)
	if d.off != 41 {
		t.Errorf("middle: off = %d, want 41", d.off)
	}
	if d.ind < d.off+scrolloff || d.ind > d.off+h-1-scrolloff {
		t.Errorf("scrolloff violated: ind=%d off=%d", d.ind, d.off)
	}

	d.ind = 200
	d.bound(h)
	if d.ind != 99 {
		t.Errorf("ind not clamped: %d", d.ind)
	}

	empty := &dir{}
	empty.bound(h)
	if empty.ind != 0 || empty.off != 0 {
		t.Error("empty dir not reset")
	}
}

func TestNavScrollCommands(t *testing.T) {
	n := testNav(100)
	d := n.currDir()
	h := 10

	d.ind = 50
	d.bound(h)
	if d.off != 41 {
		t.Fatalf("setup off = %d, want 41", d.off)
	}

	n.scrollUp(1, h)
	if d.ind != 49 || d.off != 40 {
		t.Fatalf("scrollUp at bottom row: ind=%d off=%d, want ind=49 off=40", d.ind, d.off)
	}

	d.ind, d.off = 50, 45
	n.scrollUp(1, h)
	if d.ind != 50 || d.off != 44 {
		t.Fatalf("scrollUp with cursor room: ind=%d off=%d, want ind=50 off=44", d.ind, d.off)
	}

	d.ind, d.off = 5, 0
	n.scrollUp(1, h)
	if d.ind != 4 || d.off != 0 {
		t.Fatalf("scrollUp at top: ind=%d off=%d, want ind=4 off=0", d.ind, d.off)
	}

	d.ind, d.off = 50, 45
	n.scrollDown(1, h)
	if d.ind != 50 || d.off != 46 {
		t.Fatalf("scrollDown with cursor room: ind=%d off=%d, want ind=50 off=46", d.ind, d.off)
	}

	d.ind, d.off = 50, 50
	n.scrollDown(1, h)
	if d.ind != 51 || d.off != 51 {
		t.Fatalf("scrollDown at top row: ind=%d off=%d, want ind=51 off=51", d.ind, d.off)
	}

	d.ind, d.off = 95, 90
	n.scrollDown(1, h)
	if d.ind != 96 || d.off != 90 {
		t.Fatalf("scrollDown at bottom: ind=%d off=%d, want ind=96 off=90", d.ind, d.off)
	}
}

func TestNavScreenPositionCommands(t *testing.T) {
	n := testNav(100)
	d := n.currDir()
	h := 10

	d.ind = 50
	d.bound(h)

	n.high(h)
	if d.ind != 41 {
		t.Fatalf("high: ind=%d, want 41", d.ind)
	}
	n.middle(h)
	if d.ind != 46 {
		t.Fatalf("middle: ind=%d, want 46", d.ind)
	}
	n.low(h)
	if d.ind != 50 {
		t.Fatalf("low: ind=%d, want 50", d.ind)
	}
}

func TestKeyResolverCountsAndMultiKeyBindings(t *testing.T) {
	var resolver keyResolver

	if action, ok := resolver.accept("5", normalBindings); ok || action.command != "" {
		t.Fatalf("count prefix resolved early: action=%+v ok=%v", action, ok)
	}
	action, ok := resolver.accept("j", normalBindings)
	if !ok || action.command != "down" || action.count != 5 {
		t.Fatalf("5j resolved to action=%+v ok=%v, want down count 5", action, ok)
	}

	action, ok = resolver.accept("g", normalBindings)
	if ok || action.command != "" {
		t.Fatalf("g prefix resolved early: action=%+v ok=%v", action, ok)
	}
	action, ok = resolver.accept("g", normalBindings)
	if !ok || action.command != "top" || action.count != 1 {
		t.Fatalf("gg resolved to action=%+v ok=%v, want top count 1", action, ok)
	}

	if action, ok = resolver.accept("3", normalBindings); ok || action.command != "" {
		t.Fatalf("count prefix resolved early: action=%+v ok=%v", action, ok)
	}
	action, ok = resolver.accept("G", normalBindings)
	if !ok || action.command != "bottom" || action.count != 3 {
		t.Fatalf("3G resolved to action=%+v ok=%v, want bottom count 3", action, ok)
	}

	if _, ok = resolver.accept("g", normalBindings); ok {
		t.Fatal("g prefix resolved early")
	}
	if action, ok = resolver.accept("<esc>", normalBindings); ok || action.command != "" {
		t.Fatalf("escape while pending resolved action=%+v ok=%v", action, ok)
	}
	action, ok = resolver.accept("g", normalBindings)
	if ok || action.command != "" {
		t.Fatalf("g after escape resolved early: action=%+v ok=%v", action, ok)
	}
	action, ok = resolver.accept("g", normalBindings)
	if !ok || action.command != "top" {
		t.Fatalf("gg after escape resolved to action=%+v ok=%v, want top", action, ok)
	}
}

func TestRunCommandHonorsCounts(t *testing.T) {
	app := &app{nav: testNav(100)}
	d := app.nav.currDir()

	app.runCommand("down", 5)
	if d.ind != 5 {
		t.Fatalf("down count: ind=%d, want 5", d.ind)
	}
	app.runCommand("up", 2)
	if d.ind != 3 {
		t.Fatalf("up count: ind=%d, want 3", d.ind)
	}
	app.runCommand("bottom", 3)
	if d.ind != 2 {
		t.Fatalf("3G/bottom count: ind=%d, want 2", d.ind)
	}
	app.runCommand("top", 4)
	if d.ind != 3 {
		t.Fatalf("4gg/top count: ind=%d, want 3", d.ind)
	}
}

func TestTerminalNavigationKeypressesDoNotDoubleFire(t *testing.T) {
	const (
		width     = 80
		height    = 12
		fileCount = 20
		ctrlE     = "\x05"
		ctrlY     = "\x19"
	)

	dir := t.TempDir()
	for index := 0; index < fileCount; index++ {
		name := fmt.Sprintf("%02d.txt", index)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	term := vt.NewMockTerm(vt.MockOptSize{X: width, Y: height})
	screen, err := tcell.NewTerminfoScreenFromTty(term,
		tcell.OptKeyboardProtocol(tcell.LegacyKeyboard),
		tcell.OptNegotiation(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.EnableMouse(tcell.MouseButtonEvents)

	testApp := &app{
		screen:   screen,
		nav:      newNav(dir),
		previews: make(map[string]*preview),
		userHost: "test@host",
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		testApp.loop()
	}()
	t.Cleanup(func() {
		term.SendRaw([]byte("q"))
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Log("lsf terminal test loop did not stop before cleanup")
		}
		screen.Fini()
		if err := term.Close(); err != nil {
			t.Logf("mock terminal close failed: %v", err)
		}
	})

	waitForStatus(t, term, width, height, "1/20")

	term.SendRaw([]byte("j"))
	waitForStatus(t, term, width, height, "2/20")

	term.SendRaw([]byte("k"))
	waitForStatus(t, term, width, height, "1/20")

	term.SendRaw([]byte(ctrlE))
	waitForStatus(t, term, width, height, "2/20")

	term.SendRaw([]byte(ctrlY))
	waitForStatus(t, term, width, height, "2/20")

	term.SendRaw([]byte("gg"))
	waitForStatus(t, term, width, height, "1/20")

	term.SendRaw([]byte("5j"))
	waitForStatus(t, term, width, height, "6/20")

	term.SendRaw([]byte("gg"))
	waitForStatus(t, term, width, height, "1/20")

	term.SendRaw([]byte("3G"))
	waitForStatus(t, term, width, height, "3/20")

	term.SendRaw([]byte("G"))
	waitForStatus(t, term, width, height, "20/20")

	term.SendRaw([]byte(ctrlY))
	waitForStatus(t, term, width, height, "19/20")

	term.SendRaw([]byte("q"))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lsf did not quit after q")
	}
}

func waitForStatus(t *testing.T, term vt.MockTerm, width, height int, want string) {
	t.Helper()

	var got string
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := term.Drain(); err != nil {
			t.Fatal(err)
		}
		got = strings.TrimSpace(terminalLine(term, width, height-1))
		if strings.HasSuffix(got, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status line = %q, want suffix %q\nterminal:\n%s", got, want, terminalText(term, width, height))
}

func terminalText(term vt.MockTerm, width, height int) string {
	var lines []string
	for row := 0; row < height; row++ {
		lines = append(lines, strings.TrimRight(terminalLine(term, width, row), " "))
	}
	return strings.Join(lines, "\n")
}

func terminalLine(term vt.MockTerm, width, row int) string {
	var line strings.Builder
	for col := 0; col < width; col++ {
		cell := term.GetCell(vt.Coord{X: vt.Col(col), Y: vt.Row(row)})
		if cell.C == "" {
			line.WriteByte(' ')
			continue
		}
		line.WriteString(cell.C)
	}
	return line.String()
}

func testDir(n int) *dir {
	d := &dir{}
	for i := 0; i < n; i++ {
		d.files = append(d.files, &file{})
	}
	return d
}

func testNav(fileCount int) *nav {
	path := "/test"
	return &nav{
		dirs: map[string]*dir{
			path: testDir(fileCount),
		},
		path: path,
	}
}

func TestNthNewline(t *testing.T) {
	s := "a\nb\nc\nd\n"
	cases := []struct{ n, want int }{{1, 1}, {2, 3}, {4, 7}, {5, -1}}
	for _, c := range cases {
		if got := nthNewline(s, c.n); got != c.want {
			t.Errorf("nthNewline(%d) = %d, want %d", c.n, got, c.want)
		}
	}
	if got := nthNewline("no newline", 1); got != -1 {
		t.Errorf("nthNewline on newline-free string = %d, want -1", got)
	}
}

func TestRenderPreviewTruncatesLongFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "long")
	var src []byte
	for i := 0; i < previewMaxLines*2; i++ {
		src = append(src, "some text on a line\n"...)
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(path)
	p := renderPreview(&file{FileInfo: info, path: path})
	if p.err != nil || p.binary {
		t.Fatalf("unexpected preview state: %+v", p)
	}
	if len(p.lines) != previewMaxLines {
		t.Fatalf("got %d lines, want %d", len(p.lines), previewMaxLines)
	}
	if !p.tooLong {
		t.Error("tooLong not set for truncated file")
	}
}

func TestRenderHexPreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	data := []byte{0x7f, 'E', 'L', 'F', 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 'h', 'i'}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(path)
	p := renderHexPreview(&file{FileInfo: info, path: path})
	if p.err != nil || !p.hex {
		t.Fatalf("unexpected hex preview state: %+v", p)
	}
	if len(p.lines) != 2 {
		t.Fatalf("got %d hex rows, want 2", len(p.lines))
	}
	row0 := p.lines[0][0].text + p.lines[0][1].text + p.lines[0][2].text
	if row0 != "00000000  7f 45 4c 46 00 00 00 00  01 02 03 04 05 06 07 08 |.ELF............|" {
		t.Errorf("row0 = %q", row0)
	}
	row1 := p.lines[1][0].text + p.lines[1][1].text + p.lines[1][2].text
	if row1 != "00000010  68 69                                            |hi|" {
		t.Errorf("row1 = %q", row1)
	}
	if p.tooLong {
		t.Error("tooLong set though whole file was dumped")
	}
}
