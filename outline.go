package main

import (
	"strings"
)

// outlineRow is one line of the deck outline: either a section heading from
// the guide ("Major Arcana (Trumps)", "Suit of Wands"), the synthetic Minor
// Arcana heading that groups the four suits, or a card beneath one of them.
type outlineRow struct {
	section int    // this row's own fold group, an index into outline.collapsed
	parent  int    // -1, or the fold group of an ancestor heading (Minor Arcana)
	heading bool   // true for a section heading itself
	label   string // as written in the guide, e.g. "0. The Fool"
	notes   *CardNotes
	minor   bool // a suit card (pip or court), indented deeper than the majors
}

// outline is the deck laid out the way the guide is: sections you can fold,
// cards you can open. The guide files the Major Arcana and every suit as
// sibling sections; the outline nests the suits under a synthetic Minor
// Arcana heading instead, one level flatter than the deck's own three-tier
// arrangement of Trumps and Pips-by-suit.
type outline struct {
	rows      []outlineRow
	collapsed []bool
	cursor    int // index into rows
}

func newOutline(g *Guide) *outline {
	o := &outline{collapsed: make([]bool, len(g.Sections)+1)}
	minorGroup := -1 // the extra collapsed slot, one past every real section
	for i, s := range g.Sections {
		isSuit := strings.HasPrefix(s.Title, "Suit of ")
		parent := -1
		if isSuit {
			if minorGroup < 0 {
				minorGroup = len(g.Sections)
				o.rows = append(o.rows, outlineRow{section: minorGroup, parent: -1, heading: true, label: "Minor Arcana (Pips & Courts)"})
			}
			parent = minorGroup
		}
		o.rows = append(o.rows, outlineRow{section: i, parent: parent, heading: true, label: s.Title})
		for _, c := range s.Cards {
			label := c.Label
			if label == "" {
				label = c.Name
			}
			o.rows = append(o.rows, outlineRow{section: i, parent: parent, label: label, notes: c, minor: isSuit})
		}
	}
	return o
}

// visible lists the rows currently on show, folded sections excluded — along
// with anything nested under a folded ancestor, such as every suit once the
// Minor Arcana heading over them is folded.
func (o *outline) visible() []int {
	var out []int
	for i, r := range o.rows {
		if r.parent >= 0 && o.collapsed[r.parent] {
			continue
		}
		if r.heading || !o.collapsed[r.section] {
			out = append(out, i)
		}
	}
	return out
}

// move walks the cursor by delta rows, skipping anything folded away.
func (o *outline) move(delta int) {
	vis := o.visible()
	at := 0
	for i, row := range vis {
		if row == o.cursor {
			at = i
			break
		}
	}
	at = min(max(at+delta, 0), len(vis)-1)
	o.cursor = vis[at]
}

// moveCard walks to the next or previous card, stepping over section headings.
// At either end of the deck it stays where it is rather than landing on a
// heading with no card beyond it.
func (o *outline) moveCard(delta int) {
	start := o.cursor
	for {
		before := o.cursor
		o.move(delta)
		if o.cursor == before { // clamped at the top or the bottom
			o.cursor = start
			return
		}
		if !o.rows[o.cursor].heading {
			return
		}
	}
}

// find looks for the next card whose name matches q, searching from the row
// after from in direction dir and wrapping around. It returns -1 for no match.
func (o *outline) find(q string, from, dir int) int {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return -1
	}
	for n := 1; n <= len(o.rows); n++ {
		i := ((from+dir*n)%len(o.rows) + len(o.rows)) % len(o.rows)
		r := o.rows[i]
		if r.heading {
			continue
		}
		if strings.Contains(strings.ToLower(r.label), q) {
			return i
		}
	}
	return -1
}

// jump puts the cursor on a row, unfolding whatever section — and, for a
// suit, whatever Minor Arcana heading over it — hides it.
func (o *outline) jump(i int) {
	if i < 0 || i >= len(o.rows) {
		return
	}
	r := o.rows[i]
	o.collapsed[r.section] = false
	if r.parent >= 0 {
		o.collapsed[r.parent] = false
	}
	o.cursor = i
}

