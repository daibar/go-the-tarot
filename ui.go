package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// mode is how the deck is consulted.
type mode string

const (
	modeCeltic   mode = "celtic"
	modeThree    mode = "three"
	modeFreeform mode = "freeform"
	modeExplore  mode = "explore"
	modeCarousel mode = "carousel"
)

var modeMenu = []struct {
	mode  mode
	title string
	blurb string
}{
	{modeCeltic, "Celtic cross", "ten cards, the full spread"},
	{modeThree, "Three card", "past, present, future"},
	{modeFreeform, "Free form", "keep drawing from the pile, as many as you like"},
	{modeExplore, "Explore", "walk the deck in order, no shuffling"},
	{modeCarousel, "Carousel", "cards turn over on their own, on a timer"},
}

// plan is what the reader settled on: which mode, and for the carousel, how it
// draws and how long each card stays up.
type plan struct {
	mode    mode
	ordered bool
	dwell   time.Duration
}

// ui carries everything the interactive loops need.
type ui struct {
	t     *term
	deck  *Deck
	pile  *Pile
	opts  options
	query string
}

// chooseMode shows the mode picker. It returns false if the user quits.
func chooseMode(u *ui) (plan, bool) {
	for {
		u.t.clear()
		u.t.print("\n How would you like to read?\n\n")
		for i, m := range modeMenu {
			u.t.print(fmt.Sprintf("  %d) %-14s %s\n", i+1, m.title, m.blurb))
		}
		u.t.print("  q) Quit\n")

		choice, ok := u.t.askLine("\n Choice: ")
		if !ok || choice == keyQuit {
			return plan{}, false
		}
		n, err := strconv.Atoi(choice)
		if err != nil || n < 1 || n > len(modeMenu) {
			u.t.print(" Pick a number from the list, or q.\n")
			continue
		}
		m := modeMenu[n-1].mode
		if m != modeCarousel {
			return plan{mode: m}, true
		}
		return chooseCarousel(u)
	}
}

// chooseCarousel asks how the carousel should run: where the cards come from,
// and how many seconds each one stays up.
func chooseCarousel(u *ui) (plan, bool) {
	p := plan{mode: modeCarousel, ordered: true, dwell: u.opts.dwell}
	if p.dwell <= 0 {
		p.dwell = defaultDwell
	}

	u.t.print("\n Where should the cards come from?\n\n")
	u.t.print("  1) In order through the deck\n")
	u.t.print("  2) Drawn at random from the pile\n")
	choice, ok := u.t.askLine("\n Choice [1]: ")
	if !ok || choice == keyQuit {
		return plan{}, false
	}
	p.ordered = choice != "2"

	for {
		secs, ok := u.t.askLine(fmt.Sprintf(" Seconds per card [%d]: ", int(p.dwell.Seconds())))
		if !ok || secs == keyQuit {
			return plan{}, false
		}
		if secs == "" {
			return p, true
		}
		n, err := strconv.Atoi(secs)
		if err != nil || n < 1 {
			u.t.print(" Give a whole number of seconds.\n")
			continue
		}
		p.dwell = time.Duration(n) * time.Second
		return p, true
	}
}

// showCard puts a card up and handles the keys available while it is there. It
// returns false when the reader wants out.
func (u *ui) showCard(p Position, hints string) bool {
	opts := u.opts
	for {
		key, ok := u.t.view(page(p, opts), hints)
		if !ok || key == keyQuit {
			return false
		}
		switch key {
		case keyEnter, keyRight, "n":
			return true
		case "m":
			if !u.showMindful(p) {
				return false
			}
		case "w":
			opts.detail = !opts.detail
		}
	}
}

// showMindful puts the contemplative essay up in its own scrollable view.
func (u *ui) showMindful(p Position) bool {
	key, ok := u.t.view(mindful(p), "esc/enter back to the card")
	return ok && key != keyQuit
}

