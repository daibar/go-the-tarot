package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
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
	return t.viewScroll(body, hints, false)
}

// viewScroll is view, optionally panning sideways as well. Only the tableau
// wants that: everywhere else left and right mean something to the caller.
func (t *term) viewScroll(body, hints string, sideways bool) (string, bool) {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	widest := 0
	for _, l := range lines {
		widest = max(widest, visibleWidth(l))
	}

	top, left := 0, 0
	for {
		shown := lines
		if sideways && left > 0 {
			shown = make([]string, len(lines))
			for i, l := range lines {
				shown[i] = sliceVisible(l, left, t.cols)
			}
		}
		top = t.paint(shown, top, func(top, maxTop int) string {
			keys := viKeys
			if sideways && widest > t.cols {
				keys += " · h/l pan"
			}
			return fmt.Sprintf(" %s · %s · %s", where(top, maxTop), keys, hints)
		})

		key, ok := t.key()
		if !ok {
			return "", false
		}
		if sideways {
			step := max(t.cols/4, 4)
			switch key {
			case keyRight, "l":
				left = min(left+step, max(widest-t.cols, 0))
				continue
			case keyLeft, "h":
				left = max(left-step, 0)
				continue
			}
		}
		next, handled := scroll(key, top, t.rows-1)
		if !handled {
			return key, true
		}
		top = next
	}
}

// sliceVisible takes width visible columns of a line starting at from, keeping
// the colour escapes that go with the characters it keeps.
func sliceVisible(s string, from, width int) string {
	var sb strings.Builder
	col := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			// Copy the whole escape sequence: it costs no columns.
			j := i + 1
			for j < len(s) && !isEscapeEnd(s[j]) {
				j++
			}
			if j < len(s) {
				j++
			}
			sb.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if col >= from && col < from+width {
			sb.WriteRune(r)
		}
		col++
		i += size
	}
	return sb.String()
}

func isEscapeEnd(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
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
