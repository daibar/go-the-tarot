package main

import (
	"fmt"
	"strings"
)

// viKeys is the scrolling half of every view's help line.
const viKeys = "j/k scroll · d/u half · g/G ends"

// paint draws one screenful of lines starting at top, plus a status bar, and
// returns the offset it actually used. status is given the clamped offset and
// the furthest the text can scroll.
func (t *term) paint(lines []string, top int, status func(top, maxTop int) string) int {
	if !t.raw {
		t.print(strings.Join(lines, "\n") + "\n")
		return 0
	}
	height := t.rows - 1 // leave a row for the status line
	maxTop := max(len(lines)-height, 0)
	top = min(max(top, 0), maxTop)

	t.clear()
	end := min(top+height, len(lines))
	t.print(strings.Join(lines[top:end], "\n"))
	t.print(strings.Repeat("\n", max(height-(end-top), 0)))
	t.statusBar(status(top, maxTop))
	return top
}

// scroll applies a vi movement key, reporting whether the key was one.
func scroll(key string, top, height int) (int, bool) {
	switch key {
	case "j", keyDown:
		return top + 1, true
	case "k", keyUp:
		return top - 1, true
	case "d":
		return top + height/2, true
	case "u":
		return top - height/2, true
	case " ", "f", keyPgDn:
		return top + height, true
	case "b", keyPgUp:
		return top - height, true
	case "g", keyHome:
		return 0, true
	case "G", keyEnd:
		return 1 << 30, true // clamped to the bottom by paint
	}
	return top, false
}

// view shows text with vi-key scrolling and returns the first key it does not
// handle itself. The second return is false at end of input.
//
// Without a terminal it simply prints everything and reads one key, so piped
// input and redirected output still behave.
func (t *term) view(body, hints string) (string, bool) {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	top := 0
	for {
		top = t.paint(lines, top, func(top, maxTop int) string {
			return fmt.Sprintf(" %s · %s · %s", where(top, maxTop), viKeys, hints)
		})
		key, ok := t.key()
		if !ok {
			return "", false
		}
		next, handled := scroll(key, top, t.rows-1)
		if !handled {
			return key, true
		}
		top = next
	}
}

// where describes the scroll position for the status bar.
func where(top, maxTop int) string {
	switch {
	case maxTop == 0:
		return "all"
	case top == 0:
		return "top"
	case top >= maxTop:
		return "end"
	}
	return fmt.Sprintf("%d%%", top*100/maxTop)
}

// statusBar draws one reverse-video line across the bottom of the screen.
func (t *term) statusBar(line string) {
	fmt.Printf("\r\n\x1b[7m%s\x1b[0m", fit(line, t.cols))
}

// fit cuts or pads a line to exactly width columns, counting runes rather than
// bytes so the separators do not get sliced in half.
func fit(s string, width int) string {
	r := []rune(s)
	if len(r) > width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}