// indexOf finds the row for a card by its exact name, or -1 if there is none.
func (o *outline) indexOf(name string) int {
	for i, r := range o.rows {
		if r.notes != nil && r.notes.Name == name {
			return i
		}
	}
	return -1
}

// render draws the outline into height rows, scrolled to keep the cursor shown.
// highlight is off when there is no terminal to steer, so the plain listing
// does not carry stray escape codes.
func (o *outline) render(height, cols int, highlight bool) string {
	vis := o.visible()
	at := 0
	for i, row := range vis {
		if row == o.cursor {
			at = i
			break
		}
	}
	top := min(max(at-height/2, 0), max(len(vis)-height, 0))

	var sb strings.Builder
	for i := top; i < len(vis) && i-top < height; i++ {
		r := o.rows[vis[i]]
		indent := "    "
		switch {
		case r.heading && r.parent < 0:
			indent = "  "
		case r.heading:
			indent = "    " // a suit heading, nested one level under Minor Arcana
		case r.minor:
			indent = "      " // every suit card, pip or court, grouped a step deeper still
		}
		line := indent + r.label
		if vis[i] == o.cursor && highlight {
			sb.WriteString("\x1b[7m" + fit(line, cols-1) + "\x1b[0m\n")
			continue
		}
		sb.WriteString(strings.TrimRight(fit(line, cols-1), " ") + "\n")
	}
	return sb.String()
}

// runExplore browses the deck through the guide's outline: arrows to move,
// right or enter to open a card, left to fold a section away, / to search.
func runExplore(u *ui) {
	o := newOutline(u.deck.guide)
	o.move(1) // start on the first card rather than its heading
	exploreLoop(u, o, false)
}

// exploreFrom opens explore mode straight onto the card being previewed, for a
// look taken mid-reading. Backspace jumps straight back to the card that
// opened it, from the card view or the outline alike, leaving the reading
// exactly as it was; q still reaches past that for the main menu, the same as
// everywhere else. o is the only way to reach the outline itself.
func (u *ui) exploreFrom(p Position) bool {
	o := newOutline(u.deck.guide)
	o.jump(o.indexOf(p.Card.Name))
	toReading, ok := exploreCard(u, o, p.Card.Reversed, true)
	if !ok {
		return false
	}
	if toReading {
		return true
	}
	return exploreLoop(u, o, true)
}

// exploreLoop is the outline browser shared by runExplore and exploreFrom.
// embedded marks a look taken from inside a reading: backspace there backs out
// to whatever opened it, and reaching for q still bubbles all the way up, the
// same as anywhere else in the program.
func exploreLoop(u *ui, o *outline, embedded bool) bool {
	reversed := false
	query, notice := "", ""

	for {
		if !u.t.raw {
			// Without a terminal there is nothing to steer, so just list it.
			u.t.print(o.render(len(o.rows), 78, false))
		} else {
			hint := " arrows move · right opens · left folds · / search · r reverse"
			if embedded {
				hint += " · backspace return to the reading"
			}
			hint += " · q quit"
			switch {
			case notice != "":
				hint = " " + notice + " ·" + hint
			case reversed:
				hint = " reversed ·" + hint
			}
			paintOutline(u, o, hint)
		}

		key, ok := u.t.key()
		if !ok || key == keyQuit {
			return false
		}
		if embedded && key == keyBksp {
			return true
		}
		row := o.rows[o.cursor]
		notice = "" // the last message only lasts until the next keypress
		switch key {
		case "/":
			q, foundIt, canceled, ok := searchOutline(u, o)
			if !ok {
				return false
			}
			if !canceled {
				query = q
				if query != "" && !foundIt {
					notice = "no card matching " + query
				}
			}
		case "n", "N":
			if query == "" {
				notice = "nothing searched for yet"
				break
			}
			dir := 1
			if key == "N" {
				dir = -1
			}
			if i := o.find(query, o.cursor, dir); i >= 0 {
				o.jump(i)
			} else {
				notice = "no card matching " + query
			}
		case keyDown, "j":
			o.move(1)
		case keyUp, "k":
			o.move(-1)
		case keyPgDn, "f":
			o.move(10)
		case keyPgUp, "b":
			o.move(-10)
		case "g", keyHome:
			o.cursor = 0
			o.move(1)
		case "G", keyEnd:
			o.cursor = len(o.rows) - 1
		case "r":
			reversed = !reversed
		case keyLeft, "h":
			// Fold the section, or step up to its heading first.
			if !row.heading {
				for i := o.cursor; i >= 0; i-- {
					if o.rows[i].heading {
						o.cursor = i
						break
					}
				}
				continue
			}
			o.collapsed[row.section] = true
		case keyRight, "l", keyEnter, " ":
			if row.heading {
				if o.collapsed[row.section] {
					o.collapsed[row.section] = false
					continue
				}
				o.move(1)
				continue
			}
			toReading, ok := exploreCard(u, o, reversed, embedded)
			if !ok {
				return false
			}
			if toReading {
				return true
			}
		}
	}
}

