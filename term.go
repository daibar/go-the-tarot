package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Keys are reported as short names. Printable keys report themselves, so a
// switch can read "j", "q", "up" side by side.
const (
	keyEnter  = "enter"
	keyEsc    = "esc"
	keyUp     = "up"
	keyDown   = "down"
	keyLeft   = "left"
	keyRight  = "right"
	keyPgUp   = "pgup"
	keyPgDn   = "pgdn"
	keyHome   = "home"
	keyEnd    = "end"
	keyBksp   = "backspace"
	keyQuit   = "q"
	keyEditor = "editor" // ctrl-e: hand free text to $EDITOR and back
)

// term reads single keypresses from the terminal and draws full screen views.
// When stdin is not a terminal it falls back to reading whole lines, which
// keeps piped input and scripts working.
type term struct {
	in         *bufio.Reader
	raw        bool
	restore    func()
	rows       int
	cols       int
	savedState string // stty -g output, for leaving raw mode exactly the way it was found

	// Keys are pumped through a channel once something needs to wait on a
	// clock, or a resize, as well as the keyboard; see startKeys.
	keys chan string

	// stopKeys and keysDone let a caller reclaim the terminal from the
	// background reader below without leaving it mid-read: closing stopKeys
	// asks the goroutine to give up between polls, and keysDone closes once
	// it actually has. See suspendKeys.
	stopKeys chan struct{}
	keysDone chan struct{}

	// resized carries one pending SIGWINCH at a time: the pane was widened or
	// narrowed since the terminal was last measured, and whatever is on
	// screen wants redrawing at the new size rather than sitting cut off
	// until the next keypress.
	resized chan struct{}
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
	t.savedState = saved
	t.restore = t.suspendRaw
	fmt.Print("\x1b[?25l")
	t.size()
	t.watchResize()
	return t
}

// suspendRaw puts the terminal back the way it was found before raw mode,
// and shows the cursor again — used both to close down for good and, with
// resumeRaw, to hand the terminal to a subprocess (an editor) temporarily.
func (t *term) suspendRaw() {
	stty(t.savedState)
	fmt.Print("\x1b[?25h")
}

// resumeRaw re-enters raw mode after suspendRaw, once a subprocess that
// borrowed the terminal has given it back.
func (t *term) resumeRaw() {
	stty("raw", "-echo")
	fmt.Print("\x1b[?25l")
}

