package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// mode is how the deck is consulted.
type mode string

const (
	modeCeltic   mode = "celtic"
	modeFreeform mode = "freeform"
	modeExplore  mode = "explore"
	modeCarousel mode = "carousel"
)

type menuEntry struct {
	mode  mode
	title string
	blurb string
}

// modeMenu is every reading on offer: the spreads first, then the browsing
// modes.
var modeMenu = func() []menuEntry {
	menu := make([]menuEntry, 0, len(spreadOrder)+3)
	for _, s := range spreadOrder {
		menu = append(menu, menuEntry{mode(s.Key), s.Title, s.Blurb})
	}
	return append(menu,
		menuEntry{modeFreeform, "Free form", "keep drawing from the pile, as many as you like"},
		menuEntry{modeExplore, "Explore", "walk the deck in order, no shuffling"},
		menuEntry{modeCarousel, "Carousel", "cards turn over on their own, on a timer"},
	)
}()

// spreadNames lists the spread keys for error messages.
func spreadNames() string {
	names := make([]string, 0, len(spreadOrder))
	for _, s := range spreadOrder {
		names = append(names, s.Key)
	}
	return strings.Join(names, ", ")
}

// plan is what the reader settled on: which mode, and for the carousel, how it
// draws and how long each card stays up. reversals and query are set only when
// the reader was asked; otherwise the flag, or whatever query is already
// running, stands.
type plan struct {
	mode      mode
	ordered   bool
	dwell     time.Duration
	reversals *bool
	query     *string
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
	items := make([]menuItem, 0, len(modeMenu))
	for _, m := range modeMenu {
		items = append(items, menuItem{m.title, m.blurb})
	}
	i, ok := u.t.menu("How would you like to read?", items, menuKeys)
	if !ok {
		return plan{}, false
	}

	m := modeMenu[i].mode
	if m == modeCarousel {
		return chooseCarousel(u)
	}
	// Every reading that shuffles a pile asks; explore walks the deck in order,
	// so there is nothing to ask about there.
	p := plan{mode: m}
	if _, spread := spreads[string(m)]; spread || m == modeFreeform {
		if p.query, ok = askQuery(u); !ok {
			return plan{}, false
		}
		if p.reversals, ok = askReversals(u); !ok {
			return plan{}, false
		}
	}
	return p, true
}

// askQuery asks what a new reading is about, defaulting to whatever query is
// already set (from the command line, or an earlier reading this session) so
// pressing Enter alone keeps it.
func askQuery(u *ui) (*string, bool) {
	prompt := " What's your question, if any? (blank for none): "
	if u.query != "" {
		prompt = fmt.Sprintf(" What's your question? [%s]: ", u.query)
	}
	answer, ok := u.t.askText(prompt)
	if !ok || answer == keyQuit {
		return nil, false
	}
	if answer == "" {
		answer = u.query
	}
	return &answer, true
}

// askReversals asks whether cards may come up reversed, defaulting to whatever
// the flags already say.
func askReversals(u *ui) (*bool, bool) {
	def := "Y/n"
	if !u.pile.reversals {
		def = "y/N"
	}
	answer, ok := u.t.askKey(fmt.Sprintf(" Include reversed cards? [%s]: ", def))
	if !ok || answer == keyQuit || answer == keyEsc {
		return nil, false
	}
	value := u.pile.reversals
	switch answer {
	case "y", "yes":
		value = true
	case "n", "no":
		value = false
	}
	return &value, true
}

