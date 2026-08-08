package main

import (
	"fmt"
	"strings"
)

// outlineRow is one line of the deck outline: either a section heading from the
// guide ("Major Arcana (Trumps)", "Suit of Wands") or a card beneath it.
type outlineRow struct {
	section int    // index of the section this row belongs to
	heading bool   // true for the section heading itself
	label   string // as written in the guide, e.g. "0. The Fool"
	notes   *CardNotes
}

// outline is the deck laid out the way the guide is: sections you can fold,
// cards you can open.
type outline struct {
	rows      []outlineRow
	collapsed []bool
	cursor    int // index into rows
}

func newOutline(g *Guide) *outline {
	o := &outline{collapsed: make([]bool, len(g.Sections))}
	for i, s := range g.Sections {
		o.rows = append(o.rows, outlineRow{section: i, heading: true, label: s.Title})
		for _, c := range s.Cards {
			label := c.Label
			if label == "" {
				label = c.Name
			}
			o.rows = append(o.rows, outlineRow{section: i, label: label, notes: c})
		}
	}
	return o
}

// visible lists the rows currently on show, folded sections excluded.
func (o *outline) visible() []int {
	var out []int
	for i, r := range o.rows {
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
		line := "    " + r.label
		if r.heading {
			marker := "-"
			if o.collapsed[r.section] {
				marker = "+"
			}
			line = fmt.Sprintf("  %s %s", marker, strings.ToUpper(r.label))
		}
		if vis[i] == o.cursor && highlight {
			sb.WriteString("\x1b[7m" + fit(line, cols-1) + "\x1b[0m\n")
			continue
		}
		sb.WriteString(strings.TrimRight(fit(line, cols-1), " ") + "\n")
	}
	return sb.String()
}

// runExplore browses the deck through the guide's outline: arrows to move,
// right or enter to open a card, left to fold a section away.
func runExplore(u *ui) {
	o := newOutline(u.deck.guide)
	o.move(1) // start on the first card rather than its heading
	reversed := false

	for {
		if !u.t.raw {
			// Without a terminal there is nothing to steer, so just list it.
			u.t.print(o.render(len(o.rows), 78, false))
		} else {
			u.t.clear()
			u.t.print(" The deck, as the guide lays it out\n")
			u.t.print(o.render(u.t.rows-3, u.t.cols, true))
			hint := " arrows or j/k move · right opens · left folds · r reverse · q quit"
			if reversed {
				hint = " reversed · " + hint[1:]
			}
			u.t.statusBar(hint)
		}

		key, ok := u.t.key()
		if !ok || key == keyQuit {
			return
		}
		row := o.rows[o.cursor]
		switch key {
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
			if !exploreCard(u, o, reversed) {
				return
			}
		}
	}
}

// exploreCard opens the card under the cursor, and keeps the reader moving
// through the outline with the same keys.
func exploreCard(u *ui, o *outline, reversed bool) bool {
	opts := u.opts
	for {
		row := o.rows[o.cursor]
		if row.heading || row.notes == nil {
			return true
		}
		card, ok := u.deck.cardNamed(row.notes.Name)
		if !ok {
			return true
		}
		p := place(u.deck, u.deck.reversed(card, reversed), nil)
		p.Name = row.label

		key, ok := u.t.view(page(p, opts), "right next · left prev · backspace outline · m mindful · r reverse · q quit")
		if !ok || key == keyQuit {
			return false
		}
		switch key {
		case keyBksp, keyEsc:
			return true
		case keyRight, keyEnter, "n", " ", "l":
			o.moveCard(1)
		case keyLeft, "p", "h":
			o.moveCard(-1)
		case "r":
			reversed = !reversed
		case "m":
			if !u.showMindful(p) {
				return false
			}
		case "w":
			opts.detail = !opts.detail
		}
	}
}