// watchResize notes every SIGWINCH so a wait for a keypress can notice the
// terminal changed shape and hand back control to redraw, without treating
// the resize itself as a keypress.
func (t *term) watchResize() {
	t.resized = make(chan struct{}, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	go func() {
		// A panic here runs on its own goroutine stack, so it would never
		// reach main's recover and would take the whole process down
		// silently — log it and just stop watching resizes instead.
		defer func() {
			if r := recover(); r != nil {
				logCrash(r)
			}
		}()
		for range sig {
			select {
			case t.resized <- struct{}{}:
			default: // a redraw is already pending; one is enough
			}
		}
	}()
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

// startKeys makes sure keypresses are being read on a background goroutine,
// so a wait can give up on the keyboard — for a clock or a resize — without
// losing whatever key comes right after.
func (t *term) startKeys() {
	if t.keys != nil {
		return
	}
	keys := make(chan string, 8)
	stop := make(chan struct{})
	done := make(chan struct{})
	t.keys, t.stopKeys, t.keysDone = keys, stop, done
	go func() {
		defer close(done)
		// A panic here runs on its own goroutine stack, so it would never
		// reach main's recover and would take the whole process down
		// silently, terminal still in raw mode. Log it and close keys
		// instead, so every caller waiting on it sees a plain end of input
		// and winds down the normal way, restoring the terminal as it goes.
		defer func() {
			if r := recover(); r != nil {
				logCrash(r)
				close(keys)
			}
		}()
		for {
			k, ok, stopped := t.readKey(stop)
			if stopped {
				return
			}
			if !ok {
				close(keys)
				return
			}
			select {
			case keys <- k:
			case <-stop:
				return
			}
		}
	}()
}

// suspendKeys stops the background reader above, if one is running, and
// returns a func that starts a fresh one. A subprocess that wants the
// terminal to itself — an editor — needs this: without it, the reader stays
// mid-read and races the subprocess for whatever the reader types next.
func (t *term) suspendKeys() (resume func()) {
	if t.keys == nil {
		return t.startKeys
	}
	close(t.stopKeys)
	<-t.keysDone
	t.keys, t.stopKeys, t.keysDone = nil, nil, nil
	return t.startKeys
}

// key waits for one keypress. The second return is false at end of input.
func (t *term) key() (string, bool) {
	if !t.raw {
		k, ok, _ := t.readKey(nil)
		return k, ok
	}
	t.startKeys()
	for {
		select {
		case k, ok := <-t.keys:
			return k, ok
		case <-t.resized:
			t.size()
		}
	}
}

// keyOrResize is key, but hands back control the moment the terminal reports
// a new size, instead of waiting for an actual keypress. The caller is
// expected to measure whatever it drew against the (already refreshed) rows
// and cols and paint it again, then go back to waiting: a resize is never
// itself a keypress, so it must never reach a switch that treats every
// unrecognised key as "go back" or "not a valid choice".
func (t *term) keyOrResize() (key string, ok bool, resized bool) {
	if !t.raw {
		k, ok, _ := t.readKey(nil)
		return k, ok, false
	}
	t.startKeys()
	select {
	case k, ok := <-t.keys:
		return k, ok, false
	case <-t.resized:
		t.size()
		return "", true, true
	}
}

// keyWithin waits for a keypress, giving up after d. It reports the key, then
// whether input is still open, then whether the wait timed out, then whether
// it gave up early because the terminal was resized (rows and cols are
// already refreshed by the time this returns).
func (t *term) keyWithin(d time.Duration) (key string, ok, timedOut, resized bool) {
	t.startKeys()
	if d <= 0 {
		select {
		case k, ok := <-t.keys:
			return k, ok, false, false
		case <-t.resized:
			t.size()
			return "", true, false, true
		}
	}
	select {
	case k, ok := <-t.keys:
		return k, ok, false, false
	case <-t.resized:
		t.size()
		return "", true, false, true
	case <-time.After(d):
		return "", true, true, false
	}
}

// readKey blocks on the terminal for a single keypress. With stop set, it
// polls instead of blocking outright, so a caller — the background reader in
// startKeys — can give up between polls rather than staying mid-read; the
// third return is true when that happened.
func (t *term) readKey(stop <-chan struct{}) (key string, ok bool, stopped bool) {
	if !t.raw {
		k, ok := t.lineKey()
		return k, ok, false
	}
	b, ok, stopped := t.readByte(stop)
	if stopped || !ok {
		return "", ok, stopped
	}
	switch b {
	case '\r', '\n':
		return keyEnter, true, false
	case 3, 4: // ctrl-C, ctrl-D
		return keyQuit, true, false
	case 2: // ctrl-B
		return keyPgUp, true, false
	case 5: // ctrl-E
		return keyEditor, true, false
	case 6: // ctrl-F
		return keyPgDn, true, false
	case 21: // ctrl-U
		return "u", true, false
	case 127, 8:
		return keyBksp, true, false
	case 0x1b:
		return t.escape(), true, false
	}
	return string(rune(b)), true, false
}

// readByte reads one byte. With stop set, it polls with a short timeout
// instead of blocking outright, checking stop between polls so the wait can
// be abandoned — to hand the terminal to a subprocess — without a read
// staying in flight to race it for keystrokes.
func (t *term) readByte(stop <-chan struct{}) (b byte, ok bool, stopped bool) {
	if stop == nil {
		b, err := t.in.ReadByte()
		return b, err == nil, false
	}
	for {
		if t.in.Buffered() > 0 {
			b, err := t.in.ReadByte()
			return b, err == nil, false
		}
		select {
		case <-stop:
			return 0, false, true
		default:
		}
		ready, err := waitReadable(50 * time.Millisecond)
		if err != nil {
			return 0, false, false
		}
		if ready {
			b, err := t.in.ReadByte()
			return b, err == nil, false
		}
	}
}

// waitReadable reports whether stdin has a byte ready within timeout,
// without consuming it — select(2) on fd 0, so a poll loop can check for
// input in short bursts instead of blocking on a read outright.
func waitReadable(timeout time.Duration) (bool, error) {
	var fds syscall.FdSet
	fds.Bits[0] = 1
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	err := syscall.Select(1, &fds, nil, nil, &tv)
	if err == syscall.EINTR {
		// A signal arrived while select(2) was waiting — our own SIGWINCH
		// handler fires one on every resize — and interrupted it before the
		// timeout. Not ready yet is the correct read: the poll loop just
		// tries again, checking stop in between as usual.
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return fds.Bits[0]&1 != 0, nil
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

// askKey prints a prompt and waits for a single keypress, echoing it. Menus use
// it so that q quits on the key alone, without waiting for Enter.
func (t *term) askKey(prompt string) (string, bool) {
	t.print(prompt)
	if !t.raw {
		line, ok := t.lineKey()
		t.print("\n")
		return line, ok
	}
	k, ok := t.key()
	if !ok {
		return "", false
	}
	if len([]rune(k)) == 1 {
		fmt.Print(k)
	}
	t.print("\n")
	return k, true
}

// promptLine reads a line over the top of the bottom status bar, the way a
// pager takes a search.
func (t *term) promptLine(prompt string) (string, bool) {
	if t.raw {
		fmt.Print("\r\x1b[2K")
	}
	return t.askLine(prompt)
}

// ask prints a prompt and reads a line of text (used outside the full screen
// views, where a keypress is not enough).
func (t *term) askLine(prompt string) (string, bool) {
	t.print(prompt)
	if !t.raw {
		line, ok := t.lineKey()
		if line == keyEnter {
			line = "" // a bare newline is an empty answer, not a keypress
		}
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

// askText is askLine for free text — a query someone typed, say — where
// lowercasing the answer the way askLine does would mangle it. Ctrl-E hands
// the draft to $EDITOR and comes back with whatever was saved there, which
// may run to more than one line; askLine's answers never do, so it has no
// need of this.
func (t *term) askText(prompt string) (string, bool) {
	t.print(prompt)
	if !t.raw {
		line, err := t.in.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil && line == "" {
			return "", false
		}
		t.print("\n")
		return line, true
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
			return strings.TrimSpace(sb.String()), true
		case k == keyEditor:
			edited, changed := t.editText(sb.String())
			if changed {
				sb.Reset()
				sb.WriteString(edited)
			}
			t.print("\n" + prompt + sb.String())
		case k == keyBksp:
			if s := sb.String(); s != "" {
				r := []rune(s)
				last := r[len(r)-1]
				sb.Reset()
				sb.WriteString(string(r[:len(r)-1]))
				if last == '\n' {
					// Backspacing across a line the editor added: reprint
					// what is left rather than try to move the cursor up.
					t.print("\n" + prompt + sb.String())
					continue
				}
				fmt.Print("\b \b")
			}
		case len([]rune(k)) == 1:
			sb.WriteString(k)
			fmt.Print(k)
		}
	}
}

// editText hands seed to the reader's editor — $VISUAL, then $EDITOR, then
// vi — and returns what came back. The terminal is put back in raw mode
// before this returns either way, so the caller's prompt can carry on
// whether or not the editor actually succeeded; the second return is false
// when it didn't, and the caller should keep the seed text unchanged.
func (t *term) editText(seed string) (string, bool) {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}

	f, err := os.CreateTemp("", "tarot-journal-*.txt")
	if err != nil {
		t.print(fmt.Sprintf(" (could not open the editor: %v)\n", err))
		return seed, false
	}
	path := f.Name()
	defer os.Remove(path)
	_, writeErr := f.WriteString(seed)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		t.print(" (could not open the editor: writing the draft failed)\n")
		return seed, false
	}

	resume := t.suspendKeys()
	t.suspendRaw()
	args := append(append([]string{}, parts[1:]...), path)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	runErr := cmd.Run()
	t.resumeRaw()
	resume()

	if runErr != nil {
		t.print(fmt.Sprintf(" (the editor reported a problem: %v)\n", runErr))
		return seed, false
	}
	edited, err := os.ReadFile(path)
	if err != nil {
		t.print(" (could not read the draft back)\n")
		return seed, false
	}
	return strings.TrimRight(string(edited), "\n"), true
}
