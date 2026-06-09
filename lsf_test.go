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
