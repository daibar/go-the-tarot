package main

import (
	"fmt"
	"strings"
	"time"
)

// defaultDwell is how long a card stays up in the carousel before the next one
// turns over.
const defaultDwell = 60 * time.Second

// dwellStep is how much the + and - keys change the dwell by.
const dwellStep = 15 * time.Second

const minDwell = 2 * time.Second

// runCarousel turns the cards over on its own: in order through the deck when
// ordered is set, otherwise drawn at random from the pile. The reader can still
// steer — pause, step, scroll, or open the mindful reading — and the timer picks
// up again from wherever they leave it.
func runCarousel(u *ui, ordered bool) {
	o := newOutline(u.deck.guide)
	o.moveCard(1)
	reversed := false

	// Cards already turned over, so that stepping back works in both sources.
	var shown []Position
	at := 0

	turn := func() Position {
		if !ordered {
			card, ok := u.pile.Draw()
			if !ok {
				return Position{}
			}
			p := place(u.deck, card, nil)
			p.Name = fmt.Sprintf("Draw %d", len(shown)+1)
			return p
		}
		if len(shown) > 0 {
			o.moveCard(1)
		}
		row := o.rows[o.cursor]
		card, ok := u.deck.cardNamed(row.notes.Name)
		if !ok {
			return Position{}
		}
		p := place(u.deck, u.deck.reversed(card, reversed), nil)
		p.Name = row.label
		return p
	}

	dwell := u.opts.dwell
	if dwell <= 0 {
		dwell = defaultDwell
	}
	opts := u.opts
	paused := false
	shown = append(shown, turn())
	deadline := time.Now().Add(dwell)
	top := 0

	advance := func() {
		at++
		if at == len(shown) {
			shown = append(shown, turn())
		}
		top, deadline = 0, time.Now().Add(dwell)
	}

	for {
		p := shown[at]
		if p.Notes == nil { // the source ran out
			return
		}
		top = u.t.paint(pageLines(p, opts), top, func(top, maxTop int) string {
			return carouselStatus(at+1, len(shown), dwell, paused, reversed, ordered, top, maxTop)
		})

		wait := time.Until(deadline)
		if !paused && wait <= 0 {
			advance()
			continue
		}
		if paused {
			wait = 0 // wait on the keyboard alone
		}
		key, ok, timedOut := u.t.keyWithin(wait)
		if !ok {
			return
		}
		if timedOut {
			advance()
			continue
		}

		switch key {
		case keyQuit:
			return
		case " ", "P":
			paused = !paused
			if !paused {
				deadline = time.Now().Add(dwell)
			}
		case keyRight, keyEnter, "n":
			advance()
		case keyLeft, "p":
			if at > 0 {
				at--
				top, deadline = 0, time.Now().Add(dwell)
			}
		case "m":
			if !u.showMindful(p) {
				return
			}
			deadline = time.Now().Add(dwell) // the essay does not eat the clock
		case "w":
			opts.detail = !opts.detail
		case "r":
			if ordered {
				reversed = !reversed
				shown[at] = place(u.deck, u.deck.reversed(p.Card, reversed), nil)
				shown[at].Name = p.Name
			}
		case "+", "=":
			dwell += dwellStep
			deadline = time.Now().Add(dwell)
		case "-", "_":
			dwell = max(dwell-dwellStep, minDwell)
			deadline = time.Now().Add(dwell)
		default:
			if next, handled := scroll(key, top, u.t.rows-1); handled {
				top = next
			}
		}
	}
}

// carouselStatus is the bottom bar: where you are, how fast it is going, and
// what the keys do.
func carouselStatus(n, total int, dwell time.Duration, paused, reversed, ordered bool, top, maxTop int) string {
	pace := fmt.Sprintf("%ds", int(dwell.Seconds()))
	if paused {
		pace = "paused"
	}
	var extra string
	if reversed && ordered {
		extra = " · reversed"
	}
	return fmt.Sprintf(" %d/%d · %s%s · %s · space pause · +/- pace · m mindful · q quit",
		n, total, pace, extra, where(top, maxTop))
}

// pageLines is a card's page ready for the pager.
func pageLines(p Position, opts options) []string {
	return strings.Split(strings.TrimRight(page(p, opts), "\n"), "\n")
}
