package main

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v3"
)

var keyNames = map[tcell.Key]string{
	tcell.KeyEnter:     "<enter>",
	tcell.KeyBackspace: "<backspace>",
	tcell.KeyTab:       "<tab>",
	tcell.KeyEsc:       "<esc>",
	tcell.KeyDelete:    "<delete>",
	tcell.KeyUp:        "<up>",
	tcell.KeyDown:      "<down>",
	tcell.KeyLeft:      "<left>",
	tcell.KeyRight:     "<right>",
	tcell.KeyHome:      "<home>",
	tcell.KeyEnd:       "<end>",
	tcell.KeyPgUp:      "<pgup>",
	tcell.KeyPgDn:      "<pgdn>",
	tcell.KeyCtrlA:     "<c-a>",
	tcell.KeyCtrlB:     "<c-b>",
	tcell.KeyCtrlC:     "<c-c>",
	tcell.KeyCtrlD:     "<c-d>",
	tcell.KeyCtrlE:     "<c-e>",
	tcell.KeyCtrlF:     "<c-f>",
	tcell.KeyCtrlG:     "<c-g>",
	tcell.KeyCtrlH:     "<c-h>",
	tcell.KeyCtrlI:     "<c-i>",
	tcell.KeyCtrlJ:     "<c-j>",
	tcell.KeyCtrlK:     "<c-k>",
	tcell.KeyCtrlL:     "<c-l>",
	tcell.KeyCtrlM:     "<c-m>",
	tcell.KeyCtrlN:     "<c-n>",
	tcell.KeyCtrlO:     "<c-o>",
	tcell.KeyCtrlP:     "<c-p>",
	tcell.KeyCtrlQ:     "<c-q>",
	tcell.KeyCtrlR:     "<c-r>",
	tcell.KeyCtrlS:     "<c-s>",
	tcell.KeyCtrlT:     "<c-t>",
	tcell.KeyCtrlU:     "<c-u>",
	tcell.KeyCtrlV:     "<c-v>",
	tcell.KeyCtrlW:     "<c-w>",
	tcell.KeyCtrlX:     "<c-x>",
	tcell.KeyCtrlY:     "<c-y>",
	tcell.KeyCtrlZ:     "<c-z>",
}

var normalBindings = map[string]string{
	"q":       "quit",
	"<esc>":   "quit",
	"<c-c>":   "quit",
	"k":       "up",
	"<up>":    "up",
	"j":       "down",
	"<down>":  "down",
	"h":       "updir",
	"<left>":  "updir",
	"l":       "open",
	"<right>": "open",
	"<enter>": "open",
	"gg":      "top",
	"<home>":  "top",
	"G":       "bottom",
	"<end>":   "bottom",
	"H":       "high",
	"M":       "middle",
	"L":       "low",
	"<c-u>":   "half-up",
	"<c-d>":   "half-down",
	"<c-b>":   "page-up",
	"<pgup>":  "page-up",
	"<c-f>":   "page-down",
	"<pgdn>":  "page-down",
	"<c-y>":   "scroll-up",
	"<c-e>":   "scroll-down",
	"zh":      "toggle-hidden",
	".":       "toggle-hidden",
	"r":       "reload",
	"<c-r>":   "reload",
	"e":       "edit",
	"v":       "view",
	"x":       "hex-peek",
	"/":       "search",
	"n":       "search-next",
	"N":       "search-prev",
}

type keyResolver struct {
	acc   string
	count string
}

type keyAction struct {
	command string
	count   int
}

func (r *keyResolver) accept(key string, bindings map[string]string) (keyAction, bool) {
	if key == "<esc>" && r.acc != "" {
		r.reset()
		return keyAction{}, false
	}
	if isCountKey(key) && r.acc == "" {
		r.count += key
		return keyAction{}, false
	}

	r.acc += key
	command, exact, partial := findBinding(bindings, r.acc)
	if !partial {
		r.reset()
		return keyAction{}, false
	}
	if !exact {
		return keyAction{}, false
	}

	count := 1
	if r.count != "" {
		if n, err := strconv.Atoi(r.count); err == nil && n > 0 {
			count = n
		}
	}
	r.reset()
	return keyAction{command: command, count: count}, true
}

func (r *keyResolver) reset() {
	r.acc = ""
	r.count = ""
}

func findBinding(bindings map[string]string, prefix string) (command string, exact, partial bool) {
	for keys, cmd := range bindings {
		if !strings.HasPrefix(keys, prefix) {
			continue
		}
		partial = true
		if keys == prefix {
			command = cmd
			exact = true
		}
	}
	return command, exact, partial
}

func isCountKey(key string) bool {
	return len(key) == 1 && key[0] >= '0' && key[0] <= '9'
}

func readKey(ev *tcell.EventKey) string {
	var key string
	if ev.Key() == tcell.KeyRune {
		switch ev.Str() {
		case "<":
			key = "<lt>"
		case ">":
			key = "<gt>"
		case " ":
			key = "<space>"
		default:
			key = ev.Str()
		}
	} else {
		key = keyNames[ev.Key()]
	}
	if key == "" {
		return ""
	}
	return addKeyModifier(key, ev.Modifiers())
}

func addKeyModifier(key string, mod tcell.ModMask) string {
	if strings.HasPrefix(key, "<c-") || strings.HasPrefix(key, "<s-") || strings.HasPrefix(key, "<a-") {
		return key
	}
	switch {
	case mod&tcell.ModCtrl != 0:
		return wrapKeyModifier(strings.ToLower(key), "c")
	case mod&tcell.ModShift != 0 && strings.HasPrefix(key, "<"):
		return wrapKeyModifier(key, "s")
	case mod&tcell.ModAlt != 0:
		return wrapKeyModifier(key, "a")
	default:
		return key
	}
}

func wrapKeyModifier(key, mod string) string {
	key = strings.TrimPrefix(key, "<")
	key = strings.TrimSuffix(key, ">")
	return "<" + mod + "-" + key + ">"
}