// paintOutline draws the outline full screen with a status line under it —
// shared by exploreLoop's own draw and the incremental search it runs.
func paintOutline(u *ui, o *outline, status string) {
	u.t.clear()
	u.t.print(" The deck, as the guide lays it out\n")
	u.t.print(o.render(u.t.rows-3, u.t.cols, true))
	u.t.statusBar(status)
}

// searchOutline is an incremental "/" search: the outline jumps to the first
// match after every keystroke instead of waiting for Enter, the way most
// pagers and editors do it. Esc cancels back to wherever the cursor started,
// leaving the last confirmed search term alone for n and N to repeat; Enter
// keeps wherever the search landed, match or not. ok is false when input ran
// out, the same signal as everywhere else in the program.
func searchOutline(u *ui, o *outline) (query string, found, canceled, ok bool) {
	start := o.cursor
	if !u.t.raw {
		// No live typing without a terminal to redraw for: take the whole
		// line at once, the way every other prompt in the program does.
		q, ok := u.t.promptLine(" search: ")
		if !ok {
			return "", false, false, false
		}
		if i := o.find(q, start, 1); i >= 0 {
			o.jump(i)
			return q, true, false, true
		}
		return q, false, false, true
	}

	var sb strings.Builder
	for {
		q := sb.String()
		i := o.find(q, start, 1)
		if i >= 0 {
			o.jump(i)
		} else {
			o.cursor = start
		}

		status := " search: " + q
		if q != "" && i < 0 {
			status += " (no match)"
		}
		paintOutline(u, o, status)

		key, ok := u.t.key()
		if !ok {
			return "", false, false, false
		}
		switch {
		case key == keyEnter:
			return q, i >= 0, false, true
		case key == keyEsc:
			o.cursor = start
			return "", false, true, true
		case key == keyBksp:
			if s := sb.String(); s != "" {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
			}
		case len([]rune(key)) == 1:
			sb.WriteString(key)
		}
	}
}

// exploreCard opens the card under the cursor, and keeps the reader moving
// through the outline with the same keys. o and esc step back to the outline;
// when embedded, backspace instead jumps straight past the outline, back to
// the reading that opened this look. ok is false on q, the same signal as
// everywhere else in the program.
func exploreCard(u *ui, o *outline, reversed, embedded bool) (toReading, ok bool) {
	opts := u.opts
	hint := "right next · left prev · o outline · m mindful · r reverse · q quit"
	if embedded {
		hint = "right next · left prev · o outline · backspace return to the reading · m mindful · r reverse · q quit"
	}
	for {
		row := o.rows[o.cursor]
		if row.heading || row.notes == nil {
			return false, true
		}
		card, cardOK := u.deck.cardNamed(row.notes.Name)
		if !cardOK {
			return false, true
		}
		p := place(u.deck, u.deck.reversed(card, reversed), nil)

		key, keyOK := u.t.view(page(p, opts), hint)
		if !keyOK || key == keyQuit {
			return false, false
		}
		switch key {
		case "o", keyEsc:
			return false, true
		case keyBksp:
			if embedded {
				return true, true
			}
		case keyRight, keyEnter, "n", " ", "l":
			o.moveCard(1)
		case keyLeft, "p", "h":
			o.moveCard(-1)
		case "r":
			reversed = !reversed
		case "m":
			if !u.showMindful(p) {
				return false, false
			}
		case "w":
			opts.detail = !opts.detail
		}
	}
}
