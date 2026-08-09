package main

import (
	"fmt"
	"strconv"
	"strings"
)

// menuItem is one selectable line: a label and what it means.
type menuItem struct {
	label string
	blurb string
}

// menu draws a highlighted list and returns the index chosen. The arrows and
// the vi keys move, Enter chooses, a number jumps straight to an entry, and q
// backs out — which is what the false second return means.
//
// Without a terminal it prints the list and reads a number, so piped input and
// scripts still work.
func (t *term) menu(title string, items []menuItem, hints string) (int, bool) {
	if len(items) == 0 {
		return 0, false
	}
	at := 0
	for {
		if !t.raw {
			t.print("\n" + title + "\n\n")
			for i, it := range items {
				t.print(fmt.Sprintf("  %d) %-14s %s\n", i+1, it.label, it.blurb))
			}
			t.print("  q) Quit\n")
			choice, ok := t.askKey("\n Choice: ")
			if !ok || choice == keyQuit || choice == keyEsc {
				return 0, false
			}
			if n, err := strconv.Atoi(choice); err == nil && n >= 1 && n <= len(items) {
				return n - 1, true
			}
			t.print(" Pick a number from the list, or q.\n")
			continue
		}

		t.clear()
		t.print("\n " + title + "\n\n")
		for i, it := range items {
			line := fmt.Sprintf("  %d) %-14s %s", i+1, it.label, it.blurb)
			if i == at {
				t.print("\x1b[7m" + fit(line, t.cols-1) + "\x1b[0m\n")
				continue
			}
			t.print(strings.TrimRight(line, " ") + "\n")
		}
		t.print("  q) Quit\n")
		t.statusBar(" " + hints)

		key, ok := t.key()
		if !ok || key == keyQuit || key == keyEsc {
			return 0, false
		}
		switch key {
		case keyDown, "j":
			at = min(at+1, len(items)-1)
		case keyUp, "k":
			at = max(at-1, 0)
		case "g", keyHome:
			at = 0
		case "G", keyEnd:
			at = len(items) - 1
		case keyEnter, keyRight, "l", " ":
			return at, true
		default:
			if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(items) {
				return n - 1, true
			}
		}
	}
}

const menuKeys = "arrows or j/k move · enter chooses · a number jumps · q quits"
