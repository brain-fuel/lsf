package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// scrolloff is the number of rows kept visible above/below the cursor.
// Match lf's default; non-zero scrolloff should become a configurable option
// when lsf grows lf-style settings.
const scrolloff = 0

type file struct {
	os.FileInfo
	path    string
	linkDir bool // symlink whose target is a directory
	broken  bool // symlink whose target cannot be stat'ed
}

func (f *file) isDir() bool {
	return f.IsDir() || f.linkDir
}

type dir struct {
	path  string
	all   []*file // every entry, unsorted
	files []*file // visible entries, sorted
	ind   int     // cursor index into files
	off   int     // index of first visible row
	err   error
}

func readdir(path string) *dir {
	d := &dir{path: path}
	entries, err := os.ReadDir(path)
	if err != nil {
		d.err = err
		return d
	}
	for _, e := range entries {
		info, err := os.Lstat(filepath.Join(path, e.Name()))
		if err != nil {
			continue
		}
		f := &file{FileInfo: info, path: filepath.Join(path, e.Name())}
		if info.Mode()&os.ModeSymlink != 0 {
			tinfo, err := os.Stat(f.path)
			if err != nil {
				f.broken = true
			} else if tinfo.IsDir() {
				f.linkDir = true
			}
		}
		d.all = append(d.all, f)
	}
	return d
}

// refresh rebuilds the visible file list from all, preserving the cursor's
// file when possible.
func (d *dir) refresh(showHidden bool) {
	var curr string
	if f := d.curr(); f != nil {
		curr = f.Name()
	}
	d.files = nil
	for _, f := range d.all {
		if !showHidden && strings.HasPrefix(f.Name(), ".") {
			continue
		}
		d.files = append(d.files, f)
	}
	sort.SliceStable(d.files, func(i, j int) bool {
		fi, fj := d.files[i], d.files[j]
		if fi.isDir() != fj.isDir() {
			return fi.isDir()
		}
		return naturalLess(strings.ToLower(fi.Name()), strings.ToLower(fj.Name()))
	})
	if curr != "" {
		d.cursorTo(curr)
	}
}

func (d *dir) curr() *file {
	if d.ind < 0 || d.ind >= len(d.files) {
		return nil
	}
	return d.files[d.ind]
}

func (d *dir) cursorTo(name string) {
	for i, f := range d.files {
		if f.Name() == name {
			d.ind = i
			return
		}
	}
	if d.ind >= len(d.files) {
		d.ind = len(d.files) - 1
	}
}

// bound clamps the cursor and scroll offset for a pane of height h.
func (d *dir) bound(h int) {
	n := len(d.files)
	if n == 0 || h <= 0 {
		d.ind, d.off = 0, 0
		return
	}
	d.ind = max(0, min(d.ind, n-1))
	edge := min(scrolloff, max(0, (h-1)/2))
	if d.ind < d.off+edge {
		d.off = d.ind - edge
	}
	if d.ind > d.off+h-1-edge {
		d.off = d.ind - h + 1 + edge
	}
	d.off = max(0, min(d.off, n-h))
}

type nav struct {
	dirs   map[string]*dir
	path   string
	hidden bool
	search string
}

func newNav(path string) *nav {
	return &nav{dirs: make(map[string]*dir), path: path}
}

func (n *nav) dir(path string) *dir {
	if d, ok := n.dirs[path]; ok {
		return d
	}
	d := readdir(path)
	d.refresh(n.hidden)
	n.dirs[path] = d
	return d
}

func (n *nav) currDir() *dir {
	return n.dir(n.path)
}

func (n *nav) parentDir() *dir {
	parent := filepath.Dir(n.path)
	if parent == n.path {
		return nil
	}
	d := n.dir(parent)
	d.cursorTo(filepath.Base(n.path))
	return d
}

func (n *nav) up(dist int)    { n.currDir().ind -= dist }
func (n *nav) down(dist int)  { n.currDir().ind += dist }
func (n *nav) top()           { n.currDir().ind = 0 }
func (n *nav) bottom()        { n.currDir().ind = len(n.currDir().files) - 1 }
func (n *nav) move(index int) { n.currDir().ind = index }

func (n *nav) scrollUp(dist, h int) {
	d := n.currDir()
	d.bound(h)
	for ; dist > 0; dist-- {
		switch {
		case d.off > 0:
			d.off--
			if d.ind >= d.off+h {
				d.ind = d.off + h - 1
			}
		case d.ind > 0:
			d.ind--
		}
	}
}

func (n *nav) scrollDown(dist, h int) {
	d := n.currDir()
	d.bound(h)
	maxOff := max(0, len(d.files)-h)
	for ; dist > 0; dist-- {
		switch {
		case d.off < maxOff:
			d.off++
			if d.ind < d.off {
				d.ind = d.off
			}
		case d.ind < len(d.files)-1:
			d.ind++
		}
	}
}

func (n *nav) high(h int) {
	d := n.currDir()
	d.bound(h)
	d.ind = d.off
}

func (n *nav) middle(h int) {
	d := n.currDir()
	d.bound(h)
	end := min(d.off+h, len(d.files))
	if end > d.off {
		d.ind = d.off + (end-d.off)/2
	}
}

func (n *nav) low(h int) {
	d := n.currDir()
	d.bound(h)
	end := min(d.off+h, len(d.files))
	if end > d.off {
		d.ind = end - 1
	}
}

func (n *nav) updir() {
	parent := filepath.Dir(n.path)
	if parent == n.path {
		return
	}
	prev := n.path
	n.path = parent
	n.currDir().cursorTo(filepath.Base(prev))
}

// open descends into the file under the cursor if it is a directory and
// reports whether it did.
func (n *nav) open() bool {
	f := n.currDir().curr()
	if f == nil || !f.isDir() {
		return false
	}
	n.path = f.path
	return true
}

func (n *nav) toggleHidden() {
	n.hidden = !n.hidden
	for _, d := range n.dirs {
		d.refresh(n.hidden)
	}
}

func (n *nav) reload() {
	curr := ""
	if d, ok := n.dirs[n.path]; ok {
		if f := d.curr(); f != nil {
			curr = f.Name()
		}
	}
	delete(n.dirs, n.path)
	if curr != "" {
		n.currDir().cursorTo(curr)
	}
}

// searchNext moves the cursor to the next (or previous) file whose name
// contains the current search string, wrapping around. Reports a match.
func (n *nav) searchNext(back bool) bool {
	d := n.currDir()
	if n.search == "" || len(d.files) == 0 {
		return false
	}
	pat := strings.ToLower(n.search)
	step := 1
	if back {
		step = len(d.files) - 1
	}
	for i := 1; i <= len(d.files); i++ {
		j := (d.ind + i*step) % len(d.files)
		if strings.Contains(strings.ToLower(d.files[j].Name()), pat) {
			d.ind = j
			return true
		}
	}
	return false
}

// naturalLess compares strings ordering embedded digit runs numerically,
// so "file2" sorts before "file10".
func naturalLess(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		ad, bd := isDigit(a[0]), isDigit(b[0])
		if ad && bd {
			an, bn := digitRun(a), digitRun(b)
			na, nb := strings.TrimLeft(a[:an], "0"), strings.TrimLeft(b[:bn], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			if an != bn { // equal value, more zero-padding first
				return an > bn
			}
			a, b = a[an:], b[bn:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func digitRun(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}
