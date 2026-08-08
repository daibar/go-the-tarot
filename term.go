package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Keys are reported as short names. Printable keys report themselves, so a
// switch can read "j", "q", "up" side by side.
const (
	keyEnter = "enter"
	keyEsc   = "esc"
	keyUp    = "up"
	keyDown  = "down"
	keyLeft  = "left"
	keyRight = "right"
	keyPgUp  = "pgup"
	keyPgDn  = "pgdn"
	keyHome  = "home"
	keyEnd   = "end"
	keyBksp  = "backspace"
	keyQuit  = "q"
)

// term reads single keypresses from the terminal and draws full screen views.
// When stdin is not a terminal it falls back to reading whole lines, which
// keeps piped input and scripts working.
type term struct {
	in      *bufio.Reader
	raw     bool
	restore func()
	rows    int
	cols    int

	// Keys are pumped through a channel once something needs to wait on a
	// clock as well as the keyboard; see keyWithin.
	keys chan string
}

func newTerm() *term {
	t := &term{in: bufio.NewReader(os.Stdin), rows: 24, cols: 80}
	if !isTTY() {
		return t
	}
	// stty keeps this dependency free; x/term would be a module download.
	saved, err := stty("-g")
	if err != nil {
		return t
	}
	if _, err := stty("raw", "-echo"); err != nil {
		return t
	}
	t.raw = true
	t.restore = func() {
		stty(saved)
		fmt.Print("\x1b[?25h") // show the cursor again
	}
	fmt.Print("\x1b[?25l")
	t.size()
	return t
}

func (t *term) close() {
	if t.restore != nil {
		t.restore()
		t.restore = nil
		t.raw = false
	}
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func stty(args ...string) (string, error) {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// size asks the terminal how big it is, falling back to 24x80.
func (t *term) size() {
	out, err := stty("size")
	if err != nil {
		return
	}
	if parts := strings.Fields(out); len(parts) == 2 {
		if rows, err := strconv.Atoi(parts[0]); err == nil && rows > 4 {
			t.rows = rows
		}
		if cols, err := strconv.Atoi(parts[1]); err == nil && cols > 20 {
			t.cols = cols
		}
	}
}

// key waits for one keypress. The second return is false at end of input.
func (t *term) key() (string, bool) {
	if t.keys != nil {
		k, ok := <-t.keys
		return k, ok
	}
	return t.readKey()
}

// keyWithin waits for a keypress, giving up after d. It reports the key, then
// whether input is still open, then whether the wait timed out.
func (t *term) keyWithin(d time.Duration) (string, bool, bool) {
	if t.keys == nil {
		// From here on every read comes off this goroutine, so that a pending
		// read can be abandoned when the clock runs out.
		t.keys = make(chan string, 8)
		go func() {
			for {
				k, ok := t.readKey()
				if !ok {
					close(t.keys)
					return
				}
				t.keys <- k
			}
		}()
	}
	if d <= 0 {
		k, ok := <-t.keys
		return k, ok, false
	}
	select {
	case k, ok := <-t.keys:
		return k, ok, false
	case <-time.After(d):
		return "", true, true
	}
}

// readKey blocks on the terminal for a single keypress.
func (t *term) readKey() (string, bool) {
	if !t.raw {
		return t.lineKey()
	}
	b, err := t.in.ReadByte()
	if err != nil {
		return "", false
	}
	switch b {
	case '\r', '\n':
		return keyEnter, true
	case 3, 4: // ctrl-C, ctrl-D
		return keyQuit, true
	case 2: // ctrl-B
		return keyPgUp, true
	case 6: // ctrl-F
		return keyPgDn, true
	case 21: // ctrl-U
		return "u", true
	case 127, 8:
		return keyBksp, true
	case 0x1b:
		return t.escape(), true
	}
	return string(rune(b)), true
}

// escape decodes an arrow key or similar. A lone Esc has nothing buffered
// behind it, which is how it is told apart from a sequence.
func (t *term) escape() string {
	if t.in.Buffered() == 0 {
		return keyEsc
	}
	b, err := t.in.ReadByte()
	if err != nil || (b != '[' && b != 'O') {
		return keyEsc
	}
	seq, err := t.in.ReadByte()
	if err != nil {
		return keyEsc
	}
	switch seq {
	case 'A':
		return keyUp
	case 'B':
		return keyDown
	case 'C':
		return keyRight
	case 'D':
		return keyLeft
	case 'H':
		return keyHome
	case 'F':
		return keyEnd
	}
	// Numeric sequences such as "5~" for page up.
	if seq >= '0' && seq <= '9' {
		digits := string(rune(seq))
		for {
			b, err := t.in.ReadByte()
			if err != nil || b == '~' {
				break
			}
			digits += string(rune(b))
		}
		switch digits {
		case "5":
			return keyPgUp
		case "6":
			return keyPgDn
		case "1", "7":
			return keyHome
		case "4", "8":
			return keyEnd
		}
	}
	return keyEsc
}

// lineKey is the fallback for piped input: read a line and treat it as a key,
// so scripts can send "j", "down", or a bare newline.
func (t *term) lineKey() (string, bool) {
	line, err := t.in.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if err != nil && line == "" {
		return "", false
	}
	if line == "" {
		return keyEnter, true
	}
	return line, true
}

// print writes text, translating line ends for raw mode.
func (t *term) print(s string) {
	if t.raw {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	fmt.Print(s)
}

func (t *term) clear() {
	if t.raw {
		fmt.Print("\x1b[H\x1b[2J")
	}
}

// ask prints a prompt and reads a line of text (used outside the full screen
// views, where a keypress is not enough).
func (t *term) askLine(prompt string) (string, bool) {
	t.print(prompt)
	if !t.raw {
		line, ok := t.lineKey()
		t.print("\n") // nothing echoes a piped line back for us
		return line, ok
	}
	var sb strings.Builder
	for {
		k, ok := t.key()
		if !ok {
			return "", false
		}
		switch {
		case k == keyEnter:
			t.print("\n")
			return strings.ToLower(strings.TrimSpace(sb.String())), true
		case k == keyBksp:
			if s := sb.String(); s != "" {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
				fmt.Print("\b \b")
			}
		case len([]rune(k)) == 1: // a printable key types itself
			sb.WriteString(k)
			fmt.Print(k)
		}
	}
}
