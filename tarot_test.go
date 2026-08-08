package main

import (
	"bufio"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func testDeck(t *testing.T) *Deck {
	t.Helper()
	deck, err := loadDeck("")
	if err != nil {
		t.Fatal(err)
	}
	return deck
}

func TestLoadDeck(t *testing.T) {
	deck := testDeck(t)
	if got := len(deck.cards); got != deckSize*2 {
		t.Errorf("card table has %d entries, want %d", got, deckSize*2)
	}
	// Every card must be renderable and carry guide notes in both orientations.
	for n := 1; n <= deckSize*2; n++ {
		c := deck.card(n)
		if c.Name == "" {
			t.Fatalf("card %d missing from the table", n)
		}
		if (n > deckSize) != c.Reversed {
			t.Errorf("card %d reversed=%v, want %v", n, c.Reversed, n > deckSize)
		}
		if _, err := renderCard(c, 4, false); err != nil {
			t.Errorf("card %d (%s): %v", n, c.Name, err)
		}
	}
}

func TestNotesCoverEveryCard(t *testing.T) {
	deck := testDeck(t)
	if got := len(deck.guide.ByName); got != deckSize {
		t.Fatalf("guide has %d cards, want %d", got, deckSize)
	}
	for n := 1; n <= deckSize; n++ {
		c := deck.card(n)
		notes := deck.notes(c)
		if notes == nil {
			t.Fatalf("no guide entry for %s", c.Name)
		}
		for _, f := range []struct{ label, value string }{
			{"sketch", notes.Sketch},
			{"imagery", notes.Imagery},
			{"upright", notes.Upright},
			{"reversed", notes.Reversed},
			{"mindful title", notes.MindfulTitle},
			{"mindful body", notes.MindfulBody},
			{"waite upright", notes.WaiteUpright},
			{"waite reversed", notes.WaiteReversed},
		} {
			if strings.TrimSpace(f.value) == "" {
				t.Errorf("%s: empty %s", c.Name, f.label)
			}
		}
		// The essay is prose, not a stray heading or list fragment.
		if strings.Contains(notes.MindfulBody, "**Waite") || strings.HasPrefix(notes.MindfulBody, "#") {
			t.Errorf("%s: mindful body picked up neighboring markup", c.Name)
		}
	}
}

func TestNotesSpotCheck(t *testing.T) {
	deck := testDeck(t)
	fool := deck.guide.ByName["The Fool"]
	if fool == nil {
		t.Fatal("The Fool is missing from the guide")
	}
	if fool.MindfulTitle != "Beginner's Mind" {
		t.Errorf("mindful title = %q, want %q", fool.MindfulTitle, "Beginner's Mind")
	}
	if !strings.HasPrefix(fool.Upright, "It is a time of new beginnings.") {
		t.Errorf("upright summary = %q", truncate(fool.Upright))
	}
	if !strings.HasPrefix(fool.WaiteUpright, "Folly, mania, extravagance") {
		t.Errorf("waite upright = %q", truncate(fool.WaiteUpright))
	}
	if !strings.Contains(fool.Sketch, "'-------------'") {
		t.Errorf("sketch does not look like the card drawing: %q", truncate(fool.Sketch))
	}
	// "Judgment" in the guide is "Judgement" in the deck data.
	if deck.guide.ByName["Judgement"] == nil {
		t.Error("Judgement is missing from the guide")
	}
}

func truncate(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

func TestPileDealsWithoutRepeating(t *testing.T) {
	deck := testDeck(t)
	for seed := int64(0); seed < 20; seed++ {
		pile := newPile(deck, rand.New(rand.NewSource(seed)), true)
		seen := map[string]bool{}
		for i := 0; i < deckSize; i++ {
			c, ok := pile.Draw()
			if !ok {
				t.Fatalf("seed %d: pile ran dry after %d cards", seed, i)
			}
			if seen[c.Name] {
				t.Fatalf("seed %d: %s dealt twice", seed, c.Name)
			}
			seen[c.Name] = true
		}
		// The next draw reshuffles rather than failing.
		if _, ok := pile.Draw(); !ok || pile.Shuffles != 2 {
			t.Errorf("seed %d: pile did not reshuffle (shuffles=%d)", seed, pile.Shuffles)
		}
	}
}

func TestNoReversals(t *testing.T) {
	deck := testDeck(t)
	pile := newPile(deck, rand.New(rand.NewSource(5)), false)
	for i := 0; i < deckSize; i++ {
		if c, _ := pile.Draw(); c.Reversed {
			t.Fatalf("%s came up reversed with reversals off", c.Name)
		}
	}
}

func TestSpreadsAreWellFormed(t *testing.T) {
	deck := testDeck(t)
	for key, s := range spreads {
		if s.Key != key {
			t.Errorf("spread %q has key %q", key, s.Key)
		}
		for i, def := range s.Positions {
			if def.Name == "" || def.Meaning == "" {
				t.Errorf("%s position %d is missing a name or meaning", key, i+1)
			}
		}

		rng := rand.New(rand.NewSource(11))
		r := newReading(deck, newPile(deck, rng, true), s, "q", nil)
		if len(r.Positions) != len(s.Positions) {
			t.Errorf("%s dealt %d cards, want %d", key, len(r.Positions), len(s.Positions))
		}
		text := plain(r, options{art: artNone})
		if strings.Count(text, divider) != len(s.Positions)+2 {
			t.Errorf("%s reading has %d dividers, want %d", key, strings.Count(text, divider), len(s.Positions)+2)
		}
	}
}

func TestPageIncludesGuideSummary(t *testing.T) {
	deck := testDeck(t)
	upright := place(deck, deck.card(1), &celticSpread.Positions[0]) // The Fool
	reversed := place(deck, deck.card(1+deckSize), nil)              // The Fool, reversed
	opts := options{art: artNone}

	up := page(upright, opts)
	if !strings.Contains(up, "Upright:") || strings.Contains(up, "Reversed:") {
		t.Error("upright page should carry the upright summary only")
	}
	if !strings.Contains(strings.ReplaceAll(up, "\n ", " "), "time of new beginnings") {
		t.Error("upright page is missing the guide summary")
	}
	down := page(reversed, opts)
	if !strings.Contains(down, "Reversed:") {
		t.Error("reversed page is missing the reversed summary")
	}
	// The essay stays behind the m key.
	if strings.Contains(up, "Ise Jingu") {
		t.Error("the mindful essay should not be printed with the card")
	}
	if !strings.Contains(mindful(upright), "Ise Jingu") {
		t.Error("the mindful reading is missing its essay")
	}
	if !strings.Contains(detail(upright), "Waite (1911):") {
		t.Error("detail is missing Waite")
	}
}

func TestSketchArtFlipsWhenReversed(t *testing.T) {
	deck := testDeck(t)
	up := sketch(place(deck, deck.card(1), nil))
	down := sketch(place(deck, deck.card(1+deckSize), nil))
	if up == "" || down == "" {
		t.Fatal("sketch art came out empty")
	}
	if up == down {
		t.Error("a reversed card should be drawn flipped")
	}
	upLines := strings.Split(strings.TrimRight(up, "\n"), "\n")
	downLines := strings.Split(strings.TrimRight(down, "\n"), "\n")
	if len(upLines) != len(downLines) || upLines[0] != downLines[len(downLines)-1] {
		t.Error("flipping should reverse the line order and nothing else")
	}
}

func TestReadingIsReproducible(t *testing.T) {
	deck := testDeck(t)
	read := func() string {
		rng := rand.New(rand.NewSource(99))
		return plain(newReading(deck, newPile(deck, rng, true), celticSpread, "q", nil), options{art: artNone})
	}
	first, second := read(), read()
	if first != second {
		t.Error("same seed produced different readings")
	}
}

func TestWrap(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		width    int
	}{
		{"one two three", "one two\nthree", 8},
		{"exactlyten", "exactlyten", 5}, // a word longer than width is never split
		{"", "", 10},
		{"a  b", "a b", 10},
	} {
		if got := wrap(tc.in, tc.width); got != tc.want {
			t.Errorf("wrap(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}

	long := wrap(celticSpread.Positions[0].Meaning, wrapWidth)
	for _, line := range strings.Split(long, "\n") {
		if len(line) > wrapWidth {
			t.Errorf("line exceeds %d chars: %q", wrapWidth, line)
		}
	}
}

func TestSeedFrom(t *testing.T) {
	if got := seedFrom(42, "anything"); got != 42 {
		t.Errorf("explicit seed = %d, want 42", got)
	}
	if seedFrom(0, "a") == seedFrom(0, "b") {
		t.Error("different queries produced the same seed")
	}
}

func TestGuideOutlineMatchesTheDeck(t *testing.T) {
	deck := testDeck(t)
	want := []string{
		"Major Arcana (Trumps)",
		"Suit of Wands", "Suit of Cups", "Suit of Swords", "Suit of Pentacles",
	}
	var got []string
	total := 0
	for _, s := range deck.guide.Sections {
		got = append(got, s.Title)
		total += len(s.Cards)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("sections = %v, want %v", got, want)
	}
	if total != deckSize {
		t.Errorf("sections hold %d cards, want %d", total, deckSize)
	}
	if n := len(deck.guide.Sections[0].Cards); n != 22 {
		t.Errorf("Major Arcana has %d cards, want 22", n)
	}
	if first := deck.guide.Sections[0].Cards[0]; first.Label != "0. The Fool" {
		t.Errorf("first label = %q, want %q", first.Label, "0. The Fool")
	}
	// Every card in the outline resolves back to a card in the deck.
	for _, s := range deck.guide.Sections {
		for _, c := range s.Cards {
			if _, ok := deck.cardNamed(c.Name); !ok {
				t.Errorf("outline card %q is not in the deck", c.Name)
			}
		}
	}
}

func TestOutlineFoldingAndCursor(t *testing.T) {
	deck := testDeck(t)
	o := newOutline(deck.guide)
	if len(o.rows) != deckSize+len(deck.guide.Sections) {
		t.Fatalf("outline has %d rows, want %d", len(o.rows), deckSize+len(deck.guide.Sections))
	}
	if !o.rows[0].heading {
		t.Error("the outline should open with a section heading")
	}

	o.move(1)
	if o.rows[o.cursor].heading {
		t.Error("moving down from the heading should land on a card")
	}
	// Folding the Major Arcana hides its cards but keeps every heading.
	o.collapsed[0] = true
	vis := o.visible()
	if len(vis) != len(o.rows)-22 {
		t.Errorf("folded outline shows %d rows, want %d", len(vis), len(o.rows)-22)
	}

	// The cursor never walks off either end.
	o.collapsed[0] = false
	o.move(-50)
	if o.cursor != 0 {
		t.Errorf("cursor = %d after moving up past the top, want 0", o.cursor)
	}
	o.move(1000)
	if o.cursor != len(o.rows)-1 {
		t.Errorf("cursor = %d after moving past the end, want %d", o.cursor, len(o.rows)-1)
	}
}

func TestSideBySideArt(t *testing.T) {
	deck := testDeck(t)
	p := place(deck, deck.card(1), nil)
	both := art(p, options{art: artBoth, height: 12})
	if both == "" {
		t.Fatal("side by side art came out empty")
	}
	drawing := strings.Split(sketch(p), "\n")
	lines := strings.Split(strings.TrimRight(both, "\n \n"), "\n")
	// The drawing rides alongside the photo, not above or below it.
	if len(lines) > 14 {
		t.Errorf("art is %d lines tall, want the photo's height", len(lines))
	}
	joined := strings.Join(lines, "\n")
	for _, l := range drawing {
		if strings.TrimSpace(l) != "" && !strings.Contains(joined, strings.TrimSpace(l)) {
			t.Errorf("drawing line %q is missing from the side by side art", l)
			break
		}
	}
}

func TestVisibleWidthIgnoresColor(t *testing.T) {
	if got := visibleWidth("\x1b[38;2;1;2;3mab\x1b[0m"); got != 2 {
		t.Errorf("visibleWidth = %d, want 2", got)
	}
	rows := sideBySide([]string{"\x1b[31mxx\x1b[0m"}, []string{"y"}, 2)
	if want := "\x1b[31mxx\x1b[0m  y"; rows[0] != want {
		t.Errorf("sideBySide = %q, want %q", rows[0], want)
	}
}

func TestPageOrdersImageryBeforeTheName(t *testing.T) {
	deck := testDeck(t)
	p := place(deck, deck.card(1), nil)
	text := page(p, options{art: artNone})
	imagery := strings.Index(text, "A young wanderer")
	name := strings.Index(text, "  The Fool\n")
	if imagery < 0 || name < 0 {
		t.Fatalf("page is missing the imagery or the name:\n%s", text)
	}
	if imagery > name {
		t.Error("the imagery should come just before the card's name")
	}
	// The bash version's generated sentence is gone.
	if strings.Contains(text, "# The Fool in light") {
		t.Error("the generated interpretation should no longer be printed")
	}
}

func TestKeyDecoding(t *testing.T) {
	tr := &term{in: bufio.NewReader(strings.NewReader("\x1b[A\x1b[B\x1b[C\x1b[D\x1b[5~j\r\x1b")), raw: true}
	want := []string{keyUp, keyDown, keyRight, keyLeft, keyPgUp, "j", keyEnter, keyEsc}
	for _, w := range want {
		got, ok := tr.key()
		if !ok {
			t.Fatalf("input ran out before %q", w)
		}
		if got != w {
			t.Errorf("key = %q, want %q", got, w)
		}
	}
	if _, ok := tr.key(); ok {
		t.Error("expected end of input")
	}
}

func TestLineModeKeys(t *testing.T) {
	tr := &term{in: bufio.NewReader(strings.NewReader("\nj\nDOWN\nq\n"))}
	want := []string{keyEnter, "j", "down", keyQuit}
	for _, w := range want {
		if got, ok := tr.key(); !ok || got != w {
			t.Errorf("key = %q (ok=%v), want %q", got, ok, w)
		}
	}
}

func TestExploreCardStepping(t *testing.T) {
	deck := testDeck(t)
	o := newOutline(deck.guide)
	o.moveCard(1)
	if got := o.rows[o.cursor].label; got != "0. The Fool" {
		t.Fatalf("first card = %q, want %q", got, "0. The Fool")
	}
	// Left at the very first card stays put rather than landing on a heading.
	o.moveCard(-1)
	if got := o.rows[o.cursor].label; got != "0. The Fool" {
		t.Errorf("stepping back from the first card went to %q", got)
	}
	// Stepping forward crosses the section boundary onto the next card.
	for range 21 {
		o.moveCard(1)
	}
	if got := o.rows[o.cursor].label; got != "21. The World" {
		t.Fatalf("last major = %q, want %q", got, "21. The World")
	}
	o.moveCard(1)
	if got := o.rows[o.cursor]; got.heading || got.label != "Ace of Wands" {
		t.Errorf("card after the majors = %q (heading=%v), want Ace of Wands", got.label, got.heading)
	}
	o.moveCard(-1)
	if got := o.rows[o.cursor].label; got != "21. The World" {
		t.Errorf("stepping back across the boundary went to %q", got)
	}
	// And it never lands on a heading anywhere in the deck.
	o.cursor = 0
	o.moveCard(1)
	for range deckSize * 2 {
		o.moveCard(1)
		if o.rows[o.cursor].heading {
			t.Fatalf("landed on the heading %q", o.rows[o.cursor].label)
		}
	}
}

func TestBackspaceIsItsOwnKey(t *testing.T) {
	tr := &term{in: bufio.NewReader(strings.NewReader("\x7f\x08\x1b[D")), raw: true}
	want := []string{keyBksp, keyBksp, keyLeft}
	for _, w := range want {
		if got, ok := tr.key(); !ok || got != w {
			t.Errorf("key = %q (ok=%v), want %q", got, ok, w)
		}
	}
}

func TestPickModeResolvesTheCarousel(t *testing.T) {
	for _, tc := range []struct {
		flag    string
		dwell   time.Duration
		want    mode
		ordered bool
		wantDur time.Duration
	}{
		{"celtic", 0, modeCeltic, true, 0},
		{"explore", 0, modeExplore, true, 0},
		{"freeform", 0, modeFreeform, true, 0},
		// A dwell turns the browsing modes into the carousel.
		{"explore", 30 * time.Second, modeCarousel, true, 30 * time.Second},
		{"freeform", 30 * time.Second, modeCarousel, false, 30 * time.Second},
		// The carousel names itself, and picks up the default pace.
		{"carousel", 0, modeCarousel, true, defaultDwell},
		{"CAROUSEL", 5 * time.Second, modeCarousel, true, 5 * time.Second},
	} {
		u := &ui{opts: options{dwell: tc.dwell}}
		got, err := pickMode(u, tc.flag)
		if err != nil {
			t.Fatalf("-mode %s -dwell %s: %v", tc.flag, tc.dwell, err)
		}
		if got.mode != tc.want || got.dwell != tc.wantDur {
			t.Errorf("-mode %s -dwell %s = %v/%s, want %v/%s", tc.flag, tc.dwell, got.mode, got.dwell, tc.want, tc.wantDur)
		}
		if got.mode == modeCarousel && got.ordered != tc.ordered {
			t.Errorf("-mode %s: ordered = %v, want %v", tc.flag, got.ordered, tc.ordered)
		}
	}
	if _, err := pickMode(&ui{}, "tea"); err == nil {
		t.Error("an unknown mode should be an error")
	}
}

func TestCarouselStatus(t *testing.T) {
	got := carouselStatus(3, 9, 45*time.Second, false, false, true, 0, 0)
	if !strings.HasPrefix(got, " 3/9 · 45s · ") {
		t.Errorf("status = %q", got)
	}
	if paused := carouselStatus(1, 1, time.Minute, true, false, true, 0, 0); !strings.Contains(paused, "paused") {
		t.Errorf("paused status = %q", paused)
	}
	if rev := carouselStatus(1, 1, time.Minute, false, true, true, 0, 0); !strings.Contains(rev, "reversed") {
		t.Errorf("reversed status = %q", rev)
	}
}

func TestKeyWithinTimesOut(t *testing.T) {
	// Nothing to read: the wait ends on the clock, and input is still open.
	tr := &term{in: bufio.NewReader(&blockingReader{})}
	start := time.Now()
	if key, ok, timedOut := tr.keyWithin(40 * time.Millisecond); !timedOut || !ok || key != "" {
		t.Errorf("keyWithin = %q/%v/%v, want a timeout", key, ok, timedOut)
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Errorf("returned after %s, want to have waited", elapsed)
	}
	// A key that is already there comes back without waiting.
	tr2 := &term{in: bufio.NewReader(strings.NewReader("j\n"))}
	if key, ok, timedOut := tr2.keyWithin(time.Minute); key != "j" || !ok || timedOut {
		t.Errorf("keyWithin = %q/%v/%v, want j", key, ok, timedOut)
	}
}

// blockingReader never returns, standing in for an idle keyboard.
type blockingReader struct{}

func (b *blockingReader) Read(p []byte) (int, error) { select {} }