// runSpread deals a fixed layout, walks through it, then offers the review menu.
func runSpread(u *ui, s *Spread) *Reading {
	progress := func(n int) { u.t.print(fmt.Sprintf("Drawing card number %d\n", n)) }
	if u.opts.quiet {
		progress = nil
	}
	r := newReading(u.deck, u.pile, s, u.query, progress)

	if !u.opts.quiet {
		u.t.print("Beginning reading\n")
	}
	u.t.print("\n" + header(r))
	for _, p := range r.Positions {
		if !u.showCard(p, "enter next · m mindful · w Waite · q quit") {
			return r
		}
	}
	review(u, r)
	return r
}

// review is the post-reading menu, in place of the bash version's fzf picker.
func review(u *ui, r *Reading) {
	for {
		u.t.clear()
		u.t.print("\n Review your reading.\n")
		if r.Query != "" {
			u.t.print(indent(wrap("Your query was: "+r.Query, wrapWidth), " ") + "\n")
		}
		u.t.print("\n")
		for i, p := range r.Positions {
			name := p.Name
			if name == "" {
				name = fmt.Sprintf("Card %d", i+1)
			}
			u.t.print(fmt.Sprintf("  %2d) %-34s %s\n", i+1, name, p.title()))
		}
		u.t.print("   x) Export reading and quit\n")
		u.t.print("   q) Just quit\n")

		choice, ok := u.t.askLine("\n Choice: ")
		if !ok {
			return
		}
		switch choice {
		case keyQuit, "":
			return
		case "x":
			u.export(r)
			return
		default:
			n, err := strconv.Atoi(choice)
			if err != nil || n < 1 || n > len(r.Positions) {
				u.t.print(" Pick a card number, x, or q.\n")
				continue
			}
			if !u.showCard(r.Positions[n-1], "enter back to the review · m mindful · q quit") {
				return
			}
		}
	}
}

// runFreeform keeps dealing from the pile for as long as the reader wants,
// reshuffling when it runs out.
func runFreeform(u *ui) *Reading {
	r := &Reading{Query: u.query}
	u.t.print("\n" + header(r))
	u.t.print(" Free form reading. Draw as many cards as you like.\n")

	for {
		u.t.print(fmt.Sprintf("\n [enter] draw (%d left in the pile) · l list · x export · q quit: ", u.pile.Remaining()))
		key, ok := u.t.key()
		u.t.print("\n")
		if !ok || key == keyQuit {
			return r
		}
		switch key {
		case keyEnter, " ", keyRight, "d":
			if u.pile.Remaining() == 0 {
				u.t.print(" The pile is spent. Shuffling the deck again.\n")
			}
			card, drawn := u.pile.Draw()
			if !drawn {
				return r
			}
			p := place(u.deck, card, nil)
			p.Name = fmt.Sprintf("Draw %d", len(r.Positions)+1)
			r.Positions = append(r.Positions, p)
			if !u.showCard(p, "enter back to the pile · m mindful · q quit") {
				return r
			}
		case "l":
			if len(r.Positions) == 0 {
				u.t.print(" You have not drawn anything yet.\n")
				continue
			}
			u.t.print("\n")
			for i, p := range r.Positions {
				u.t.print(fmt.Sprintf("  %2d) %s\n", i+1, p.title()))
			}
		case "x":
			u.export(r)
			return r
		default:
			if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(r.Positions) {
				if !u.showCard(r.Positions[n-1], "enter back to the pile · m mindful · q quit") {
					return r
				}
				continue
			}
			u.t.print(" enter to draw, l to list, x to export, q to quit.\n")
		}
	}
}

func (u *ui) export(r *Reading) {
	if len(r.Positions) == 0 {
		u.t.print(" Nothing drawn yet, so there is nothing to export.\n")
		return
	}
	path, err := exportReading(r, u.opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tarot:", err)
		return
	}
	u.t.print(" Reading written to " + path + "\n")
}