// chooseCarousel asks how the carousel should run: where the cards come from,
// and how many seconds each one stays up.
func chooseCarousel(u *ui) (plan, bool) {
	p := plan{mode: modeCarousel, ordered: true, dwell: u.opts.dwell}
	if p.dwell <= 0 {
		p.dwell = defaultDwell
	}

	source, ok := u.t.menu("Where should the cards come from?", []menuItem{
		{"In order", "through the deck, as the guide lays it out"},
		{"At random", "drawn from the pile"},
	}, menuKeys)
	if !ok {
		return plan{}, false
	}
	p.ordered = source == 0
	if !p.ordered {
		// Drawing at random from the pile, so reversals are in play.
		if p.reversals, ok = askReversals(u); !ok {
			return plan{}, false
		}
	}

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

// showCard puts a card up and handles the keys available while it is there. r
// may be nil; when it is a laid out spread, "a" shows the whole tableau. It
// returns false when the reader wants out.
func (u *ui) showCard(p Position, r *Reading, hints string) bool {
	for {
		key, ok := u.t.view(page(p, u.opts), hints)
		if !ok || key == keyQuit {
			return false
		}
		switch key {
		case keyEnter, keyRight, "n", keyBksp, keyEsc:
			return true
		case "m":
			if !u.showMindful(p) {
				return false
			}
		case "a":
			if r != nil && !u.showTableau(r) {
				return false
			}
		case "e":
			if !u.exploreFrom(p) {
				return false
			}
		case "t":
			// Strip the card back to its picture, or put the words back. It
			// stays that way until it is turned back on.
			u.opts.bare = !u.opts.bare
		case "w":
			u.opts.detail = !u.opts.detail
		}
	}
}

// showTableau lays the whole spread out, every card pictured in its place. A
// card's number opens it there and then; anything else goes back.
func (u *ui) showTableau(r *Reading) bool {
	if !hasLayout(r.Spread) {
		return true
	}
	hints := fmt.Sprintf("%s open a card · esc back", cardKeys(len(r.Positions)))
	for {
		key, ok := u.t.viewScroll(tableau(r, u.opts), hints, true)
		if !ok || key == keyQuit {
			return false
		}
		i, isCard := cardKey(key, len(r.Positions))
		if !isCard {
			return true
		}
		if !u.showCard(r.Positions[i], nil, "enter back to the layout · m mindful · w Waite · e explore · q quit") {
			return false
		}
	}
}

// cardKey reads a card number off a keypress. With ten cards in a spread, 0
// stands for the tenth so that a single key is always enough.
func cardKey(key string, cards int) (int, bool) {
	if len(key) != 1 || key[0] < '0' || key[0] > '9' {
		return 0, false
	}
	n := int(key[0] - '0')
	if n == 0 {
		n = 10
	}
	if n > cards {
		return 0, false
	}
	return n - 1, true
}

// cardKeys describes which keys open which card, for the status bar.
func cardKeys(cards int) string {
	if cards >= 10 {
		return "1-9 and 0"
	}
	return fmt.Sprintf("1-%d", cards)
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

	if u.opts.layout {
		// Straight to the tableau, then the review: no card by card walk.
		if !u.showTableau(r) {
			return r
		}
		review(u, r)
		return r
	}
	if !u.opts.quiet {
		u.t.print("Beginning reading\n")
	}
	u.t.print("\n" + header(r))
	hints := "enter next · a layout · t text · m mindful · w Waite · e explore · q quit"
	if !hasLayout(s) {
		hints = "enter next · t text · m mindful · w Waite · e explore · q quit"
	}
	for _, p := range r.Positions {
		if !u.showCard(p, r, hints) {
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
			label := p.Label
			if label == "" {
				label = fmt.Sprintf("Draw %d", i+1)
			}
			u.t.print(fmt.Sprintf("  %2d) %-34s %s\n", i+1, label, p.Card.Title()))
		}
		if hasLayout(r.Spread) {
			u.t.print("   a) See the whole spread laid out\n")
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
		case "a":
			if !u.showTableau(r) {
				return
			}
		case "x":
			u.export(r)
			return
		default:
			n, err := strconv.Atoi(choice)
			if err != nil || n < 1 || n > len(r.Positions) {
				u.t.print(" Pick a card number, x, or q.\n")
				continue
			}
			if !u.showCard(r.Positions[n-1], r, "enter back to the review · a layout · m mindful · e explore · q quit") {
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
			p.Draw = len(r.Positions) + 1
			r.Positions = append(r.Positions, p)
			if !u.showCard(p, nil, "enter back to the pile · t text · m mindful · e explore · q quit") {
				return r
			}
		case "l":
			if len(r.Positions) == 0 {
				u.t.print(" You have not drawn anything yet.\n")
				continue
			}
			u.t.print("\n")
			for i, p := range r.Positions {
				u.t.print(fmt.Sprintf("  %2d) %s\n", i+1, p.heading()))
			}
		case "x":
			u.export(r)
			return r
		default:
			if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(r.Positions) {
				if !u.showCard(r.Positions[n-1], nil, "enter back to the pile · t text · m mindful · e explore · q quit") {
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
