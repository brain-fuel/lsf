package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
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
	d := &dir{}
	for i := 0; i < 100; i++ {
		d.files = append(d.files, &file{})
	}
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

	d.ind = 50
	d.bound(h)
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
